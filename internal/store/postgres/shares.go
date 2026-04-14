package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/snowskeleton/igg-server/internal/model"
)

func (s *Store) CreateShare(ctx context.Context, share *model.CarShare) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO car_shares (car_id, shared_by_id, shared_with_id, invited_email, status, token)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		share.CarID, share.SharedByID, share.SharedWithID, share.InvitedEmail, share.Status, share.Token)
	return err
}

func (s *Store) GetShareByID(ctx context.Context, id string) (*model.CarShare, error) {
	cs := &model.CarShare{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, car_id, shared_by_id, shared_with_id, invited_email, status, token, created_at, updated_at
		 FROM car_shares WHERE id = $1`, id).
		Scan(&cs.ID, &cs.CarID, &cs.SharedByID, &cs.SharedWithID, &cs.InvitedEmail, &cs.Status, &cs.Token, &cs.CreatedAt, &cs.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get share: %w", err)
	}
	return cs, nil
}

func (s *Store) GetShareByToken(ctx context.Context, token string) (*model.CarShare, error) {
	cs := &model.CarShare{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, car_id, shared_by_id, shared_with_id, invited_email, status, token, created_at, updated_at
		 FROM car_shares WHERE token = $1`, token).
		Scan(&cs.ID, &cs.CarID, &cs.SharedByID, &cs.SharedWithID, &cs.InvitedEmail, &cs.Status, &cs.Token, &cs.CreatedAt, &cs.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get share by token: %w", err)
	}
	return cs, nil
}

func (s *Store) GetSharesForCar(ctx context.Context, carID string) ([]model.CarShare, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, car_id, shared_by_id, shared_with_id, invited_email, status, token, created_at, updated_at
		 FROM car_shares WHERE car_id = $1 ORDER BY created_at`, carID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.CarShare
	for rows.Next() {
		var cs model.CarShare
		if err := rows.Scan(&cs.ID, &cs.CarID, &cs.SharedByID, &cs.SharedWithID, &cs.InvitedEmail, &cs.Status, &cs.Token, &cs.CreatedAt, &cs.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

func (s *Store) GetPendingSharesForEmail(ctx context.Context, email string) ([]model.CarShare, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, car_id, shared_by_id, shared_with_id, invited_email, status, token, created_at, updated_at
		 FROM car_shares WHERE invited_email = $1 AND status = 'pending' ORDER BY created_at`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.CarShare
	for rows.Next() {
		var cs model.CarShare
		if err := rows.Scan(&cs.ID, &cs.CarID, &cs.SharedByID, &cs.SharedWithID, &cs.InvitedEmail, &cs.Status, &cs.Token, &cs.CreatedAt, &cs.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

func (s *Store) UpdateShareStatus(ctx context.Context, id, status string, sharedWithID *string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE car_shares SET status = $2, shared_with_id = COALESCE($3, shared_with_id), updated_at = now() WHERE id = $1`,
		id, status, sharedWithID)
	return err
}

// GetSharesOwnedByUser gets all shares for cars owned by userID.
func (s *Store) GetSharesOwnedByUser(ctx context.Context, userID string) ([]model.CarShare, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cs.id, cs.car_id, cs.shared_by_id, cs.shared_with_id, cs.invited_email, cs.status, cs.token, cs.created_at, cs.updated_at
		 FROM car_shares cs
		 JOIN cars c ON cs.car_id = c.id
		 WHERE c.owner_id = $1
		 ORDER BY cs.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.CarShare
	for rows.Next() {
		var cs model.CarShare
		if err := rows.Scan(&cs.ID, &cs.CarID, &cs.SharedByID, &cs.SharedWithID, &cs.InvitedEmail, &cs.Status, &cs.Token, &cs.CreatedAt, &cs.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

// GetSharesReceivedByUser returns shares where the user is the invited recipient.
func (s *Store) GetSharesReceivedByUser(ctx context.Context, userID string) ([]model.CarShare, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cs.id, cs.car_id, cs.shared_by_id, cs.shared_with_id, cs.invited_email, cs.status, cs.token, cs.created_at, cs.updated_at
		 FROM car_shares cs
		 WHERE cs.shared_with_id = $1
		 ORDER BY cs.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.CarShare
	for rows.Next() {
		var cs model.CarShare
		if err := rows.Scan(&cs.ID, &cs.CarID, &cs.SharedByID, &cs.SharedWithID, &cs.InvitedEmail, &cs.Status, &cs.Token, &cs.CreatedAt, &cs.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}
