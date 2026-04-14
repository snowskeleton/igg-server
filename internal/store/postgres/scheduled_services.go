package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/snowskeleton/igg-server/internal/model"
)

func (s *Store) UpsertScheduledService(ctx context.Context, tx *sql.Tx, ss *model.ScheduledService) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO scheduled_services (id, car_id, name, full_description, notification_uuid, repeating, odometer_first_occurance, frequency_miles, frequency_time, frequency_time_interval, frequency_time_start, deleted, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		 ON CONFLICT (id) DO UPDATE SET
		   name=$3, full_description=$4, notification_uuid=$5, repeating=$6,
		   odometer_first_occurance=$7, frequency_miles=$8, frequency_time=$9,
		   frequency_time_interval=$10, frequency_time_start=$11, deleted=$12, updated_at=$14
		 WHERE scheduled_services.updated_at < $14`,
		ss.ID, ss.CarID, ss.Name, ss.FullDescription, ss.NotificationUUID, ss.Repeating,
		ss.OdometerFirstOccurance, ss.FrequencyMiles, ss.FrequencyTime,
		ss.FrequencyTimeInterval, ss.FrequencyTimeStart, ss.Deleted, ss.CreatedAt, ss.UpdatedAt)
	return err
}

func (s *Store) GetScheduledServicesForCars(ctx context.Context, carIDs []string, since time.Time) ([]model.ScheduledService, error) {
	if len(carIDs) == 0 {
		return nil, nil
	}
	query, args := inQuery(
		`SELECT id, car_id, name, full_description, notification_uuid, repeating, odometer_first_occurance, frequency_miles, frequency_time, frequency_time_interval, frequency_time_start, deleted, created_at, updated_at
		 FROM scheduled_services WHERE updated_at > $1 AND car_id IN (`, since, carIDs)
	query += `) ORDER BY updated_at`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get scheduled services: %w", err)
	}
	defer rows.Close()

	var out []model.ScheduledService
	for rows.Next() {
		var ss model.ScheduledService
		if err := rows.Scan(&ss.ID, &ss.CarID, &ss.Name, &ss.FullDescription, &ss.NotificationUUID,
			&ss.Repeating, &ss.OdometerFirstOccurance, &ss.FrequencyMiles, &ss.FrequencyTime,
			&ss.FrequencyTimeInterval, &ss.FrequencyTimeStart, &ss.Deleted, &ss.CreatedAt, &ss.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, ss)
	}
	return out, rows.Err()
}
