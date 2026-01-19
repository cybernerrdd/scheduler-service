package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"scheduler-service/internal/config"
)

func Run(router *gin.Engine, cfg *config.Config) {
	if cfg.Port == "" {
		log.Fatal("PORT environment variable is required")
	}

	// Default HOST to 0.0.0.0 if not set (works for Railway and most deployments)
	host := cfg.Host
	if host == "" {
		host = "0.0.0.0"
	}

	addr := fmt.Sprintf("%s:%s", host, cfg.Port)
	log.Printf("Server listening on %s", addr)
	log.Printf("Health check available at http://%s/health", addr)

	if err := router.Run(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}
