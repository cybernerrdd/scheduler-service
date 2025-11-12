package app

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"scheduler-service/internal/repository/postgres"
	"scheduler-service/internal/service"
)

// Auth middleware supporting API keys, static tokens, or JWT
func AuthMiddlewareFromEnv() gin.HandlerFunc {
	staticTokens := strings.Split(strings.TrimSpace(os.Getenv("STATIC_TOKENS")), ",")
	jwtSecret := strings.TrimSpace(os.Getenv("JWT_HMAC_SECRET"))

	return func(c *gin.Context) {
		// Try to get API key from header (X-API-Key) or Authorization header
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			// Try Authorization header with Bearer
			auth := c.GetHeader("Authorization")
			if auth != "" {
				parts := strings.Fields(auth)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					apiKey = parts[1]
				}
			}
		}

		// If API key is provided, validate it
		if apiKey != "" {
			// Get DB from context or app instance
			// We'll need to pass DB pool through context or use a different approach
			// For now, let's get it from the app instance stored in context
			if db, ok := c.Get("db_pool"); ok {
				if pool, ok := db.(*pgxpool.Pool); ok {
					apiKeyRepo := postgres.NewAPIKeyRepo()
					apiKeyService := service.NewAPIKeyService(pool, apiKeyRepo)
					
					apiKeyRecord, err := apiKeyService.ValidateAPIKey(c.Request.Context(), apiKey)
					if err == nil && apiKeyRecord != nil {
						// Store email in context for later use
						c.Set("user_email", apiKeyRecord.Email)
						c.Next()
						return
					}
				}
			}
		}

		// Fallback to existing JWT/static token logic
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization. Provide API key in X-API-Key header or Authorization Bearer token"})
			return
		}
		parts := strings.Fields(auth)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}
		tokenStr := parts[1]

		// JWT path
		if jwtSecret != "" {
			_, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrTokenMalformed
				}
				return []byte(jwtSecret), nil
			}, jwt.WithLeeway(5*time.Second))
			if err == nil {
				c.Next()
				return
			}
		}

		// static tokens
		for _, t := range staticTokens {
			if tokenStr == strings.TrimSpace(t) {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
	}
}

// AuthMiddlewareWithDB creates auth middleware with DB access
// API keys are now REQUIRED - no fallback to static tokens or JWT
func AuthMiddlewareWithDB(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get API key from header (X-API-Key) or Authorization header
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			// Try Authorization header with Bearer
			auth := c.GetHeader("Authorization")
			if auth != "" {
				parts := strings.Fields(auth)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					apiKey = parts[1]
				}
			}
		}

		// API key is REQUIRED
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "API key is not provided",
			})
			return
		}

		// Validate the API key
		apiKeyRepo := postgres.NewAPIKeyRepo()
		apiKeyService := service.NewAPIKeyService(db, apiKeyRepo)
		
		apiKeyRecord, err := apiKeyService.ValidateAPIKey(c.Request.Context(), apiKey)
		if err != nil || apiKeyRecord == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid API key",
			})
			return
		}

		// Store email in context for later use
		c.Set("user_email", apiKeyRecord.Email)
		c.Next()
	}
}

// hashAPIKey creates a SHA256 hash of the API key
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}
