package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"scheduler-service/internal/models"
	"scheduler-service/internal/service"
)

type AvailabilityHandlers struct {
	DB      *pgxpool.Pool
	AvailSv *service.AvailabilityService
	BookSv  *service.BookingService
}

// POST /users/:id/availability
func (h *AvailabilityHandlers) SetAvailability(c *gin.Context) {
	userID := c.Param("id")
	var payload []models.AvailabilityRule
	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	saved, err := h.AvailSv.SetAvailability(c.Request.Context(), userID, payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Only include created_at_utc in response
	type CreatedAvailability struct {
		ID             string    `json:"id"`
		UserID         string    `json:"user_id"`
		Type           string    `json:"type,omitempty"`
		DaysOfWeek     []int     `json:"days_of_week"`
		StartTime      string    `json:"start_time"`
		EndTime        string    `json:"end_time"`
		SlotLengthMins int       `json:"slot_length_minutes"`
		Title          string    `json:"title,omitempty"`
		Available      bool      `json:"available"`
		IsRecurring    bool      `json:"is_recurring"`
		CreatedAtUTC   time.Time `json:"created_at_utc"`
	}
	var filtered []CreatedAvailability
	for _, rule := range saved {
		filtered = append(filtered, CreatedAvailability{
			ID:             rule.ID,
			UserID:         rule.UserID,
			Type:           rule.Type,
			DaysOfWeek:     rule.DaysOfWeek,
			StartTime:      rule.StartTime,
			EndTime:        rule.EndTime,
			SlotLengthMins: rule.SlotLengthMins,
			Title:          rule.Title,
			Available:      rule.Available,
			IsRecurring:    rule.IsRecurring,
			CreatedAtUTC:   rule.CreatedAt,
		})
	}
	c.JSON(http.StatusCreated, filtered)
}

// PUT /users/:id/availability
// Accepts array of availability rules (same format as POST)
func (h *AvailabilityHandlers) UpdateAvailability(c *gin.Context) {
	userID := c.Param("id")
	var payload []models.AvailabilityRule
	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var updated []models.AvailabilityRule
	for _, rule := range payload {
		if rule.ID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id is required for each rule in update"})
			return
		}
		res, err := h.AvailSv.UpdateAvailability(c.Request.Context(), userID, rule.ID, &rule)
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("availability rule %s not found", rule.ID)})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		updated = append(updated, *res)
	}

	// Only include updated_at_utc in response
	type UpdatedAvailability struct {
		ID             string    `json:"id"`
		UserID         string    `json:"user_id"`
		Type           string    `json:"type,omitempty"`
		DaysOfWeek     []int     `json:"days_of_week"`
		StartTime      string    `json:"start_time"`
		EndTime        string    `json:"end_time"`
		SlotLengthMins int       `json:"slot_length_minutes"`
		Title          string    `json:"title,omitempty"`
		Available      bool      `json:"available"`
		IsRecurring    bool      `json:"is_recurring"`
		UpdatedAtUTC   time.Time `json:"updated_at_utc"`
	}
	var filtered []UpdatedAvailability
	for _, rule := range updated {
		filtered = append(filtered, UpdatedAvailability{
			ID:             rule.ID,
			UserID:         rule.UserID,
			Type:           rule.Type,
			DaysOfWeek:     rule.DaysOfWeek,
			StartTime:      rule.StartTime,
			EndTime:        rule.EndTime,
			SlotLengthMins: rule.SlotLengthMins,
			Title:          rule.Title,
			Available:      rule.Available,
			IsRecurring:    rule.IsRecurring,
			UpdatedAtUTC:   rule.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, filtered)
}

// DELETE /users/:id/availability/:rule_id
func (h *AvailabilityHandlers) DeleteAvailability(c *gin.Context) {
	userID := c.Param("id")
	ruleID := c.Param("rule_id")

	err := h.AvailSv.DeleteAvailability(c.Request.Context(), userID, ruleID)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "availability rule not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "availability rule deleted successfully"})
}

// GET /users/:id/availability
func (h *AvailabilityHandlers) ListAvailability(c *gin.Context) {
	userID := c.Param("id")
	rules, err := h.AvailSv.ListAvailability(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

// GET /users/:id/slots?from=ISO&to=ISO
func (h *AvailabilityHandlers) GetSlots(c *gin.Context) {
	userID := c.Param("id")
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to required (ISO8601)"})
		return
	}
	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"})
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"})
		return
	}
	if !from.Before(to) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from must be before to"})
		return
	}
	slots, err := h.AvailSv.GenerateAvailableSlots(c.Request.Context(), userID, from.UTC(), to.UTC())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(slots) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "No available slots found for the specified date range",
			"slots":    []service.Slot{},
		})
		return
	}
	c.JSON(http.StatusOK, slots)
}

