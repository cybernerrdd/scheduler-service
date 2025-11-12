package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"scheduler-service/internal/models"
	"scheduler-service/internal/repository/postgres"
	"scheduler-service/internal/service"
)

// GoogleCalendarConfig holds OAuth2 configuration
type GoogleCalendarConfig struct {
	Config *oauth2.Config
}

// CalendarEvent represents a Google Calendar event
type CalendarEvent struct {
	ID             string          `json:"id"`
	Summary        string          `json:"summary"`
	Description    string          `json:"description,omitempty"`
	StartTime      time.Time       `json:"start_time"`
	EndTime        time.Time       `json:"end_time"`
	Location       string          `json:"location,omitempty"`
	Status         string          `json:"status"`
	Creator        string          `json:"creator,omitempty"`
	MeetingLink    string          `json:"meeting_link,omitempty"`
	ConferenceData *ConferenceInfo `json:"conference_data,omitempty"`
}

// ConferenceInfo represents meeting/conference details
type ConferenceInfo struct {
	Type         string   `json:"type,omitempty"`          // "hangoutsMeet", "zoom", etc.
	URL          string   `json:"url,omitempty"`           // Meeting URL
	ID           string   `json:"id,omitempty"`            // Meeting ID
	PhoneNumbers []string `json:"phone_numbers,omitempty"` // Dial-in numbers
}

// InitGoogleCalendarConfig initializes OAuth2 config for Google Calendar
func InitGoogleCalendarConfig() *GoogleCalendarConfig {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")

	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			calendar.CalendarScope,
		},
		Endpoint: google.Endpoint,
	}

	return &GoogleCalendarConfig{Config: config}
}

// GoogleAuthHandler initiates OAuth2 flow
func (a *App) GoogleAuthHandler(c *gin.Context) {
	calendarConfig := InitGoogleCalendarConfig()
	if calendarConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Google Calendar not configured"})
		return
	}

	// Generate state parameter for security
	state := fmt.Sprintf("user_%s_%d", c.Query("user_id"), time.Now().Unix())

	url := calendarConfig.Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.JSON(http.StatusOK, gin.H{
		"auth_url": url,
		"state":    state,
	})
}

// GoogleOAuth2CallbackHandler handles OAuth2 callback
func (a *App) GoogleOAuth2CallbackHandler(c *gin.Context) {
	calendarConfig := InitGoogleCalendarConfig()
	if calendarConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Google Calendar not configured"})
		return
	}

	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "authorization code required"})
		return
	}

	// Exchange code for token
	token, err := calendarConfig.Config.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to exchange code for token"})
		return
	}

	// Store token (in a real app, you'd store this in database associated with user)
	tokenJSON, _ := json.Marshal(token)

	c.JSON(http.StatusOK, gin.H{
		"message": "Authorization successful",
		"state":   state,
		"token":   string(tokenJSON), // In production, don't return token directly
	})
}

