package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	checkRepo "github.com/bidmybl/api-health-monitor/internal/features/checks/repository"
	endpointRepo "github.com/bidmybl/api-health-monitor/internal/features/endpoints/repository"
)

type fakeEndpointRepository struct {
	endpoints []endpointRepo.Endpoint
	err       error
}

func (f *fakeEndpointRepository) GetEndpointsDueForCheck(
	ctx context.Context,
) ([]endpointRepo.Endpoint, error) {
	return f.endpoints, f.err
}

type fakeChecksService struct {
	mu         sync.Mutex
	checkedIDs []int64
	errors     map[int64]error
}

func (f *fakeChecksService) CheckEndpoint(
	ctx context.Context,
	endpointID int64,
) (checkRepo.CheckResult, error) {
	f.mu.Lock()
	f.checkedIDs = append(f.checkedIDs, endpointID)
	f.mu.Unlock()

	if err := f.errors[endpointID]; err != nil {
		return checkRepo.CheckResult{}, err
	}

	return checkRepo.CheckResult{}, nil
}

func (f *fakeChecksService) getCheckedIDs() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	ids := make([]int64, len(f.checkedIDs))
	copy(ids, f.checkedIDs)

	return ids
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSchedulerRunDueChecks(t *testing.T) {
	endpoints := []endpointRepo.Endpoint{
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}

	repository := &fakeEndpointRepository{endpoints: endpoints}
	service := &fakeChecksService{errors: make(map[int64]error)}

	scheduler := New(repository, service, newTestLogger(), 0)
	scheduler.runDueChecks(context.Background())

	checkedIDs := service.getCheckedIDs()

	if len(checkedIDs) != len(endpoints) {
		t.Fatalf("checked %d endpoints, want %d", len(checkedIDs), len(endpoints))
	}

	checked := make(map[int64]bool)
	for _, id := range checkedIDs {
		checked[id] = true
	}

	for _, endpoint := range endpoints {
		if !checked[endpoint.ID] {
			t.Errorf("endpoint %d was not checked", endpoint.ID)
		}
	}
}

func TestSchedulerContinuesAfterCheckError(t *testing.T) {
	endpoints := []endpointRepo.Endpoint{
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}

	repository := &fakeEndpointRepository{endpoints: endpoints}
	service := &fakeChecksService{
		errors: map[int64]error{
			2: errors.New("check failed"),
		},
	}

	scheduler := New(repository, service, newTestLogger(), 0)
	scheduler.runDueChecks(context.Background())

	checkedIDs := service.getCheckedIDs()

	if len(checkedIDs) != 3 {
		t.Fatalf("checked %d endpoints, want 3", len(checkedIDs))
	}
}

func TestSchedulerLimitsConcurrency(t *testing.T) {
	const (
		endpointCount = 12
		maxConcurrent = 3
	)

	var (
		mu          sync.Mutex
		active      int
		maxObserved int
	)

	service := &fakeChecksService{
		errors: make(map[int64]error),
	}

	// Wrap the service to track concurrent executions.
	trackingService := &trackingChecksService{
		base: service,
		check: func(ctx context.Context, endpointID int64) {
			mu.Lock()
			active++
			if active > maxObserved {
				maxObserved = active
			}
			mu.Unlock()

			time.Sleep(20 * time.Millisecond)

			mu.Lock()
			active--
			mu.Unlock()
		},
	}

	endpoints := make([]endpointRepo.Endpoint, endpointCount)
	for i := range endpoints {
		endpoints[i].ID = int64(i + 1)
	}

	scheduler := New(
		&fakeEndpointRepository{endpoints: endpoints},
		trackingService,
		newTestLogger(),
		0,
	)
	scheduler.maxConcurrency = maxConcurrent

	scheduler.runDueChecks(context.Background())

	if maxObserved > maxConcurrent {
		t.Errorf("observed concurrency %d, limit is %d", maxObserved, maxConcurrent)
	}

	if maxObserved < 2 {
		t.Errorf("expected concurrent execution, observed %d", maxObserved)
	}

	if len(service.getCheckedIDs()) != endpointCount {
		t.Errorf("not all endpoints were checked")
	}
}

type trackingChecksService struct {
	base  ChecksService
	check func(context.Context, int64)
}

func (s *trackingChecksService) CheckEndpoint(
	ctx context.Context,
	endpointID int64,
) (checkRepo.CheckResult, error) {
	s.check(ctx, endpointID)
	return s.base.CheckEndpoint(ctx, endpointID)
}

func TestSchedulerStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repository := &fakeEndpointRepository{
		endpoints: []endpointRepo.Endpoint{
			{ID: 1},
			{ID: 2},
		},
	}

	service := &fakeChecksService{errors: make(map[int64]error)}
	scheduler := New(repository, service, newTestLogger(), 0)

	scheduler.runDueChecks(ctx)

	if len(service.getCheckedIDs()) != 0 {
		t.Errorf("checked endpoints after cancellation")
	}
}
