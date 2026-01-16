package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"scheduler-service/internal/config"
)

func Run(router *gin.Engine, cfg *config.Config) {
	if cfg.Host == "" {
		panic("HOST environment variable is required")
	}
	if cfg.Port == "" {
		panic("PORT environment variable is required")
	}

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	if err := router.Run(addr); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
