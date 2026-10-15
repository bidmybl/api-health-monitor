package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL string
}

func Load() (Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, errors.New("DATABASE_URL is not set")
	}

	return Config{
		DatabaseURL: dbURL,
	}, nil
}