type createBookingReq struct {
	UserID         string `json:"user_id"`
	CandidateEmail string `json:"candidate_email" binding:"required,email"`
	StartAtUTCStr  string `json:"start_at_utc" binding:"required"`
	EndAtUTCStr    string `json:"end_at_utc" binding:"required"`
	Source         string `json:"source,omitempty"`
	Type           string `json:"type,omitempty"`
	Description    string `json:"description,omitempty"`
	Title          string `json:"title,omitempty"`
}

// GET /users/:id/bookings
func (h *AvailabilityHandlers) ListBookings(c *gin.Context) {
	userID := c.Param("id")
	fromStr := c.Query("from")
	toStr := c.Query("to")

	ctx := c.Request.Context()

	var (
		from time.Time
		to   time.Time
		err  error
	)

	if fromStr != "" && toStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from"})
			return
		}
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to"})
			return
		}
		if !from.Before(to) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from must be before to"})
			return
		}
	}

	bookings, err := h.BookSv.ListBookings(ctx, userID, from, to, fromStr != "" && toStr != "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bookings)
}

// POST /users/:id/bookings
func (h *AvailabilityHandlers) CreateBooking(c *gin.Context) {
	userID := c.Param("id")
	var req createBookingReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start, err := time.Parse(time.RFC3339, req.StartAtUTCStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_at_utc"})
		return
	}
	end, err := time.Parse(time.RFC3339, req.EndAtUTCStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_at_utc"})
		return
	}
	if !start.Before(end) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start must be before end"})
		return
	}

	booking, err := h.BookSv.CreateBooking(c.Request.Context(), userID, serviceCreateReq(req, start, end))
	if err != nil {
		if err.Error() == "slot already booked" {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "slot not available" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"id":              booking.ID,
		"user_id":         booking.UserID,
		"candidate_email": booking.CandidateEmail,
		"status":          booking.Status,
		"start_at_utc":    booking.StartAtUTC,
		"end_at_utc":      booking.EndAtUTC,
		"created_at":      booking.CreatedAt,
	}
	if req.Source != "" {
		response["source"] = req.Source
	}
	if req.Type != "" {
		response["type"] = req.Type
	}
	if req.Description != "" {
		response["description"] = req.Description
	}
	if req.Title != "" {
		response["title"] = req.Title
	}

	c.JSON(http.StatusCreated, response)
}

// DELETE /bookings/:id
func (h *AvailabilityHandlers) CancelBooking(c *gin.Context) {
	id := c.Param("id")
	if err := h.BookSv.CancelBooking(c.Request.Context(), id); err != nil {
		if err == pgx.ErrNoRows || err.Error() == "booking not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "booking not found"})
			return
		}
		if err.Error() == "already cancelled" {
			c.JSON(http.StatusConflict, gin.H{"error": "booking not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func validateAvailabilityRule(rule *models.AvailabilityRule) error {
	return serviceValidateAvailabilityRule(rule)
}
func serviceValidateAvailabilityRule(rule *models.AvailabilityRule) error {
	startTime, err := time.Parse("15:04", rule.StartTime)
	if err != nil {
		return err
	}
	endTime, err := time.Parse("15:04", rule.EndTime)
	if err != nil {
		return err
	}
	if !endTime.After(startTime) {
		return pgx.ErrNoRows
	}
	return nil
}

func serviceCreateReq(req createBookingReq, start, end time.Time) service.CreateBookingParams {
	return service.CreateBookingParams{
		CandidateEmail: req.CandidateEmail,
		Start:          start,
		End:            end,
		Source:         req.Source,
		Type:           req.Type,
		Description:    req.Description,
		Title:          req.Title,
	}
}
