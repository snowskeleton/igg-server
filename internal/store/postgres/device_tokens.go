package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/snowskeleton/igg-server/internal/model"
)

func (s *Store) UpsertDeviceToken(ctx context.Context, dt *model.DeviceToken) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO device_tokens (user_id, device_id, token, platform, notify_mode)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, device_id)
		 DO UPDATE SET token = $3, platform = $4, notify_mode = $5, updated_at = now()`,
		dt.UserID, dt.DeviceID, dt.Token, dt.Platform, dt.NotifyMode)
	return err
}

func (s *Store) DeleteDeviceToken(ctx context.Context, userID, deviceID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM device_tokens WHERE user_id = $1 AND device_id = $2`,
		userID, deviceID)
	return err
}

func (s *Store) DeleteAllDeviceTokensForUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM device_tokens WHERE user_id = $1`, userID)
	return err
}

func (s *Store) DeleteDeviceTokenByToken(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM device_tokens WHERE token = $1`, token)
	return err
}

// GetDeviceTokensForUsers returns device tokens for the given user IDs,
// excluding tokens belonging to the specified device ID (the one that triggered the sync).
func (s *Store) GetDeviceTokensForUsers(ctx context.Context, userIDs []string, excludeDeviceID string) ([]model.DeviceToken, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(userIDs))
	args := make([]any, len(userIDs)+1)
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	args[len(userIDs)] = excludeDeviceID

	query := fmt.Sprintf(
		`SELECT id, user_id, device_id, token, platform, notify_mode, created_at, updated_at
		 FROM device_tokens
		 WHERE user_id IN (%s) AND device_id != $%d`,
		strings.Join(placeholders, ","), len(userIDs)+1)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get device tokens: %w", err)
	}
	defer rows.Close()

	var out []model.DeviceToken
	for rows.Next() {
		var dt model.DeviceToken
		if err := rows.Scan(&dt.ID, &dt.UserID, &dt.DeviceID, &dt.Token, &dt.Platform, &dt.NotifyMode, &dt.CreatedAt, &dt.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, dt)
	}
	return out, rows.Err()
}

// GetUsersWithAccessToCars returns user IDs of owners and accepted sharers for the given car IDs,
// excluding the specified user ID.
func (s *Store) GetUsersWithAccessToCars(ctx context.Context, carIDs []string, excludeUserID string) ([]string, error) {
	if len(carIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(carIDs))
	args := make([]any, len(carIDs)+1)
	for i, id := range carIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	args[len(carIDs)] = excludeUserID
	excludePlaceholder := fmt.Sprintf("$%d", len(carIDs)+1)
	inClause := strings.Join(placeholders, ",")

	query := fmt.Sprintf(
		`SELECT DISTINCT user_id FROM (
			SELECT owner_id AS user_id FROM cars WHERE id IN (%s)
			UNION
			SELECT shared_with_id AS user_id FROM car_shares
			WHERE car_id IN (%s) AND status = 'accepted' AND shared_with_id IS NOT NULL
		) sub WHERE user_id != %s`,
		inClause, inClause, excludePlaceholder)

	// Double the carID args for both subqueries
	fullArgs := make([]any, 0, 2*len(carIDs)+1)
	for _, id := range carIDs {
		fullArgs = append(fullArgs, id)
	}
	for _, id := range carIDs {
		fullArgs = append(fullArgs, id)
	}
	fullArgs = append(fullArgs, excludeUserID)

	// Fix placeholders for the second subquery
	placeholders2 := make([]string, len(carIDs))
	for i := range carIDs {
		placeholders2[i] = fmt.Sprintf("$%d", len(carIDs)+i+1)
	}
	excludePlaceholder = fmt.Sprintf("$%d", 2*len(carIDs)+1)

	query = fmt.Sprintf(
		`SELECT DISTINCT user_id FROM (
			SELECT owner_id AS user_id FROM cars WHERE id IN (%s)
			UNION
			SELECT shared_with_id AS user_id FROM car_shares
			WHERE car_id IN (%s) AND status = 'accepted' AND shared_with_id IS NOT NULL
		) sub WHERE user_id != %s`,
		strings.Join(placeholders, ","),
		strings.Join(placeholders2, ","),
		excludePlaceholder)

	rows, err := s.db.QueryContext(ctx, query, fullArgs...)
	if err != nil {
		return nil, fmt.Errorf("get users with access: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, uid)
	}
	return userIDs, rows.Err()
}
