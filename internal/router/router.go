package router

import (
	"github.com/gin-gonic/gin"

	"scheduler-service/internal/app"
	"scheduler-service/internal/config"
	"scheduler-service/internal/handlers"
	"scheduler-service/internal/repository/postgres"
	"scheduler-service/internal/service"
)

func Build(appInstance *app.App, cfg *config.Config) *gin.Engine {
	r := gin.Default()

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
			users.GET("/:id/availability", availHandlers.ListAvailability)
			users.GET("/:id/slots", availHandlers.GetSlots)
			users.POST("/:id/bookings", availHandlers.CreateBooking)
			users.GET("/:id/bookings", availHandlers.ListBookings)
		}

		api.DELETE("/bookings/:id", availHandlers.CancelBooking)
	}

	return r
}
