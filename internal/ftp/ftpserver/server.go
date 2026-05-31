package ftpserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jcarvalho/ftm-platform/internal/files"
	"github.com/jcarvalho/ftm-platform/internal/ftp/config"
)

type Server struct {
	cfg      config.Config
	listener net.Listener
	mu       sync.Mutex
	active   int
	started  time.Time
}

type Status struct {
	State             string `json:"state"`
	BindAddress       string `json:"bind_address"`
	Port              int    `json:"port"`
	ActiveConnections int    `json:"active_connections"`
	StartedAt         string `json:"started_at"`
	UpdatedAt         string `json:"updated_at"`
}

func New(cfg config.Config) *Server {
	return &Server{cfg: cfg, started: time.Now()}
}

func (s *Server) ListenAndServe() error {
	if err := files.EnsureDir(s.cfg.RootDir); err != nil {
		return err
	}
	if err := s.writePid(); err != nil {
		return err
	}
	defer os.Remove(s.cfg.PidFile)

	addr := net.JoinHostPort(s.cfg.BindAddress, strconv.Itoa(s.cfg.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln
	log.Printf("FTP server listening on %s", addr)

	s.writeStatus("running")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			s.writeStatus("running")
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		s.addActive(1)
		go func() {
			defer s.addActive(-1)
			handleSession(s, conn)
		}()
	}
}

func (s *Server) authenticate(username, password string) (config.User, bool) {
	user, ok := s.cfg.Users[username]
	return user, ok && user.Password == password
}

func (s *Server) passiveAddress() string {
	host := s.cfg.BindAddress
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return host
}

func (s *Server) newPassiveListener() (net.Listener, int, error) {
	for port := s.cfg.PassivePortMin; port <= s.cfg.PassivePortMax; port++ {
		addr := net.JoinHostPort(s.cfg.BindAddress, strconv.Itoa(port))
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, port, nil
		}
	}
	return nil, 0, fmt.Errorf("no passive port available")
}

func (s *Server) addActive(delta int) {
	s.mu.Lock()
	s.active += delta
	if s.active < 0 {
		s.active = 0
	}
	s.mu.Unlock()
	s.writeStatus("running")
}

func (s *Server) writePid() error {
	if err := files.EnsureParent(s.cfg.PidFile); err != nil {
		return err
	}
	return os.WriteFile(s.cfg.PidFile, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

func (s *Server) writeStatus(state string) {
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	status := Status{
		State:             state,
		BindAddress:       s.cfg.BindAddress,
		Port:              s.cfg.Port,
		ActiveConnections: active,
		StartedAt:         s.started.Format(time.RFC3339),
		UpdatedAt:         time.Now().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return
	}
	if err := files.EnsureParent(s.cfg.StatusFile); err != nil {
		return
	}
	_ = os.WriteFile(s.cfg.StatusFile, data, 0o644)
}
