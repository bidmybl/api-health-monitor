package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/bidmybl/api-health-monitor/internal/config"

	"github.com/jackc/pgx/v5"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	if err := conn.Ping(ctx); err != nil {
		slog.Error("failed to ping db", "error", err)
	}

	logger.Info("connected to PostgreSQL")
}