// GetGoogleCalendarEvents fetches events from Google Calendar
func (a *App) GetGoogleCalendarEvents(c *gin.Context) {
	// Get token from request (in production, get from database)
	tokenStr := c.GetHeader("X-Google-Token")
	if tokenStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google token required in X-Google-Token header"})
		return
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(tokenStr), &token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token format"})
		return
	}

	calendarConfig := InitGoogleCalendarConfig()
	if calendarConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Google Calendar not configured"})
		return
	}

	// Create HTTP client with token
	client := calendarConfig.Config.Client(context.Background(), &token)

	// Create Calendar service
	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create calendar service"})
		return
	}

	// Parse query parameters
	calendarID := c.DefaultQuery("calendar_id", "primary")
	timeMin := c.Query("time_min") // RFC3339 format
	timeMax := c.Query("time_max") // RFC3339 format
	userID := c.Query("user_id")   // target user to create availability/booking for
	maxResults := int64(250)

	// Build the events call
	eventsCall := srv.Events.List(calendarID).
		SingleEvents(true).
		OrderBy("startTime").
		MaxResults(maxResults)

	if timeMin != "" {
		eventsCall = eventsCall.TimeMin(timeMin)
	}
	if timeMax != "" {
		eventsCall = eventsCall.TimeMax(timeMax)
	}

	// Execute the call
	events, err := eventsCall.Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to retrieve events: %v", err)})
		return
	}

	// Wire services for creating availability and bookings if user_id is provided
	var (
		availSvc   *service.AvailabilityService
		bookingSvc *service.BookingService
	)
	if userID != "" && a.DB != nil {
		availRepo := postgres.NewAvailabilityRepo()
		bookingRepo := postgres.NewBookingRepo()
		availSvc = service.NewAvailabilityService(a.DB, availRepo, bookingRepo)
		bookingSvc = service.NewBookingService(a.DB, bookingRepo, availSvc)
	}

	// Convert to our format
	var calendarEvents []CalendarEvent
	fmt.Printf("Processing %d events for user_id: %s\n", len(events.Items), userID)
	for _, item := range events.Items {

		str, _ := json.MarshalIndent(item, "", "")
		fmt.Println("-----------------------------------------------------------------------")
		fmt.Println("item: ", string(str))
		event := CalendarEvent{
			ID:          item.Id,
			Summary:     item.Summary,
			Description: item.Description,
			Location:    item.Location,
			Status:      item.Status,
		}

		// Handle creator
		if item.Creator != nil {
			event.Creator = item.Creator.Email
		}

		// Extract meeting link (Google Meet link)
		if item.HangoutLink != "" {
			event.MeetingLink = item.HangoutLink
		}

		// Extract detailed conference data
		if item.ConferenceData != nil && len(item.ConferenceData.EntryPoints) > 0 {
			conferenceInfo := &ConferenceInfo{}

			// Get conference type
			if item.ConferenceData.ConferenceSolution != nil {
				conferenceInfo.Type = item.ConferenceData.ConferenceSolution.Name
			}

			// Get meeting ID
			if item.ConferenceData.ConferenceId != "" {
				conferenceInfo.ID = item.ConferenceData.ConferenceId
			}

			// Extract entry points (URLs and phone numbers)
			var phoneNumbers []string
			for _, entryPoint := range item.ConferenceData.EntryPoints {
				switch entryPoint.EntryPointType {
				case "video":
					if conferenceInfo.URL == "" && entryPoint.Uri != "" {
						conferenceInfo.URL = entryPoint.Uri
						// If no HangoutLink, use this as meeting link
						if event.MeetingLink == "" {
							event.MeetingLink = entryPoint.Uri
						}
					}
				case "phone":
					if entryPoint.Uri != "" {
						phoneNumbers = append(phoneNumbers, entryPoint.Uri)
					}
				case "more":
					// Additional meeting details
					if entryPoint.Uri != "" && conferenceInfo.URL == "" {
						conferenceInfo.URL = entryPoint.Uri
						if event.MeetingLink == "" {
							event.MeetingLink = entryPoint.Uri
						}
					}
				}
			}

			if len(phoneNumbers) > 0 {
				conferenceInfo.PhoneNumbers = phoneNumbers
			}

			// Only include conference data if we have meaningful info
			if conferenceInfo.URL != "" || conferenceInfo.ID != "" || len(conferenceInfo.PhoneNumbers) > 0 {
				event.ConferenceData = conferenceInfo
			}
		}

		// Parse start time
		if item.Start.DateTime != "" {
			if startTime, err := time.Parse(time.RFC3339, item.Start.DateTime); err == nil {
				event.StartTime = startTime
			}
		} else if item.Start.Date != "" {
			if startTime, err := time.Parse("2006-01-02", item.Start.Date); err == nil {
				event.StartTime = startTime
			}
		}

		// Parse end time
		if item.End.DateTime != "" {
			if endTime, err := time.Parse(time.RFC3339, item.End.DateTime); err == nil {
				event.EndTime = endTime
			}
		} else if item.End.Date != "" {
			if endTime, err := time.Parse("2006-01-02", item.End.Date); err == nil {
				event.EndTime = endTime
			}
		}

		calendarEvents = append(calendarEvents, event)

		// Debug logging
		fmt.Printf("Event: %s, MeetingLink: %s, ConferenceData: %+v\n", event.Summary, event.MeetingLink, event.ConferenceData)
		fmt.Printf("StartTime: %v, EndTime: %v\n", event.StartTime, event.EndTime)
		fmt.Printf("isGoogleMeetEvent: %v\n", isGoogleMeetEvent(&event))

		// If user_id provided and this is a Google Meet event, create availability and booking
		if userID != "" && isGoogleMeetEvent(&event) && !event.StartTime.IsZero() && !event.EndTime.IsZero() && bookingSvc != nil && availSvc != nil {
			fmt.Printf("Creating availability and booking for Google Meet event: %s\n", event.Summary)
			startUTC := event.StartTime.UTC()
			endUTC := event.EndTime.UTC()
			if endUTC.After(startUTC) {
				// Create a matching availability rule for the specific weekday/time window
				durMins := int(endUTC.Sub(startUTC).Minutes())
				if durMins <= 0 {
					// skip invalid duration
					fmt.Printf("Skipping invalid duration: %d minutes\n", durMins)
					continue
				}
				rule := models.AvailabilityRule{
					DayOfWeek:      int(startUTC.Weekday()),
					StartTime:      startUTC.Format("15:04"),
					EndTime:        endUTC.Format("15:04"),
					SlotLengthMins: durMins,
					Title:          event.Summary,
					Available:      true,
				}
				fmt.Printf("Creating availability rule: %+v\n", rule)
				availResult, availErr := availSvc.SetAvailability(c.Request.Context(), userID, []models.AvailabilityRule{rule})
				if availErr != nil {
					fmt.Printf("Error creating availability: %v\n", availErr)
				} else {
					fmt.Printf("Availability created successfully: %+v\n", availResult)
				}

				// Create booking for this time window
				bookingParams := service.CreateBookingParams{
					CandidateEmail: event.Creator,
					Start:          startUTC,
					End:            endUTC,
					Source:         "google_calendar",
					Type:           "google_meet",
					Description:    event.MeetingLink,
					Title:          event.Summary,
				}
				fmt.Printf("Creating booking: %+v\n", bookingParams)
				bookingResult, bookingErr := bookingSvc.CreateBooking(c.Request.Context(), userID, bookingParams)
				if bookingErr != nil {
					fmt.Printf("Error creating booking: %v\n", bookingErr)
				} else {
					fmt.Printf("Booking created successfully: %+v\n", bookingResult)
				}
			}
		} else {
			fmt.Printf("Skipping event - userID: %s, isGoogleMeet: %v, hasTimes: %v, hasServices: %v\n",
				userID, isGoogleMeetEvent(&event), !event.StartTime.IsZero() && !event.EndTime.IsZero(), bookingSvc != nil && availSvc != nil)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"events": calendarEvents,
		"count":  len(calendarEvents),
	})
}

