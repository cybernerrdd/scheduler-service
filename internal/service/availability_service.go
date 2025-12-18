package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"scheduler-service/internal/models"
	"scheduler-service/internal/repository"
)

type AvailabilityService struct {
	DB    repository.Querier
	Avail repository.AvailabilityRepository
	Book  repository.BookingRepository
}

type Slot struct {
	StartUTC time.Time `json:"start_utc"`
	EndUTC   time.Time `json:"end_utc"`
	Event    string    `json:"event,omitempty"` // Event identifier for the slot
}

func NewAvailabilityService(db repository.Querier, ar repository.AvailabilityRepository, br repository.BookingRepository) *AvailabilityService {
	return &AvailabilityService{DB: db, Avail: ar, Book: br}
}

func (s *AvailabilityService) SetAvailability(ctx context.Context, userID string, rules []models.AvailabilityRule) ([]models.AvailabilityRule, error) {
	var saved []models.AvailabilityRule
	for i := range rules {
		rules[i].UserID = userID
		now := time.Now().UTC()
		rules[i].CreatedAt = now
		rules[i].UpdatedAt = now
		if err := validateAvailabilityRule(&rules[i]); err != nil {
			return nil, err
		}
		if err := s.Avail.InsertAvailabilityRule(ctx, s.DB, &rules[i]); err != nil {
			return nil, err
		}
		saved = append(saved, rules[i])
	}
	return saved, nil
}

func (s *AvailabilityService) UpdateAvailability(ctx context.Context, userID, ruleID string, rule *models.AvailabilityRule) (*models.AvailabilityRule, error) {
	// Fetch existing rule first
	existing, err := s.Avail.GetAvailabilityRule(ctx, s.DB, userID, ruleID)
	if err != nil {
		return nil, err
	}
	// If days_of_week is empty, preserve the original value
	if len(rule.DaysOfWeek) == 0 {
		rule.DaysOfWeek = existing.DaysOfWeek
	}
	// If start_time is empty, preserve the original value
	if rule.StartTime == "" {
		rule.StartTime = existing.StartTime
	}
	// If end_time is empty, preserve the original value
	if rule.EndTime == "" {
		rule.EndTime = existing.EndTime
	}
	// If slot_length_minutes is zero, preserve the original value
	if rule.SlotLengthMins == 0 {
		rule.SlotLengthMins = existing.SlotLengthMins
	}
	if err := validateAvailabilityRule(rule); err != nil {
		return nil, err
	}
	id, err := s.Avail.UpdateAvailabilityRule(ctx, s.DB, userID, ruleID, rule)
	if err != nil {
		return nil, err
	}
	// Fetch the updated record from database to get correct timestamps
	updatedRule, err := s.Avail.GetAvailabilityRule(ctx, s.DB, userID, id)
	if err != nil {
		return nil, err
	}
	return updatedRule, nil
}

func (s *AvailabilityService) DeleteAvailability(ctx context.Context, userID, ruleID string) error {
	return s.Avail.DeleteAvailabilityRule(ctx, s.DB, userID, ruleID)
}

func (s *AvailabilityService) ListAvailability(ctx context.Context, userID string) ([]models.AvailabilityRule, error) {
	return s.Avail.ListAvailabilityRules(ctx, s.DB, userID)
}

func (s *AvailabilityService) ListBookings(ctx context.Context, userID string, from, to time.Time, filtered bool) ([]models.Booking, error) {
	return s.Book.ListBookings(ctx, s.DB, userID, from, to, filtered)
}

