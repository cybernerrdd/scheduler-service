package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"

	"scheduler-service/internal/config"
)

func main() {
	cfg, _ := config.Load()
	dbURL := cfg.DatabaseURL
	if dbURL == "" {
		log.Fatal("DATABASE_URL required")
	}

	sqlDB, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer sqlDB.Close()

	migrationsPath := filepath.Join("internal", "migrations")
	if _, err := os.Stat(migrationsPath); os.IsNotExist(err) {
		wd, _ := os.Getwd()
		migrationsPath = filepath.Join(wd, "internal", "migrations")
	}

	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		log.Fatalf("Failed to get absolute path: %v", err)
	}

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		log.Fatalf("failed to create postgres driver: %v", err)
	}

	sourceURL := "file://" + absPath
	sourceDriver, err := (&file.File{}).Open(sourceURL)
	if err != nil {
		log.Fatalf("failed to open source driver: %v", err)
	}

	m, err := migrate.NewWithInstance("file", sourceDriver, "postgres", driver)
	if err != nil {
		log.Fatalf("failed to create migrate instance: %v", err)
	}

	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("failed to run migrations: %v", err)
		}
		log.Println("migrations applied successfully")
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("failed to rollback migrations: %v", err)
		}
		log.Println("migrations rolled back successfully")
	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("failed to get version: %v", err)
		}
		log.Printf("current migration version: %d (dirty: %v)", version, dirty)
	default:
		log.Fatalf("unknown command: %s. Use 'up', 'down', or 'version'", command)
	}
}