// isGoogleMeetEvent determines whether an event is a Google Meet
func isGoogleMeetEvent(e *CalendarEvent) bool {
	if e == nil {
		return false
	}
	if e.MeetingLink != "" && strings.Contains(strings.ToLower(e.MeetingLink), "meet.google.com") {
		return true
	}
	if e.ConferenceData != nil {
		name := strings.ToLower(e.ConferenceData.Type)
		if strings.Contains(name, "hangouts") || strings.Contains(name, "google") || strings.Contains(name, "meet") {
			return true
		}
	}
	return false
}

// GetGoogleCalendarList fetches available calendars
func (a *App) GetGoogleCalendarList(c *gin.Context) {
	// Get token from request
	tokenStr := c.GetHeader("X-Google-Token")
	if tokenStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google token required in X-Google-Token header"})
		return
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(tokenStr), &token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token format"})
		return
	}

	calendarConfig := InitGoogleCalendarConfig()
	if calendarConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Google Calendar not configured"})
		return
	}

	// Create HTTP client with token
	client := calendarConfig.Config.Client(context.Background(), &token)

	// Create Calendar service
	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create calendar service"})
		return
	}

	// Get calendar list
	calendarList, err := srv.CalendarList.List().Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to retrieve calendars: %v", err)})
		return
	}

	type CalendarInfo struct {
		ID          string `json:"id"`
		Summary     string `json:"summary"`
		Description string `json:"description,omitempty"`
		Primary     bool   `json:"primary"`
		AccessRole  string `json:"access_role"`
	}

	var calendars []CalendarInfo
	for _, item := range calendarList.Items {
		calendar := CalendarInfo{
			ID:          item.Id,
			Summary:     item.Summary,
			Description: item.Description,
			Primary:     item.Primary,
			AccessRole:  item.AccessRole,
		}
		calendars = append(calendars, calendar)
	}

	c.JSON(http.StatusOK, gin.H{
		"calendars": calendars,
		"count":     len(calendars),
	})
}

