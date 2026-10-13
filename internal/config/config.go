package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
}

func Load() (Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is not set")
	}

	return Config{
		DatabaseURL: dbURL,
	}, nil
}