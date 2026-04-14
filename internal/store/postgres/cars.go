package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/snowskeleton/igg-server/internal/model"
)

func (s *Store) UpsertCar(ctx context.Context, tx *sql.Tx, c *model.Car) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO cars (id, owner_id, make, model, name, plate, vin, year, starting_odometer, pinned, deleted, archived, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		 ON CONFLICT (id) DO UPDATE SET
		   make=$3, model=$4, name=$5, plate=$6, vin=$7, year=$8, starting_odometer=$9,
		   pinned=$10, deleted=$11, archived=$12, updated_at=$14
		 WHERE cars.updated_at < $14`,
		c.ID, c.OwnerID, c.Make, c.Model, c.Name, c.Plate, c.VIN, c.Year,
		c.StartingOdometer, c.Pinned, c.Deleted, c.Archived, c.CreatedAt, c.UpdatedAt)
	return err
}

func (s *Store) GetCarByID(ctx context.Context, id string) (*model.Car, error) {
	c := &model.Car{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, owner_id, make, model, name, plate, vin, year, starting_odometer, pinned, deleted, archived, created_at, updated_at
		 FROM cars WHERE id = $1`, id).
		Scan(&c.ID, &c.OwnerID, &c.Make, &c.Model, &c.Name, &c.Plate, &c.VIN, &c.Year,
			&c.StartingOdometer, &c.Pinned, &c.Deleted, &c.Archived, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get car: %w", err)
	}
	return c, nil
}

func (s *Store) GetCarsForUser(ctx context.Context, userID string, since time.Time) ([]model.Car, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.owner_id, c.make, c.model, c.name, c.plate, c.vin, c.year, c.starting_odometer, c.pinned, c.deleted, c.archived, c.created_at, c.updated_at
		 FROM cars c
		 WHERE c.updated_at > $2 AND (
		   c.owner_id = $1
		   OR c.id IN (SELECT car_id FROM car_shares WHERE shared_with_id = $1 AND status = 'accepted')
		 )`, userID, since)
	if err != nil {
		return nil, fmt.Errorf("get cars for user: %w", err)
	}
	defer rows.Close()

	var cars []model.Car
	for rows.Next() {
		var c model.Car
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.Make, &c.Model, &c.Name, &c.Plate, &c.VIN, &c.Year,
			&c.StartingOdometer, &c.Pinned, &c.Deleted, &c.Archived, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cars = append(cars, c)
	}
	return cars, rows.Err()
}

// GetAccessibleCarIDs returns all car IDs the user owns or has accepted shares for.
func (s *Store) GetAccessibleCarIDs(ctx context.Context, userID string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM cars WHERE owner_id = $1
		 UNION
		 SELECT car_id FROM car_shares WHERE shared_with_id = $1 AND status = 'accepted'`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		m[id] = true
	}
	return m, rows.Err()
}

// GetOwnedCarIDs returns car IDs the user owns.
func (s *Store) GetOwnedCarIDs(ctx context.Context, userID string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM cars WHERE owner_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		m[id] = true
	}
	return m, rows.Err()
}
