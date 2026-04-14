package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/snowskeleton/igg-server/internal/model"
)

func (s *Store) UpsertCarSettings(ctx context.Context, tx *sql.Tx, cs *model.CarSettings) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO car_settings (id, car_id, user_id, selected_tab, range_days, include_fuel, include_maintenance, include_completed, include_pending, custom, created_at, updated_at)
		 VALUES (COALESCE(NULLIF($1,'')::uuid, gen_random_uuid()), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT (car_id, user_id) DO UPDATE SET
		   selected_tab=$4, range_days=$5, include_fuel=$6, include_maintenance=$7,
		   include_completed=$8, include_pending=$9, custom=$10, updated_at=$12
		 WHERE car_settings.updated_at < $12`,
		cs.ID, cs.CarID, cs.UserID, cs.SelectedTab, cs.RangeDays, cs.IncludeFuel,
		cs.IncludeMaintenance, cs.IncludeCompleted, cs.IncludePending, cs.Custom,
		cs.CreatedAt, cs.UpdatedAt)
	return err
}

func (s *Store) GetCarSettingsForUser(ctx context.Context, userID string, carIDs []string, since time.Time) ([]model.CarSettings, error) {
	if len(carIDs) == 0 {
		return nil, nil
	}
	query := `SELECT id, car_id, user_id, selected_tab, range_days, include_fuel, include_maintenance, include_completed, include_pending, custom, created_at, updated_at
		 FROM car_settings WHERE user_id = $1 AND updated_at > $2 AND car_id IN (`
	args := []interface{}{userID, since}
	for i, id := range carIDs {
		if i > 0 {
			query += ","
		}
		query += fmt.Sprintf("$%d", i+3)
		args = append(args, id)
	}
	query += `) ORDER BY updated_at`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get car settings: %w", err)
	}
	defer rows.Close()

	var out []model.CarSettings
	for rows.Next() {
		var cs model.CarSettings
		if err := rows.Scan(&cs.ID, &cs.CarID, &cs.UserID, &cs.SelectedTab, &cs.RangeDays,
			&cs.IncludeFuel, &cs.IncludeMaintenance, &cs.IncludeCompleted, &cs.IncludePending,
			&cs.Custom, &cs.CreatedAt, &cs.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}
