package postgres

import "context"

func (s *Store) CleanExpiredMagicTokens(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM magic_tokens WHERE expires_at < now() OR used = true`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) CleanRevokedRefreshTokens(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM refresh_tokens WHERE revoked = true`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) CleanOldNotificationLogs(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM notification_log WHERE created_at < now() - interval '30 days'`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
