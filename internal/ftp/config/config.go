package config

import (
	"fmt"
	"strings"

	"github.com/jcarvalho/ftm-platform/internal/minitoml"
)

type Config struct {
	BindAddress    string
	Port           int
	RootDir        string
	StatusFile     string
	PidFile        string
	PassivePortMin int
	PassivePortMax int
	Users          map[string]User
}

type User struct {
	Username string
	Password string
	RootDir  string
	Read     bool
	Write    bool
}

func Load(path string) (Config, error) {
	doc, err := minitoml.Load(path)
	if err != nil {
		return Config{}, err
	}

	server := doc["server"]
	cfg := Config{
		BindAddress:    minitoml.String(server, "bind_address", "0.0.0.0"),
		Port:           minitoml.Int(server, "port", 2121),
		RootDir:        minitoml.String(server, "root_dir", "./data/ftp-root"),
		StatusFile:     minitoml.String(server, "status_file", "./data/runtime/ftp-status.json"),
		PidFile:        minitoml.String(server, "pid_file", "./data/runtime/ftp-server.pid"),
		PassivePortMin: minitoml.Int(server, "passive_port_min", 40000),
		PassivePortMax: minitoml.Int(server, "passive_port_max", 40100),
		Users:          map[string]User{},
	}
	if cfg.PassivePortMax < cfg.PassivePortMin {
		return Config{}, fmt.Errorf("passive_port_max must be >= passive_port_min")
	}

	for section, values := range doc {
		if !strings.HasPrefix(section, "users.") {
			continue
		}
		username := strings.TrimPrefix(section, "users.")
		if username == "" {
			continue
		}
		user := User{
			Username: username,
			Password: minitoml.String(values, "password", ""),
			RootDir:  minitoml.String(values, "root_dir", cfg.RootDir),
			Read:     minitoml.Bool(values, "read", true),
			Write:    minitoml.Bool(values, "write", false),
		}
		if user.Password == "" {
			return Config{}, fmt.Errorf("user %q has empty password", username)
		}
		cfg.Users[username] = user
	}
	if len(cfg.Users) == 0 {
		cfg.Users["admin"] = User{Username: "admin", Password: "admin123", RootDir: cfg.RootDir, Read: true, Write: true}
	}
	return cfg, nil
}
