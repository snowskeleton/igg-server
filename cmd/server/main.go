package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/snowskeleton/igg-server/internal/config"
	"github.com/snowskeleton/igg-server/internal/maintenance"
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
	maintenance.Start(store)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("igg-server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
