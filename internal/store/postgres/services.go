package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/snowskeleton/igg-server/internal/model"
)

func (s *Store) UpsertService(ctx context.Context, tx *sql.Tx, svc *model.Service) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO services (id, car_id, cost, date, pending, name, full_description, odometer, is_fuel, is_full_tank, gallons, vendor_name, deleted, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		 ON CONFLICT (id) DO UPDATE SET
		   cost=$3, date=$4, pending=$5, name=$6, full_description=$7, odometer=$8,
		   is_fuel=$9, is_full_tank=$10, gallons=$11, vendor_name=$12, deleted=$13, updated_at=$15
		 WHERE services.updated_at < $15`,
		svc.ID, svc.CarID, svc.Cost, svc.Date, svc.Pending, svc.Name, svc.FullDescription,
		svc.Odometer, svc.IsFuel, svc.IsFullTank, svc.Gallons, svc.VendorName, svc.Deleted,
		svc.CreatedAt, svc.UpdatedAt)
	return err
}

func (s *Store) GetServicesForCars(ctx context.Context, carIDs []string, since time.Time) ([]model.Service, error) {
	if len(carIDs) == 0 {
		return nil, nil
	}
	query, args := inQuery(
		`SELECT id, car_id, cost, date, pending, name, full_description, odometer, is_fuel, is_full_tank, gallons, vendor_name, deleted, created_at, updated_at
		 FROM services WHERE updated_at > $1 AND car_id IN (`, since, carIDs)
	query += `) ORDER BY updated_at`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get services: %w", err)
	}
	defer rows.Close()

	var out []model.Service
	for rows.Next() {
		var svc model.Service
		if err := rows.Scan(&svc.ID, &svc.CarID, &svc.Cost, &svc.Date, &svc.Pending, &svc.Name,
			&svc.FullDescription, &svc.Odometer, &svc.IsFuel, &svc.IsFullTank, &svc.Gallons,
			&svc.VendorName, &svc.Deleted, &svc.CreatedAt, &svc.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, svc)
	}
	return out, rows.Err()
}

// inQuery builds a parameterized IN clause. firstArg is $1, then carIDs start at $2.
func inQuery(prefix string, firstArg interface{}, ids []string) (string, []interface{}) {
	args := []interface{}{firstArg}
	q := prefix
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += fmt.Sprintf("$%d", i+2)
		args = append(args, id)
	}
	return q, args
}