func (s *AvailabilityService) GenerateAvailableSlots(ctx context.Context, userID string, fromUTC, toUTC time.Time) ([]Slot, error) {
	rules, err := s.Avail.ListAvailabilityRules(ctx, s.DB, userID)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}

	var candidate []Slot
	startDate := fromUTC.Truncate(24 * time.Hour)
	endDate := toUTC.Truncate(24 * time.Hour)
	
	for day := startDate; !day.After(endDate); day = day.Add(24 * time.Hour) {
		dayOfWeek := int(day.Weekday())
		
		for _, r := range rules {
			// Check if this day matches any of the days in the rule
			dayMatches := false
			for _, ruleDay := range r.DaysOfWeek {
				if dayOfWeek == ruleDay {
					dayMatches = true
					break
				}
			}
			if !dayMatches {
				continue
			}
			
			// Handle recurring logic
			if !r.IsRecurring {
				// For non-recurring rules, only generate slots for the week containing created_at
				ruleWeekStart := getWeekStart(r.CreatedAt)
				currentWeekStart := getWeekStart(day)
				if !ruleWeekStart.Equal(currentWeekStart) {
					continue // Skip if not in the same week as when rule was created
				}
			}
			
			startTOD, err := parseHHMM(r.StartTime)
			if err != nil {
				return nil, err
			}
			endTOD, err := parseHHMM(r.EndTime)
			if err != nil {
				return nil, err
			}
			if !endTOD.After(startTOD) {
				return nil, fmt.Errorf("end_time must be after start_time for rule %s", r.ID)
			}
			y, m, d := day.Date()
			utcStart := time.Date(y, m, d, startTOD.Hour(), startTOD.Minute(), 0, 0, time.UTC)
			utcEnd := time.Date(y, m, d, endTOD.Hour(), endTOD.Minute(), 0, 0, time.UTC)
			slotLen := time.Duration(r.SlotLengthMins) * time.Minute
			for s0 := utcStart; s0.Add(slotLen).Equal(utcEnd) || s0.Add(slotLen).Before(utcEnd); s0 = s0.Add(slotLen) {
				startUTC := s0
				endUTC := s0.Add(slotLen)
				if !endUTC.After(fromUTC) || !startUTC.Before(toUTC) {
					continue
				}
				if !r.Available {
					continue
				}
				candidate = append(candidate, Slot{StartUTC: startUTC, EndUTC: endUTC, Event: r.Event})
			}
		}
	}
	bookings, err := s.Book.ListBookingsInRange(ctx, s.DB, userID, fromUTC.Add(-1*time.Hour), toUTC.Add(1*time.Hour))
	if err != nil {
		return nil, err
	}
	booked := map[int64]struct{}{}
	for _, b := range bookings {
		booked[b.StartAtUTC.Unix()] = struct{}{}
	}
	var available []Slot
	for _, sl := range candidate {
		if _, ok := booked[sl.StartUTC.Unix()]; !ok {
			available = append(available, sl)
		}
	}
	return available, nil
}

