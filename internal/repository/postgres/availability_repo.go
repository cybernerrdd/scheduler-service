package postgres

import (
	"context"
	"time"

	"scheduler-service/internal/models"
	"scheduler-service/internal/repository"
)

type AvailabilityRepo struct{}

func NewAvailabilityRepo() *AvailabilityRepo { return &AvailabilityRepo{} }

func (r *AvailabilityRepo) InsertAvailabilityRule(ctx context.Context, q repository.Querier, ar *models.AvailabilityRule) error {
	now := time.Now().UTC()
	query := `INSERT INTO availability_rules
		(id, user_id, day_of_week, start_time, end_time, slot_length_minutes, title, available, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`
	return q.QueryRow(ctx, query,
		ar.UserID, ar.DayOfWeek, ar.StartTime, ar.EndTime, ar.SlotLengthMins,
		ar.Title, ar.Available, now, now,
	).Scan(&ar.ID)
}

func (r *AvailabilityRepo) GetAvailabilityRule(ctx context.Context, q repository.Querier, userID, ruleID string) (*models.AvailabilityRule, error) {
	query := `SELECT id,user_id,day_of_week,start_time,end_time,slot_length_minutes,title,available,created_at,updated_at
		      FROM availability_rules WHERE id=$1 AND user_id=$2`
	var rule models.AvailabilityRule
	var start, end string
	err := q.QueryRow(ctx, query, ruleID, userID).Scan(
		&rule.ID, &rule.UserID, &rule.DayOfWeek, &start, &end,
		&rule.SlotLengthMins, &rule.Title, &rule.Available, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	rule.StartTime = start
	rule.EndTime = end
	return &rule, nil
}

func (r *AvailabilityRepo) ListAvailabilityRules(ctx context.Context, q repository.Querier, userID string) ([]models.AvailabilityRule, error) {
	query := `SELECT id,user_id,day_of_week,start_time,end_time,slot_length_minutes,title,available,created_at,updated_at
		      FROM availability_rules WHERE user_id=$1 ORDER BY id`
	rows, err := q.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.AvailabilityRule
	for rows.Next() {
		var rule models.AvailabilityRule
		var start, end string
		if err := rows.Scan(&rule.ID, &rule.UserID, &rule.DayOfWeek, &start, &end,
			&rule.SlotLengthMins, &rule.Title, &rule.Available, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rule.StartTime = start
		rule.EndTime = end
		out = append(out, rule)
	}
	return out, nil
}

func (r *AvailabilityRepo) UpdateAvailabilityRule(ctx context.Context, q repository.Querier, userID, ruleID string, ar *models.AvailabilityRule) (string, error) {
	now := time.Now().UTC()
	query := `UPDATE availability_rules
		SET day_of_week=$1, start_time=$2, end_time=$3, slot_length_minutes=$4,
		    title=$5, available=$6, updated_at=$7
		WHERE id=$8 AND user_id=$9
		RETURNING id`
	var updatedID string
	err := q.QueryRow(ctx, query,
		ar.DayOfWeek, ar.StartTime, ar.EndTime, ar.SlotLengthMins,
		ar.Title, ar.Available, now, ruleID, userID,
	).Scan(&updatedID)
	return updatedID, err
}
