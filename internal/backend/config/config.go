package config

import "github.com/jcarvalho/ftm-platform/internal/minitoml"

type Config struct {
	BindAddress   string
	Port          int
	JWTSecret     string
	DataFile      string
	FTPBinary     string
	FTPConfigFile string
	FTPStatusFile string
	FTPPidFile    string
	DefaultAdmin  DefaultAdmin
}

type DefaultAdmin struct {
	Username string
	Password string
}

func Load(path string) (Config, error) {
	doc, err := minitoml.Load(path)
	if err != nil {
		return Config{}, err
	}
	server := doc["server"]
	admin := doc["default_admin"]
	return Config{
		BindAddress:   minitoml.String(server, "bind_address", "127.0.0.1"),
		Port:          minitoml.Int(server, "port", 8080),
		JWTSecret:     minitoml.String(server, "jwt_secret", "change-this-secret"),
		DataFile:      minitoml.String(server, "data_file", "./data/backend/ftm-backend.gob"),
		FTPBinary:     minitoml.String(server, "ftp_binary", "./build/ftm-ftp-server"),
		FTPConfigFile: minitoml.String(server, "ftp_config_file", "./configs/ftp-server.toml"),
		FTPStatusFile: minitoml.String(server, "ftp_status_file", "./data/runtime/ftp-status.json"),
		FTPPidFile:    minitoml.String(server, "ftp_pid_file", "./data/runtime/ftp-server.pid"),
		DefaultAdmin: DefaultAdmin{
			Username: minitoml.String(admin, "username", "admin"),
			Password: minitoml.String(admin, "password", "admin123"),
		},
	}, nil
}
