package models

import (
	"encoding/json"
	"time"
)

type AvailabilityRule struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	DayOfWeek      int       `json:"day_of_week"`
	StartTime      string    `json:"start_time"`
	EndTime        string    `json:"end_time"`
	SlotLengthMins int       `json:"slot_length_minutes"`
	Title          string    `json:"title,omitempty"`
	Available      bool      `json:"available"`
	CreatedAt      time.Time `json:"created_at_utc,omitempty"`
	UpdatedAt      time.Time `json:"updated_at_utc,omitempty"`
}

// MarshalJSON ensures timestamps are serialized in UTC
func (a AvailabilityRule) MarshalJSON() ([]byte, error) {
	type Alias AvailabilityRule
	return json.Marshal(&struct {
		CreatedAtUTC time.Time `json:"created_at_utc,omitempty"`
		UpdatedAtUTC time.Time `json:"updated_at_utc,omitempty"`
		*Alias
	}{
		CreatedAtUTC: a.CreatedAt.UTC(),
		UpdatedAtUTC: a.UpdatedAt.UTC(),
		Alias:        (*Alias)(&a),
	})
}

type Booking struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	CandidateEmail string    `json:"candidate_email"`
	StartAtUTC     time.Time `json:"start_at_utc"`
	EndAtUTC       time.Time `json:"end_at_utc"`
	Status         string    `json:"status"`
	Source         string    `json:"source,omitempty"`
	Type           string    `json:"type,omitempty"`
	Description    string    `json:"description,omitempty"`
	Title          string    `json:"title,omitempty"`
	CreatedAt      time.Time `json:"created_at_utc,omitempty"`
}

// MarshalJSON ensures times are serialized in UTC
func (b Booking) MarshalJSON() ([]byte, error) {
	type Alias Booking
	return json.Marshal(&struct {
		StartAtUTC    time.Time `json:"start_at_utc"`
		EndAtUTC      time.Time `json:"end_at_utc"`
		CreatedAtUTC  time.Time `json:"created_at_utc,omitempty"`
		*Alias
	}{
		StartAtUTC:   b.StartAtUTC.UTC(),
		EndAtUTC:     b.EndAtUTC.UTC(),
		CreatedAtUTC: b.CreatedAt.UTC(),
		Alias:        (*Alias)(&b),
	})
}

type APIKey struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	KeyHash    string    `json:"-"` // Never expose hash in JSON
	CreatedAt  time.Time `json:"created_at_utc,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at_utc,omitempty"`
}

// MarshalJSON ensures timestamps are serialized in UTC
func (a APIKey) MarshalJSON() ([]byte, error) {
	type Alias APIKey
	var lastUsedAtUTC *time.Time
	if a.LastUsedAt != nil {
		utc := a.LastUsedAt.UTC()
		lastUsedAtUTC = &utc
	}
	return json.Marshal(&struct {
		CreatedAtUTC  time.Time  `json:"created_at_utc,omitempty"`
		LastUsedAtUTC *time.Time `json:"last_used_at_utc,omitempty"`
		*Alias
	}{
		CreatedAtUTC:  a.CreatedAt.UTC(),
		LastUsedAtUTC: lastUsedAtUTC,
		Alias:         (*Alias)(&a),
	})
}
