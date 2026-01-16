package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	Host           string
	Port           string
	StaticTokens   string
	GoogleClientID string
	GoogleSecret   string
	GoogleRedirect string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		Host:           os.Getenv("HOST"),
		Port:           os.Getenv("PORT"),
		StaticTokens:   os.Getenv("STATIC_TOKENS"),
		GoogleClientID: os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleSecret:   os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirect: os.Getenv("GOOGLE_REDIRECT_URL"),
	}
	return cfg, nil
}
