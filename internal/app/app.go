package app

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"scheduler-service/internal/service"
)

type App struct {
	DB              *pgxpool.Pool
	GoogleTokenSvc  *service.GoogleTokenService
}
