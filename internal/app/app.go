package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"scheduler-service/internal/repository"
	"scheduler-service/internal/service"
)

type App struct {
	DB              *pgxpool.Pool
	GoogleTokenSvc  *service.GoogleTokenService
}
