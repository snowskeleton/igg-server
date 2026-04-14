package main

import (
	"log"
	"net/http"
	"os"

	"github.com/snowskeleton/igg-server/internal/config"
	"github.com/snowskeleton/igg-server/internal/server"
	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	store, err := postgres.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer store.Close()

	// Run migrations
	migrationsDir := "migrations"
	if dir := os.Getenv("MIGRATIONS_DIR"); dir != "" {
		migrationsDir = dir
	}
	if err := store.RunMigrations(migrationsDir); err != nil {
		log.Printf("migrations: %v (may already be applied)", err)
	}

	h := server.New(cfg, store)

	addr := ":" + cfg.Port
	log.Printf("igg-server listening on %s", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatalf("server: %v", err)
	}
}
