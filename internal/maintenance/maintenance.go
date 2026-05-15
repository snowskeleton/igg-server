package maintenance

import (
	"context"
	"log"
	"time"

	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

func Start(store *postgres.Store) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			run(ctx, store)
			cancel()
		}
	}()
}

func run(ctx context.Context, store *postgres.Store) {
	if n, err := store.CleanExpiredMagicTokens(ctx); err != nil {
		log.Printf("maintenance: clean magic tokens: %v", err)
	} else if n > 0 {
		log.Printf("maintenance: cleaned %d expired magic tokens", n)
	}

	if n, err := store.CleanRevokedRefreshTokens(ctx); err != nil {
		log.Printf("maintenance: clean refresh tokens: %v", err)
	} else if n > 0 {
		log.Printf("maintenance: cleaned %d revoked refresh tokens", n)
	}

	if n, err := store.CleanOldNotificationLogs(ctx); err != nil {
		log.Printf("maintenance: clean notification logs: %v", err)
	} else if n > 0 {
		log.Printf("maintenance: cleaned %d old notification log entries", n)
	}

	if err := store.CleanExpiredAdminSessions(ctx); err != nil {
		log.Printf("maintenance: clean admin sessions: %v", err)
	}
}
