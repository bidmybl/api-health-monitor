package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	checkRepo "github.com/bidmybl/api-health-monitor/internal/features/checks/repository"
	endpointRepo "github.com/bidmybl/api-health-monitor/internal/features/endpoints/repository"
)

const (
	defaultInterval       = 5 * time.Second
	defaultMaxConcurrency = 5
)

type EndpointRepository interface {
	GetEndpointsDueForCheck(ctx context.Context) ([]endpointRepo.Endpoint, error)
	TryAcquireCheckLock(ctx context.Context, endpointID int64) (func(context.Context) error, bool, error)
}

type ChecksService interface {
	CheckEndpoint(ctx context.Context, endpointID int64) (checkRepo.CheckResult, error)
}

type Scheduler struct {
	endpointRepo   EndpointRepository
	checksService  ChecksService
	logger         *slog.Logger
	interval       time.Duration
	maxConcurrency int
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
		endpointRepo:   endpointRepo,
		checksService:  checksService,
		logger:         logger,
		interval:       interval,
		maxConcurrency: defaultMaxConcurrency,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info(
		"scheduler started",
		"interval", s.interval,
		"max_concurrency", s.maxConcurrency,
	)

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
	if ctx.Err() != nil {
		return
	}

	endpoints, err := s.endpointRepo.GetEndpointsDueForCheck(ctx)
	if err != nil {
		if ctx.Err() == nil {
			s.logger.Error("get endpoints due for check", "error", err)
		}
		return
	}

	if len(endpoints) == 0 {
		return
	}

	workerCount := min(len(endpoints), s.maxConcurrency)
	jobs := make(chan endpointRepo.Endpoint)

	var workers sync.WaitGroup
	workers.Add(workerCount)

	for range workerCount {
		go func() {
			defer workers.Done()

			for endpoint := range jobs {
				if ctx.Err() != nil {
					return
				}

				release, acquired, err := s.endpointRepo.TryAcquireCheckLock(
					ctx,
					endpoint.ID,
				)
				if err != nil {
					if ctx.Err() != nil {
						return
					}

					s.logger.Error(
						"acquire endpoint check lock",
						"endpoint_id", endpoint.ID,
						"error", err,
					)
					continue
				}

				if !acquired {
					s.logger.Debug(
						"scheduled endpoint check skipped; already running",
						"endpoint_id", endpoint.ID,
					)
					continue
				}

				s.checkEndpoint(ctx, endpoint.ID, release)
			}
		}()
	}

sendJobs:
	for _, endpoint := range endpoints {
		select {
		case <-ctx.Done():
			break sendJobs
		case jobs <- endpoint:
		}
	}

	close(jobs)
	workers.Wait()
}

func (s *Scheduler) checkEndpoint(
	ctx context.Context,
	endpointID int64,
	release func(context.Context) error,
) {
	defer func() {
		releaseCtx, cancel := context.WithTimeout(
			context.Background(),
			2*time.Second,
		)
		defer cancel()

		if err := release(releaseCtx); err != nil {
			s.logger.Error(
				"release endpoint check lock",
				"endpoint_id", endpointID,
				"error", err,
			)
		}
	}()

	_, err := s.checksService.CheckEndpoint(ctx, endpointID)
	if err != nil {
		if ctx.Err() != nil {
			return
		}

		s.logger.Error(
			"scheduled endpoint check failed",
			"endpoint_id", endpointID,
			"error", err,
		)
		return
	}

	s.logger.Info(
		"scheduled endpoint check completed",
		"endpoint_id", endpointID,
	)
}
