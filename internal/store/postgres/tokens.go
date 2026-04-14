package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/snowskeleton/igg-server/internal/model"
)

// ── Magic tokens ──

func (s *Store) CreateMagicToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO magic_tokens (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		userID, token, expiresAt)
	return err
}

func (s *Store) GetMagicToken(ctx context.Context, token string) (*model.MagicToken, error) {
	t := &model.MagicToken{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, token, expires_at, used, created_at FROM magic_tokens WHERE token = $1`, token).
		Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.Used, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get magic token: %w", err)
	}
	return t, nil
}

func (s *Store) MarkMagicTokenUsed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE magic_tokens SET used = true WHERE id = $1`, id)
	return err
}

// ── Refresh tokens ──

func (s *Store) CreateRefreshToken(ctx context.Context, userID, tokenHash string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash) VALUES ($1, $2)`,
		userID, tokenHash)
	return err
}

func (s *Store) GetRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	t := &model.RefreshToken{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, revoked, created_at FROM refresh_tokens WHERE token_hash = $1`, tokenHash).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Revoked, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return t, nil
}

func (s *Store) RevokeRefreshToken(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked = true WHERE id = $1`, id)
	return err
}

func (s *Store) RevokeAllRefreshTokens(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked = true WHERE user_id = $1`, userID)
	return err
}
