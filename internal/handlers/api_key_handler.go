package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"scheduler-service/internal/service"
)

type APIKeyHandler struct {
	Service *service.APIKeyService
}

// GenerateAPIKey handles POST /api/auth/key
// Request body: { "email": "user@example.com", "password": "password123" }
// Response: { "api_key": "sk_...", "email": "user@example.com", "created_at_utc": "..." }
func (h *APIKeyHandler) GenerateAPIKey(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	apiKey, apiKeyRecord, err := h.Service.GenerateAPIKey(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_key":        apiKey,
		"email":          apiKeyRecord.Email,
		"created_at_utc": apiKeyRecord.CreatedAt.UTC(),
		"uuid":           apiKeyRecord.ID,
	})
}
