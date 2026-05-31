package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jcarvalho/ftm-platform/internal/backend/auth"
	"github.com/jcarvalho/ftm-platform/internal/backend/config"
	"github.com/jcarvalho/ftm-platform/internal/backend/store"
)

type Server struct {
	cfg   config.Config
	store *store.Store
	mux   *http.ServeMux
}

func New(cfg config.Config, dataStore *store.Store) *Server {
	s := &Server{cfg: cfg, store: dataStore, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := netJoin(s.cfg.BindAddress, s.cfg.Port)
	return http.ListenAndServe(addr, s.withCORS(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("POST /api/login", s.login)
	s.mux.HandleFunc("GET /api/status", s.requireAuth(s.status))
	s.mux.HandleFunc("POST /api/ftp/start", s.requireAdmin(s.startFTP))
	s.mux.HandleFunc("POST /api/ftp/stop", s.requireAdmin(s.stopFTP))
	s.mux.HandleFunc("POST /api/ftp/restart", s.requireAdmin(s.restartFTP))
	s.mux.HandleFunc("GET /api/accounts", s.requireAdmin(s.accounts))
	s.mux.HandleFunc("POST /api/accounts", s.requireAdmin(s.saveAccount))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"state": "ok"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	account, ok := s.store.Authenticate(request.Username, request.Password)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := auth.Sign(s.cfg.JWTSecret, auth.Claims{
		Sub:  account.Username,
		Role: account.Role,
		Exp:  time.Now().Add(8 * time.Hour).Unix(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  account.Username,
		"role":  account.Role,
	})
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	running := s.ftpRunning()
	status := map[string]any{
		"ftp_running": running,
	}
	if data, err := os.ReadFile(s.cfg.FTPStatusFile); err == nil {
		var ftpStatus map[string]any
		if json.Unmarshal(data, &ftpStatus) == nil {
			if !running {
				ftpStatus["state"] = "stopped"
			}
			status["ftp_status"] = ftpStatus
		}
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) startFTP(w http.ResponseWriter, _ *http.Request) {
	if s.ftpRunning() {
		writeJSON(w, http.StatusOK, map[string]string{"state": "already_running"})
		return
	}
	cmd := exec.Command(s.cfg.FTPBinary, "-config", s.cfg.FTPConfigFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	writeJSON(w, http.StatusAccepted, map[string]any{"state": "started", "pid": pid})
}

func (s *Server) stopFTP(w http.ResponseWriter, _ *http.Request) {
	state, err := s.terminateFTP()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"state": state})
}

func (s *Server) restartFTP(w http.ResponseWriter, r *http.Request) {
	_, _ = s.terminateFTP()
	time.Sleep(500 * time.Millisecond)
	s.startFTP(w, r)
}

func (s *Server) accounts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"accounts": s.store.Accounts()})
}

func (s *Server) saveAccount(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	account, err := s.store.SaveAccount(request.Username, request.Password, request.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := s.claims(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		next(w, r)
	}
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := s.claims(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		if claims.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin permission required")
			return
		}
		next(w, r)
	}
}

func (s *Server) claims(r *http.Request) (auth.Claims, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return auth.Claims{}, errors.New("missing Authorization header")
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == header {
		return auth.Claims{}, errors.New("Authorization must be Bearer token")
	}
	return auth.Verify(s.cfg.JWTSecret, token)
}

func (s *Server) ftpRunning() bool {
	pid, err := s.readFTPPid()
	if err != nil {
		return false
	}
	if !processRunning(pid) {
		_ = os.Remove(s.cfg.FTPPidFile)
		return false
	}
	if !s.ftpPortOpen() {
		return false
	}
	return true
}

func (s *Server) readFTPPid() (int, error) {
	data, err := os.ReadFile(s.cfg.FTPPidFile)
	if err != nil {
		return 0, errors.New("FTP server PID file not found")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, errors.New("invalid FTP server PID file")
	}
	return pid, nil
}

func (s *Server) terminateFTP() (string, error) {
	pid, err := s.readFTPPid()
	if err != nil {
		return "already_stopped", nil
	}
	if !processRunning(pid) {
		_ = os.Remove(s.cfg.FTPPidFile)
		return "already_stopped", nil
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			_ = os.Remove(s.cfg.FTPPidFile)
			return "already_stopped", nil
		}
		return "", err
	}
	for range 10 {
		if !processRunning(pid) || !s.ftpPortOpen() {
			_ = os.Remove(s.cfg.FTPPidFile)
			return "stopped", nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "stopping", nil
}

func processRunning(pid int) bool {
	err := syscall.Kill(pid, syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

func (s *Server) ftpPortOpen() bool {
	address := s.ftpStatusAddress()
	conn, err := net.DialTimeout("tcp", address, 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (s *Server) ftpStatusAddress() string {
	host := "127.0.0.1"
	if data, err := os.ReadFile(s.cfg.FTPStatusFile); err == nil {
		var status struct {
			BindAddress string `json:"bind_address"`
			Port        int    `json:"port"`
		}
		if json.Unmarshal(data, &status) == nil {
			if status.BindAddress != "" && status.BindAddress != "0.0.0.0" && status.BindAddress != "::" {
				host = status.BindAddress
			}
			if status.Port > 0 {
				return net.JoinHostPort(host, strconv.Itoa(status.Port))
			}
		}
	}
	return net.JoinHostPort(host, "2121")
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func netJoin(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}
