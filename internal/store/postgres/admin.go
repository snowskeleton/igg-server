package postgres

import (
	"context"
	"database/sql"
	"time"
)

// ── Admin Sessions ──

type AdminSession struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	TokenHash string    `db:"token_hash"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}

func (s *Store) CreateAdminSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO admin_sessions (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt)
	return err
}

func (s *Store) GetAdminSession(ctx context.Context, tokenHash string) (*AdminSession, error) {
	sess := &AdminSession{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at
		 FROM admin_sessions WHERE token_hash = $1`, tokenHash).
		Scan(&sess.ID, &sess.UserID, &sess.TokenHash, &sess.ExpiresAt, &sess.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *Store) DeleteAdminSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM admin_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (s *Store) CleanExpiredAdminSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM admin_sessions WHERE expires_at < now()`)
	return err
}

// ── Server Config ──

func (s *Store) GetServerConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM server_config WHERE key = $1`, key).
		Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) GetAllServerConfig(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value FROM server_config`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (s *Store) SetServerConfig(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO server_config (key, value, updated_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (key)
		 DO UPDATE SET value = $2, updated_at = now()`,
		key, value)
	return err
}
