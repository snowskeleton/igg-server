package postgres

import (
	"context"
	"time"
)

type DashboardStats struct {
	UserCount         int
	CarCount          int
	ServiceCount      int
	DeviceTokenCount  int
	ShareCount        int
}

func (s *Store) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}

	queries := []struct {
		sql  string
		dest *int
	}{
		{`SELECT COUNT(*) FROM users`, &stats.UserCount},
		{`SELECT COUNT(*) FROM cars WHERE deleted = false`, &stats.CarCount},
		{`SELECT COUNT(*) FROM services WHERE deleted = false`, &stats.ServiceCount},
		{`SELECT COUNT(*) FROM device_tokens`, &stats.DeviceTokenCount},
		{`SELECT COUNT(*) FROM car_shares WHERE status = 'accepted'`, &stats.ShareCount},
	}

	for _, q := range queries {
		if err := s.db.QueryRowContext(ctx, q.sql).Scan(q.dest); err != nil {
			return nil, err
		}
	}
	return stats, nil
}

type AdminUser struct {
	ID         string
	Email      string
	CreatedAt  time.Time
	CarCount   int
	LastSyncAt *time.Time
}

func (s *Store) GetAllUsers(ctx context.Context) ([]AdminUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.id, u.email, u.created_at,
		        (SELECT COUNT(*) FROM cars WHERE owner_id = u.id AND deleted = false) AS car_count,
		        (SELECT MAX(cursor_at) FROM sync_cursors WHERE user_id = u.id) AS last_sync
		 FROM users u
		 ORDER BY u.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AdminUser
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(&u.ID, &u.Email, &u.CreatedAt, &u.CarCount, &u.LastSyncAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

type AdminDevice struct {
	DeviceID   string
	Token      string
	Platform   string
	NotifyMode string
	UpdatedAt  time.Time
}

func (s *Store) GetDeviceTokensByUser(ctx context.Context) (map[string][]AdminDevice, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, device_id, token, platform, notify_mode, updated_at
		 FROM device_tokens
		 ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]AdminDevice)
	for rows.Next() {
		var userID string
		var d AdminDevice
		if err := rows.Scan(&userID, &d.DeviceID, &d.Token, &d.Platform, &d.NotifyMode, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out[userID] = append(out[userID], d)
	}
	return out, rows.Err()
}

type AdminNotification struct {
	DeviceID         string
	DeviceTokenPrefix string
	PushType         string
	Success          bool
	ErrorReason      *string
	CreatedAt        time.Time
}

func (s *Store) GetRecentNotificationsByUser(ctx context.Context) (map[string][]AdminNotification, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, device_id, device_token_prefix, push_type, success, error_reason, created_at
		 FROM notification_log
		 WHERE created_at > now() - interval '7 days'
		 ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]AdminNotification)
	for rows.Next() {
		var userID string
		var n AdminNotification
		if err := rows.Scan(&userID, &n.DeviceID, &n.DeviceTokenPrefix, &n.PushType, &n.Success, &n.ErrorReason, &n.CreatedAt); err != nil {
			return nil, err
		}
		out[userID] = append(out[userID], n)
	}
	return out, rows.Err()
}

func (s *Store) LogNotification(ctx context.Context, userID, deviceID, tokenPrefix, pushType string, success bool, errorReason string) {
	var errPtr *string
	if errorReason != "" {
		errPtr = &errorReason
	}
	s.db.ExecContext(ctx,
		`INSERT INTO notification_log (user_id, device_id, device_token_prefix, push_type, success, error_reason)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, deviceID, tokenPrefix, pushType, success, errPtr)
}

type AdminCar struct {
	ID           string
	Name         string
	Make         string
	Model        string
	Year         *int
	OwnerEmail   string
	ServiceCount int
	ShareCount   int
	Deleted      bool
	CreatedAt    time.Time
}

func (s *Store) GetAllCars(ctx context.Context) ([]AdminCar, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.name, c.make, c.model, c.year,
		        u.email AS owner_email,
		        (SELECT COUNT(*) FROM services WHERE car_id = c.id AND deleted = false) AS svc_count,
		        (SELECT COUNT(*) FROM car_shares WHERE car_id = c.id AND status = 'accepted') AS share_count,
		        c.deleted,
		        c.created_at
		 FROM cars c
		 JOIN users u ON u.id = c.owner_id
		 ORDER BY c.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AdminCar
	for rows.Next() {
		var c AdminCar
		if err := rows.Scan(&c.ID, &c.Name, &c.Make, &c.Model, &c.Year,
			&c.OwnerEmail, &c.ServiceCount, &c.ShareCount, &c.Deleted, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