// GenerateAvailableSlotsByEvent generates available slots filtered by event
func (s *AvailabilityService) GenerateAvailableSlotsByEvent(ctx context.Context, userID, event string, fromUTC, toUTC time.Time) ([]Slot, error) {
	rules, err := s.Avail.ListAvailabilityRulesByEvent(ctx, s.DB, userID, event)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}

	var candidate []Slot
	startDate := fromUTC.Truncate(24 * time.Hour)
	endDate := toUTC.Truncate(24 * time.Hour)
	
	for day := startDate; !day.After(endDate); day = day.Add(24 * time.Hour) {
		dayOfWeek := int(day.Weekday())
		
		for _, r := range rules {
			// Check if this day matches any of the days in the rule
			dayMatches := false
			for _, ruleDay := range r.DaysOfWeek {
				if dayOfWeek == ruleDay {
					dayMatches = true
					break
				}
			}
			if !dayMatches {
				continue
			}
			
			// Handle recurring logic
			if !r.IsRecurring {
				// For non-recurring rules, only generate slots for the week containing created_at
				ruleWeekStart := getWeekStart(r.CreatedAt)
				currentWeekStart := getWeekStart(day)
				if !ruleWeekStart.Equal(currentWeekStart) {
					continue // Skip if not in the same week as when rule was created
				}
			}
			
			startTOD, err := parseHHMM(r.StartTime)
			if err != nil {
				return nil, err
			}
			endTOD, err := parseHHMM(r.EndTime)
			if err != nil {
				return nil, err
			}
			if !endTOD.After(startTOD) {
				return nil, fmt.Errorf("end_time must be after start_time for rule %s", r.ID)
			}
			y, m, d := day.Date()
			utcStart := time.Date(y, m, d, startTOD.Hour(), startTOD.Minute(), 0, 0, time.UTC)
			utcEnd := time.Date(y, m, d, endTOD.Hour(), endTOD.Minute(), 0, 0, time.UTC)
			slotLen := time.Duration(r.SlotLengthMins) * time.Minute
			for s0 := utcStart; s0.Add(slotLen).Equal(utcEnd) || s0.Add(slotLen).Before(utcEnd); s0 = s0.Add(slotLen) {
				startUTC := s0
				endUTC := s0.Add(slotLen)
				if !endUTC.After(fromUTC) || !startUTC.Before(toUTC) {
					continue
				}
				if !r.Available {
					continue
				}
				candidate = append(candidate, Slot{StartUTC: startUTC, EndUTC: endUTC, Event: r.Event})
			}
		}
	}
	bookings, err := s.Book.ListBookingsInRange(ctx, s.DB, userID, fromUTC.Add(-1*time.Hour), toUTC.Add(1*time.Hour))
	if err != nil {
		return nil, err
	}
	booked := map[int64]struct{}{}
	for _, b := range bookings {
		booked[b.StartAtUTC.Unix()] = struct{}{}
	}
	var available []Slot
	for _, sl := range candidate {
		if _, ok := booked[sl.StartUTC.Unix()]; !ok {
			available = append(available, sl)
		}
	}
	return available, nil
}

// getWeekStart returns the start of the week (Monday) for a given date
func getWeekStart(t time.Time) time.Time {
	// Go's time.Weekday: Sunday=0, Monday=1, ..., Saturday=6
	// We want Monday to be the start of the week
	weekday := int(t.Weekday())
	// Convert: Sunday=0 -> offset=6, Monday=1 -> offset=0, etc.
	offset := (weekday + 6) % 7
	return t.Truncate(24 * time.Hour).AddDate(0, 0, -offset)
}

func validateAvailabilityRule(rule *models.AvailabilityRule) error {
	// Validate days_of_week: must have between 1 and 7 days
	if len(rule.DaysOfWeek) < 1 {
		return errors.New("days_of_week must contain at least 1 day")
	}
	if len(rule.DaysOfWeek) > 7 {
		return errors.New("days_of_week must contain at most 7 days")
	}
	
	// Validate each day is between 0-6 (Sunday=0 to Saturday=6)
	seenDays := make(map[int]bool)
	for _, day := range rule.DaysOfWeek {
		if day < 0 || day > 6 {
			return fmt.Errorf("invalid day_of_week: %d (must be between 0-6)", day)
		}
		if seenDays[day] {
			return fmt.Errorf("duplicate day_of_week: %d", day)
		}
		seenDays[day] = true
	}
	
	// Validate time format and range
	startTime, err := time.Parse("15:04", rule.StartTime)
	if err != nil {
		return fmt.Errorf("invalid start_time format: %w", err)
	}
	endTime, err := time.Parse("15:04", rule.EndTime)
	if err != nil {
		return fmt.Errorf("invalid end_time format: %w", err)
	}
	if !endTime.After(startTime) {
		return errors.New("end_time must be after start_time")
	}
	
	return nil
}

func parseHHMM(s string) (time.Time, error) {
	if len(s) < 5 {
		return time.Time{}, fmt.Errorf("invalid time string: %s", s)
	}
	// Normalize to HH:MM format (handle database format like "09:00:00.000000")
	if len(s) > 5 {
		s = s[:5]
	}
	return time.Parse("15:04", s)
}
