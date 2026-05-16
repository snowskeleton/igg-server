package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

func New(databaseURL string) (*Store, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Retry connection for up to 30 seconds to handle startup ordering
	var pingErr error
	for attempt := range 10 {
		if pingErr = db.Ping(); pingErr == nil {
			break
		}
		log.Printf("database: waiting for postgres (attempt %d/10): %v", attempt+1, pingErr)
		time.Sleep(3 * time.Second)
	}
	if pingErr != nil {
		db.Close()
		return nil, fmt.Errorf("ping db after retries: %w", pingErr)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) RunMigrations(migrationsDir string) error {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}
		statements := strings.Split(string(data), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := s.db.Exec(stmt); err != nil {
				// Ignore "already exists" errors so migrations are idempotent
				if strings.Contains(err.Error(), "already exists") ||
					strings.Contains(err.Error(), "duplicate") {
					continue
				}
				return fmt.Errorf("exec migration %s: %w (statement: %s)", filepath.Base(file), err, stmt[:min(len(stmt), 80)])
			}
		}
	}
	return nil
}

func (s *Store) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, nil)
}
