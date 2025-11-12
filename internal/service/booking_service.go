package service

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"scheduler-service/internal/models"
	"scheduler-service/internal/repository"
)

type BookingService struct {
	DB    repository.Querier
	Avail *AvailabilityService
	Repo  repository.BookingRepository
}

// NewBookingService wires booking repo and availability service.
func NewBookingService(db repository.Querier, repo repository.BookingRepository, avail *AvailabilityService) *BookingService {
	return &BookingService{DB: db, Repo: repo, Avail: avail}
}

func (s *BookingService) ListBookings(ctx context.Context, userID string, from, to time.Time, filtered bool) ([]models.Booking, error) {
	return s.Repo.ListBookings(ctx, s.DB, userID, from, to, filtered)
}

func (s *BookingService) CreateBooking(ctx context.Context, userID string, req CreateBookingParams) (models.Booking, error) {
	var out models.Booking
	start := req.Start.UTC()
	end := req.End.UTC()

	// Begin transaction from underlying pool if available
	tx, ok := s.DB.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return out, errors.New("db does not support transactions")
	}
	trx, err := tx.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer trx.Rollback(ctx)

	if id, err := s.Repo.CheckExistingBookingAtStart(ctx, trx, userID, start); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	} else if id != "" {
		return out, errors.New("slot already booked")
	}

	slots, err := s.Avail.GenerateAvailableSlots(ctx, userID, start.Add(-1*time.Second), end.Add(1*time.Second))
	if err != nil {
		return out, err
	}
	ok = false
	for _, s := range slots {
		if s.StartUTC.Equal(start) && s.EndUTC.Equal(end) {
			ok = true
			break
		}
	}
	if !ok {
		return out, errors.New("slot not available")
	}

	b := &models.Booking{UserID: userID, CandidateEmail: req.CandidateEmail, StartAtUTC: start, EndAtUTC: end, Source: req.Source, Type: req.Type, Description: req.Description, Title: req.Title, Status: "confirmed", CreatedAt: time.Now().UTC()}
	newID, err := s.Repo.InsertBooking(ctx, trx, b)
	if err != nil {
		return out, err
	}

	if err := trx.Commit(ctx); err != nil {
		return out, err
	}

	out = *b
	out.ID = newID
	return out, nil
}

func (s *BookingService) CancelBooking(ctx context.Context, id string) error {
	status, err := s.Repo.GetBookingStatus(ctx, s.DB, id)
	if err == pgx.ErrNoRows {
		return errors.New("booking not found")
	}
	if err != nil {
		return err
	}
	if status == "cancelled" {
		return errors.New("already cancelled")
	}
	rows, err := s.Repo.CancelBooking(ctx, s.DB, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("booking not found")
	}
	return nil
}

type createBookingRequest struct {
	CandidateEmail string
	Start          time.Time
	End            time.Time
	Source         string
	Type           string
	Description    string
	Title          string
}

type CreateBookingParams struct {
	CandidateEmail string
	Start          time.Time
	End            time.Time
	Source         string
	Type           string
	Description    string
	Title          string
}
