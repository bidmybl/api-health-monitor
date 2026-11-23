package scheduler

import (
	"context"
	"log/slog"
	"time"

	checkRepo "github.com/bidmybl/api-health-monitor/internal/features/checks/repository"
	endpointRepo "github.com/bidmybl/api-health-monitor/internal/features/endpoints/repository"
)

const defaultInterval = 5 * time.Second

type EndpointRepository interface {
	GetEndpointsDueForCheck(ctx context.Context) ([]endpointRepo.Endpoint, error)
}

type ChecksService interface {
	CheckEndpoint(
		ctx context.Context,
		endpointID int64,
	) (checkRepo.CheckResult, error)
}

type Scheduler struct {
	endpointRepo  EndpointRepository
	checksService ChecksService
	logger        *slog.Logger
	interval      time.Duration
}

func New(
	endpointRepo EndpointRepository,
	checksService ChecksService,
	logger *slog.Logger,
	interval time.Duration,
) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}

	if interval <= 0 {
		interval = defaultInterval
	}

	return &Scheduler{
		endpointRepo:  endpointRepo,
		checksService: checksService,
		logger:        logger,
		interval:      interval,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info(
		"scheduler started",
		"interval",
		s.interval,
	)

	// Выполняем первую проверку сразу после запуска.
	s.runDueChecks(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return

		case <-ticker.C:
			s.runDueChecks(ctx)
		}
	}
}

func (s *Scheduler) runDueChecks(ctx context.Context) {
	endpoints, err := s.endpointRepo.GetEndpointsDueForCheck(ctx)
	if err != nil {
		if ctx.Err() == nil {
			s.logger.Error("get endpoints due for check", "error", err)
		}
		return
	}

	for _, endpoint := range endpoints {
		if ctx.Err() != nil {
			return
		}

		_, err := s.checksService.CheckEndpoint(ctx, endpoint.ID)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			s.logger.Error(
				"scheduled endpoint check failed",
				"endpoint_id",
				endpoint.ID,
				"error",
				err,
			)
			continue
		}

		s.logger.Info(
			"scheduled endpoint check completed",
			"endpoint_id",
			endpoint.ID,
		)
	}
}
