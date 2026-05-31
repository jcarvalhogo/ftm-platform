package main

import (
	"flag"
	"log"

	"github.com/jcarvalho/ftm-platform/internal/backend/api"
	"github.com/jcarvalho/ftm-platform/internal/backend/config"
	"github.com/jcarvalho/ftm-platform/internal/backend/store"
)

func main() {
	configPath := flag.String("config", "./configs/backend.toml", "path to backend TOML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	dataStore, err := store.Open(cfg.DataFile)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	if err := dataStore.EnsureAdmin(cfg.DefaultAdmin.Username, cfg.DefaultAdmin.Password); err != nil {
		log.Fatalf("ensure default admin: %v", err)
	}

	log.Printf("Backend listening on %s:%d", cfg.BindAddress, cfg.Port)
	if err := api.New(cfg, dataStore).ListenAndServe(); err != nil {
		log.Fatalf("backend stopped: %v", err)
	}
}
