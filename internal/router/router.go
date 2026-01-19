package router

import (
	"os"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"

	"scheduler-service/internal/app"
	"scheduler-service/internal/config"
	"scheduler-service/internal/handlers"
	"scheduler-service/internal/repository/postgres"
	"scheduler-service/internal/service"
)

func Build(appInstance *app.App, cfg *config.Config) *gin.Engine {
	// Set release mode for production
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// Trust Railway proxy (required for correct client IP and X-Forwarded-* headers)
	r.SetTrustedProxies([]string{"0.0.0.0/0"})

	r.GET("/health", func(c *gin.Context) {
		if appInstance.DB != nil {
			ctx := c.Request.Context()
			if err := appInstance.DB.Ping(ctx); err != nil {
				c.JSON(503, gin.H{
					"status":   "unhealthy",
					"database": "disconnected",
					"error":    err.Error(),
				})
				return
			}
			c.JSON(200, gin.H{
				"status":   "healthy",
				"database": "connected",
			})
		} else {
			c.JSON(503, gin.H{
				"status":   "unhealthy",
				"database": "not initialized",
			})
		}
	})

	// Initialize Google OAuth config if credentials are available
	var googleTokenSvc *service.GoogleTokenService
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")

	if clientID != "" && clientSecret != "" && redirectURL != "" {
		oauthConfig := &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				calendar.CalendarScope,
			},
			Endpoint: google.Endpoint,
		}
		googleTokenRepo := postgres.NewGoogleTokenRepo()
		googleTokenSvc = service.NewGoogleTokenService(appInstance.DB, googleTokenRepo, oauthConfig)
		appInstance.GoogleTokenSvc = googleTokenSvc
	}

	// OAuth2 callback (must be before auth middleware)
	r.GET("/oauth2callback", appInstance.GoogleOAuth2CallbackHandler)

	api := r.Group("/api")
	{
		// Public endpoint for generating API keys (no auth required)
		apiKeyRepo := postgres.NewAPIKeyRepo()
		apiKeyService := service.NewAPIKeyService(appInstance.DB, apiKeyRepo)
		apiKeyHandler := &handlers.APIKeyHandler{Service: apiKeyService}
		api.POST("/auth/key", apiKeyHandler.GenerateAPIKey)

		// Google Calendar integration routes - no API key required
		calendar := api.Group("/calendar")
		{
			calendar.GET("/auth", appInstance.GoogleAuthHandler)
			calendar.GET("/events", appInstance.GetGoogleCalendarEvents)
			calendar.GET("/calendars", appInstance.GetGoogleCalendarList)
			calendar.POST("/refresh-token", appInstance.RefreshGoogleToken)
			calendar.POST("/interview", appInstance.CreateInterviewEvent)
		}

		// All other endpoints require API key authentication
		api.Use(app.AuthMiddlewareWithDB(appInstance.DB))

		availRepo := postgres.NewAvailabilityRepo()
		bookingRepo := postgres.NewBookingRepo()
		availService := service.NewAvailabilityService(appInstance.DB, availRepo, bookingRepo)
		bookingService := service.NewBookingService(appInstance.DB, bookingRepo, availService)

		availHandlers := &handlers.AvailabilityHandlers{DB: appInstance.DB, AvailSv: availService, BookSv: bookingService}

		users := api.Group("/users")
		{
			users.POST("/:id/availability", availHandlers.SetAvailability)
			users.PUT("/:id/availability/:rule_id", availHandlers.UpdateAvailability)
			users.DELETE("/:id/availability/:rule_id", availHandlers.DeleteAvailability)
			users.GET("/:id/availability", availHandlers.ListAvailability)
			users.GET("/:id/slots/event/:event", availHandlers.GetSlotsByEvent) // Must be before /:id/slots to avoid path conflict
			users.GET("/:id/slots", availHandlers.GetSlots)
			users.POST("/:id/bookings", availHandlers.CreateBooking)
			users.GET("/:id/bookings", availHandlers.ListBookings)
		}

		api.DELETE("/bookings/:id", availHandlers.CancelBooking)
	}

	return r
}
