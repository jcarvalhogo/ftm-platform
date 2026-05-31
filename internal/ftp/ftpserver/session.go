package ftpserver

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jcarvalho/ftm-platform/internal/ftp/config"
)

type session struct {
	server    *Server
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	username  string
	user      config.User
	loggedIn  bool
	cwd       string
	passiveLn net.Listener
}

func handleSession(server *Server, conn net.Conn) {
	defer conn.Close()
	s := &session{
		server: server,
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
		cwd:    "/",
	}
	defer s.closePassive()

	s.reply(220, "FTM FTP server ready")
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		cmd, arg, _ := strings.Cut(line, " ")
		cmd = strings.ToUpper(strings.TrimSpace(cmd))
		arg = strings.TrimSpace(arg)
		if !s.dispatch(cmd, arg) {
			return
		}
	}
}

func (s *session) dispatch(cmd, arg string) bool {
	switch cmd {
	case "USER":
		s.username = arg
		s.reply(331, "Password required")
	case "PASS":
		user, ok := s.server.authenticate(s.username, arg)
		if !ok {
			s.reply(530, "Authentication failed")
			return true
		}
		s.user = user
		s.loggedIn = true
		s.cwd = "/"
		s.reply(230, "Login successful")
	case "SYST":
		s.reply(215, "UNIX Type: L8")
	case "FEAT":
		s.raw("211-Features\r\n PASV\r\n EPSV\r\n UTF8\r\n211 End\r\n")
	case "TYPE":
		s.reply(200, "Type set")
	case "NOOP":
		s.reply(200, "OK")
	case "PWD", "XPWD":
		if !s.requireLogin() {
			return true
		}
		s.reply(257, fmt.Sprintf("\"%s\" is current directory", s.cwd))
	case "CWD":
		if s.requireLogin() {
			s.cwdCommand(arg)
		}
	case "CDUP":
		if s.requireLogin() {
			s.cwdCommand("..")
		}
	case "PASV":
		if s.requireLogin() {
			s.enterPassive()
		}
	case "EPSV":
		if s.requireLogin() {
			s.enterExtendedPassive()
		}
	case "LIST":
		if s.requireLogin() {
			s.listCommand(arg, true)
		}
	case "NLST":
		if s.requireLogin() {
			s.listCommand(arg, false)
		}
	case "RETR":
		if s.requireLogin() {
			s.retrieve(arg)
		}
	case "STOR":
		if s.requireLogin() {
			s.store(arg)
		}
	case "DELE":
		if s.requireLogin() {
			s.deleteFile(arg)
		}
	case "MKD", "XMKD":
		if s.requireLogin() {
			s.makeDir(arg)
		}
	case "RMD", "XRMD":
		if s.requireLogin() {
			s.removeDir(arg)
		}
	case "QUIT":
		s.reply(221, "Goodbye")
		return false
	default:
		s.reply(502, "Command not implemented")
	}
	return true
}

func (s *session) requireLogin() bool {
	if !s.loggedIn {
		s.reply(530, "Please login first")
		return false
	}
	return true
}

func (s *session) cwdCommand(arg string) {
	path, err := s.resolvePath(arg)
	if err != nil {
		s.reply(550, err.Error())
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		s.reply(550, "Directory not found")
		return
	}
	s.cwd = s.virtualPath(path)
	s.reply(250, "Directory changed")
}

func (s *session) enterPassive() {
	ln, port, err := s.createPassive()
	if err != nil {
		s.reply(425, err.Error())
		return
	}
	host := strings.ReplaceAll(s.server.passiveAddress(), ".", ",")
	s.passiveLn = ln
	s.reply(227, fmt.Sprintf("Entering Passive Mode (%s,%d,%d)", host, port/256, port%256))
}

func (s *session) enterExtendedPassive() {
	ln, port, err := s.createPassive()
	if err != nil {
		s.reply(425, err.Error())
		return
	}
	s.passiveLn = ln
	s.reply(229, fmt.Sprintf("Entering Extended Passive Mode (|||%d|)", port))
}

func (s *session) createPassive() (net.Listener, int, error) {
	s.closePassive()
	return s.server.newPassiveListener()
}

func (s *session) dataConn() (net.Conn, error) {
	if s.passiveLn == nil {
		return nil, fmt.Errorf("use PASV or EPSV first")
	}
	defer s.closePassive()
	if tcp, ok := s.passiveLn.(*net.TCPListener); ok {
		_ = tcp.SetDeadline(time.Now().Add(30 * time.Second))
	}
	return s.passiveLn.Accept()
}

func (s *session) closePassive() {
	if s.passiveLn != nil {
		_ = s.passiveLn.Close()
		s.passiveLn = nil
	}
}

