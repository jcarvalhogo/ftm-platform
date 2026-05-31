package main

import (
	"flag"
	"log"

	"github.com/jcarvalho/ftm-platform/internal/ftp/config"
	"github.com/jcarvalho/ftm-platform/internal/ftp/ftpserver"
)

func main() {
	configPath := flag.String("config", "./configs/ftp-server.toml", "path to FTP server TOML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := ftpserver.New(cfg).ListenAndServe(); err != nil {
		log.Fatalf("ftp server stopped: %v", err)
	}
}
