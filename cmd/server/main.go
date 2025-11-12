package main

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"scheduler-service/internal/app"
	"scheduler-service/internal/config"
	"scheduler-service/internal/router"
	"scheduler-service/internal/server"
)

func main() {
    ctx := context.Background()

    cfg, _ := config.Load()
    dbURL := cfg.DatabaseURL
    if dbURL == "" {
        log.Fatal("DATABASE_URL required")
    }

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

    appInstance := &app.App{DB: pool}

    r := router.Build(appInstance, cfg)
    server.Run(r)
}