// CreateInterviewEvent creates a Google Meet event in Google Calendar
func (a *App) CreateInterviewEvent(c *gin.Context) {
	// Get token from request
	tokenStr := c.GetHeader("X-Google-Token")
	if tokenStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google token required in X-Google-Token header"})
		return
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(tokenStr), &token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token format"})
		return
	}

	// Parse interview event from request body
	var interviewEvent InterviewEvent
	if err := c.ShouldBindJSON(&interviewEvent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if interviewEvent.CandidateName == "" || interviewEvent.CandidateEmail == "" ||
		interviewEvent.Position == "" || interviewEvent.Stage == "" ||
		interviewEvent.Mode == "" || interviewEvent.InterviewerEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required fields"})
		return
	}

	// Set default values
	if interviewEvent.Status == "" {
		interviewEvent.Status = "scheduled"
	}
	if interviewEvent.Duration == 0 {
		interviewEvent.Duration = 60 // Default 1 hour
	}

	calendarConfig := InitGoogleCalendarConfig()
	if calendarConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Google Calendar not configured"})
		return
	}

	// Create HTTP client with token
	client := calendarConfig.Config.Client(context.Background(), &token)

	// Create Calendar service
	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create calendar service"})
		return
	}

	// Prepare event details
	startTime := interviewEvent.DateTime
	endTime := startTime.Add(time.Duration(interviewEvent.Duration) * time.Minute)

	// Create event title
	eventTitle := fmt.Sprintf("%s Interview - %s (%s)", interviewEvent.Position, interviewEvent.CandidateName, interviewEvent.Stage)

	// Create event description
	description := fmt.Sprintf(`Interview Details:
Candidate: %s (%s)
Position: %s
Stage: %s
Interviewer: %s
Mode: %s
Status: %s`,
		interviewEvent.CandidateName, interviewEvent.CandidateEmail,
		interviewEvent.Position, interviewEvent.Stage,
		interviewEvent.InterviewerEmail, interviewEvent.Mode, interviewEvent.Status)

	if interviewEvent.Description != "" {
		description += "\n\nAdditional Notes:\n" + interviewEvent.Description
	}

	// Create Google Calendar event
	event := &calendar.Event{
		Summary:     eventTitle,
		Description: description,
		Start: &calendar.EventDateTime{
			DateTime: startTime.Format(time.RFC3339),
			TimeZone: "UTC",
		},
		End: &calendar.EventDateTime{
			DateTime: endTime.Format(time.RFC3339),
			TimeZone: "UTC",
		},
		Attendees: []*calendar.EventAttendee{
			{Email: interviewEvent.CandidateEmail, DisplayName: interviewEvent.CandidateName},
			{Email: interviewEvent.InterviewerEmail},
		},
		Reminders: &calendar.EventReminders{  // ← Move this INSIDE the struct
			UseDefault: true,
		},
	}

	// Add Google Meet conference if mode is "google"
	if interviewEvent.Mode == "google" {
		event.ConferenceData = &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				RequestId: fmt.Sprintf("interview-%s-%d", interviewEvent.CandidateEmail, time.Now().Unix()),
				ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
					Type: "hangoutsMeet",
				},
			},
		}
	}

	// Add location if provided
	if interviewEvent.Location != "" {
		event.Location = interviewEvent.Location
	} else if interviewEvent.Mode == "google" {
		event.Location = "Google Meet"
	}

	// Create the event
	calendarID := c.DefaultQuery("calendar_id", "primary")
	createdEvent, err := srv.Events.Insert(calendarID, event).ConferenceDataVersion(1).Do()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create event: %v", err)})
		return
	}

	// Extract meeting link if available
	meetingLink := ""
	if createdEvent.HangoutLink != "" {
		meetingLink = createdEvent.HangoutLink
	} else if createdEvent.ConferenceData != nil && len(createdEvent.ConferenceData.EntryPoints) > 0 {
		for _, entryPoint := range createdEvent.ConferenceData.EntryPoints {
			if entryPoint.EntryPointType == "video" && entryPoint.Uri != "" {
				meetingLink = entryPoint.Uri
				break
			}
		}
	}

	// Return success response
	response := gin.H{
		"message":      "Interview event created successfully",
		"event_id":     createdEvent.Id,
		"event_title":  createdEvent.Summary,
		"start_time":   createdEvent.Start.DateTime,
		"end_time":     createdEvent.End.DateTime,
		"meeting_link": meetingLink,
		"attendees":    []string{interviewEvent.CandidateEmail, interviewEvent.InterviewerEmail},
	}

	c.JSON(http.StatusCreated, response)
}

// RefreshGoogleToken refreshes an expired Google OAuth token
func (a *App) RefreshGoogleToken(c *gin.Context) {
	// Get refresh token from request body
	var requestBody struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token required"})
		return
	}

	calendarConfig := InitGoogleCalendarConfig()
	if calendarConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Google Calendar not configured"})
		return
	}

	// Create token with refresh token
	token := &oauth2.Token{
		RefreshToken: requestBody.RefreshToken,
	}

	// Use token source to get new token
	tokenSource := calendarConfig.Config.TokenSource(context.Background(), token)
	newToken, err := tokenSource.Token()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to refresh token"})
		return
	}

	// Return new token
	tokenJSON, _ := json.Marshal(newToken)
	c.JSON(http.StatusOK, gin.H{
		"message": "Token refreshed successfully",
		"token":   string(tokenJSON),
	})
}