func (s *session) listCommand(arg string, detailed bool) {
	path, err := s.resolvePath(arg)
	if err != nil {
		s.reply(550, err.Error())
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		s.reply(550, "Unable to list directory")
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	conn, err := s.openTransfer()
	if err != nil {
		return
	}
	defer conn.Close()
	for _, entry := range entries {
		if detailed {
			info, _ := entry.Info()
			line := formatListEntry(entry.Name(), info)
			_, _ = io.WriteString(conn, line+"\r\n")
		} else {
			_, _ = io.WriteString(conn, entry.Name()+"\r\n")
		}
	}
	s.reply(226, "Transfer complete")
}

func (s *session) retrieve(arg string) {
	if !s.user.Read {
		s.reply(550, "Read permission denied")
		return
	}
	path, err := s.resolvePath(arg)
	if err != nil {
		s.reply(550, err.Error())
		return
	}
	file, err := os.Open(path)
	if err != nil {
		s.reply(550, "File not found")
		return
	}
	defer file.Close()
	conn, err := s.openTransfer()
	if err != nil {
		return
	}
	defer conn.Close()
	_, _ = io.Copy(conn, file)
	s.reply(226, "Transfer complete")
}

func (s *session) store(arg string) {
	if !s.user.Write {
		s.reply(550, "Write permission denied")
		return
	}
	path, err := s.resolvePath(arg)
	if err != nil {
		s.reply(550, err.Error())
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		s.reply(550, "Unable to create parent directory")
		return
	}
	file, err := os.Create(path)
	if err != nil {
		s.reply(550, "Unable to create file")
		return
	}
	defer file.Close()
	conn, err := s.openTransfer()
	if err != nil {
		return
	}
	defer conn.Close()
	_, _ = io.Copy(file, conn)
	s.reply(226, "Transfer complete")
}

func (s *session) deleteFile(arg string) {
	if !s.user.Write {
		s.reply(550, "Write permission denied")
		return
	}
	path, err := s.resolvePath(arg)
	if err != nil {
		s.reply(550, err.Error())
		return
	}
	if err := os.Remove(path); err != nil {
		s.reply(550, "Unable to delete file")
		return
	}
	s.reply(250, "Deleted")
}

func (s *session) makeDir(arg string) {
	if !s.user.Write {
		s.reply(550, "Write permission denied")
		return
	}
	path, err := s.resolvePath(arg)
	if err != nil {
		s.reply(550, err.Error())
		return
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		s.reply(550, "Unable to create directory")
		return
	}
	s.reply(257, fmt.Sprintf("\"%s\" created", s.virtualPath(path)))
}

func (s *session) removeDir(arg string) {
	if !s.user.Write {
		s.reply(550, "Write permission denied")
		return
	}
	path, err := s.resolvePath(arg)
	if err != nil {
		s.reply(550, err.Error())
		return
	}
	if err := os.Remove(path); err != nil {
		s.reply(550, "Unable to remove directory")
		return
	}
	s.reply(250, "Removed")
}

func (s *session) openTransfer() (net.Conn, error) {
	s.reply(150, "Opening data connection")
	conn, err := s.dataConn()
	if err != nil {
		s.reply(425, err.Error())
		return nil, err
	}
	return conn, nil
}

func (s *session) resolvePath(arg string) (string, error) {
	root, err := filepath.Abs(s.user.RootDir)
	if err != nil {
		return "", err
	}
	target := arg
	if target == "" {
		target = s.cwd
	}
	if strings.HasPrefix(target, "/") {
		target = strings.TrimPrefix(target, "/")
	} else {
		target = filepath.Join(strings.TrimPrefix(s.cwd, "/"), target)
	}
	clean := filepath.Clean(filepath.Join(root, target))
	if clean != root && !strings.HasPrefix(clean, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes root")
	}
	return clean, nil
}

func (s *session) virtualPath(path string) string {
	root, err := filepath.Abs(s.user.RootDir)
	if err != nil {
		return "/"
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return "/"
	}
	return "/" + filepath.ToSlash(rel)
}

func (s *session) reply(code int, message string) {
	s.raw(strconv.Itoa(code) + " " + message + "\r\n")
}

func (s *session) raw(text string) {
	_, _ = s.writer.WriteString(text)
	_ = s.writer.Flush()
}

func formatListEntry(name string, info os.FileInfo) string {
	if info == nil {
		return name
	}
	mode := "-rw-r--r--"
	if info.IsDir() {
		mode = "drwxr-xr-x"
	}
	return fmt.Sprintf("%s 1 owner group %12d %s %s", mode, info.Size(), info.ModTime().Format("Jan _2 15:04"), name)
}
