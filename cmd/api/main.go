package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bidmybl/api-health-monitor/internal/config"
	"github.com/bidmybl/api-health-monitor/internal/features/checks/checker"
	checkRepo "github.com/bidmybl/api-health-monitor/internal/features/checks/repository"
	checkService "github.com/bidmybl/api-health-monitor/internal/features/checks/service"
	checksTransport "github.com/bidmybl/api-health-monitor/internal/features/checks/transport"
	endpointRepo "github.com/bidmybl/api-health-monitor/internal/features/endpoints/repository"
	endpointTransport "github.com/bidmybl/api-health-monitor/internal/features/endpoints/transport"
	"github.com/bidmybl/api-health-monitor/internal/server"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create database pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	logger.Info("connected to PostgreSQL")

	endpointRepository := endpointRepo.NewRepository(pool)
	checkRepository := checkRepo.NewRepository(pool)

	endpointHandler := endpointTransport.NewHandler(
		endpointRepository,
		logger,
	)

	httpChecker := checker.NewChecker(10 * time.Second)

	checksService := checkService.NewService(
		endpointRepository,
		httpChecker,
		checkRepository,
	)

	checksHandler := checksTransport.NewHandler(
		checksService,
		logger,
	)

	router := server.NewRouter(
		endpointHandler,
		checksHandler,
	)

	httpServer := &http.Server{
		Addr:              ":8090",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- httpServer.ListenAndServe()
	}()

	logger.Info("HTTP server started", "address", httpServer.Addr)

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")

	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server failed: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	logger.Info("HTTP server stopped")
	return nil
}
