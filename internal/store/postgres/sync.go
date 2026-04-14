package postgres

import (
	"context"
	"time"
)

func (s *Store) GetSyncCursor(ctx context.Context, userID, deviceID string) (time.Time, error) {
	var cursorAt time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT cursor_at FROM sync_cursors WHERE user_id = $1 AND device_id = $2`,
		userID, deviceID).Scan(&cursorAt)
	if err != nil {
		return time.Time{}, nil // no cursor yet, return zero time
	}
	return cursorAt, nil
}

func (s *Store) UpsertSyncCursor(ctx context.Context, userID, deviceID string, cursorAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sync_cursors (user_id, device_id, cursor_at) VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, device_id) DO UPDATE SET cursor_at = $3`,
		userID, deviceID, cursorAt)
	return err
}
