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

	log.Println("Starting scheduler service...")

	cfg, _ := config.Load()
	dbURL := cfg.DatabaseURL
	if dbURL == "" {
		log.Fatal("DATABASE_URL required")
	}
	if cfg.Port == "" {
		log.Fatal("PORT required")
	}

	log.Printf("Connecting to database...")
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()
	log.Println("Database connected successfully")

	appInstance := &app.App{DB: pool}

	log.Println("Building router...")
	r := router.Build(appInstance, cfg)

	host := cfg.Host
	if host == "" {
		host = "0.0.0.0"
	}
	log.Printf("Starting server on %s:%s...", host, cfg.Port)
	server.Run(r, cfg)
}
