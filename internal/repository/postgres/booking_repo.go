package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"scheduler-service/internal/models"
	"scheduler-service/internal/repository"
)

type BookingRepo struct{}

func NewBookingRepo() *BookingRepo { return &BookingRepo{} }

func (r *BookingRepo) ListBookingsInRange(ctx context.Context, q repository.Querier, userID string, from, to repository.AppTime) ([]models.Booking, error) {
	query := `SELECT id,user_id,candidate_email,start_at_utc,end_at_utc,status,created_at 
		      FROM bookings
		      WHERE user_id=$1 AND start_at_utc >= $2 AND start_at_utc < $3 AND status='confirmed'`
	rows, err := q.Query(ctx, query, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Booking
	for rows.Next() {
		var b models.Booking
		if err := rows.Scan(&b.ID, &b.UserID, &b.CandidateEmail, &b.StartAtUTC, &b.EndAtUTC, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func (r *BookingRepo) ListBookings(ctx context.Context, q repository.Querier, userID string, from, to repository.AppTime, filtered bool) ([]models.Booking, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if filtered {
		query := `SELECT id,user_id,candidate_email,start_at_utc,end_at_utc,status,created_at 
		          FROM bookings 
		          WHERE user_id=$1 AND start_at_utc >= $2 AND start_at_utc < $3 AND status != 'cancelled'
		          ORDER BY start_at_utc`
		rows, err = q.Query(ctx, query, userID, from, to)
	} else {
		query := `SELECT id,user_id,candidate_email,start_at_utc,end_at_utc,status,created_at 
		          FROM bookings 
		          WHERE user_id=$1 AND status != 'cancelled'
		          ORDER BY start_at_utc`
		rows, err = q.Query(ctx, query, userID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Booking
	for rows.Next() {
		var b models.Booking
		if err := rows.Scan(&b.ID, &b.UserID, &b.CandidateEmail, &b.StartAtUTC, &b.EndAtUTC, &b.Status, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func (r *BookingRepo) CheckExistingBookingAtStart(ctx context.Context, q repository.Querier, userID string, start repository.AppTime) (string, error) {
	query := `SELECT id FROM bookings 
		       WHERE user_id=$1 AND status='confirmed' 
		       AND start_at_utc = $2 FOR UPDATE`
	var id string
	err := q.QueryRow(ctx, query, userID, start).Scan(&id)
	if err != nil && err != pgx.ErrNoRows {
		return "", err
	}
	return id, err
}

func (r *BookingRepo) InsertBooking(ctx context.Context, q repository.Querier, b *models.Booking) (string, error) {
	query := `INSERT INTO bookings 
		(id, user_id, candidate_email, start_at_utc, end_at_utc, status, source, type, description, title, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, 'confirmed', $5, $6, $7, $8, now())
		RETURNING id`
	var newID string
	err := q.QueryRow(ctx, query, b.UserID, b.CandidateEmail, b.StartAtUTC, b.EndAtUTC, b.Source, b.Type, b.Description, b.Title).Scan(&newID)
	return newID, err
}

func (r *BookingRepo) GetBookingStatus(ctx context.Context, q repository.Querier, id string) (string, error) {
	query := `SELECT status FROM bookings WHERE id=$1`
	var status string
	err := q.QueryRow(ctx, query, id).Scan(&status)
	return status, err
}

func (r *BookingRepo) CancelBooking(ctx context.Context, q repository.Querier, id string) (int64, error) {
	query := `UPDATE bookings SET status='cancelled' WHERE id=$1 AND status != 'cancelled'`
	res, err := q.Exec(ctx, query, id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

// ensure interface satisfaction
var (
	_ = time.Now // silence unused import if needed
)
