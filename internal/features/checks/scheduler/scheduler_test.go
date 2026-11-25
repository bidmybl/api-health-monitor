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

func (f *fakeEndpointRepository) TryAcquireCheckLock(
	ctx context.Context,
	endpointID int64,
) (func(context.Context) error, bool, error) {
	return func(context.Context) error {
		return nil
	}, true, nil
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

	baseService := &fakeChecksService{
		errors: make(map[int64]error),
	}

	service := &trackingChecksService{
		base:    baseService,
		started: make(chan struct{}, endpointCount),
		release: make(chan struct{}),
	}

	endpoints := make([]endpointRepo.Endpoint, endpointCount)
	for i := range endpoints {
		endpoints[i].ID = int64(i + 1)
	}

	scheduler := New(
		&fakeEndpointRepository{endpoints: endpoints},
		service,
		newTestLogger(),
		0,
	)
	scheduler.maxConcurrency = maxConcurrent

	done := make(chan struct{})

	go func() {
		scheduler.runDueChecks(context.Background())
		close(done)
	}()

	// Wait until all workers have started their first check.
	startTimeout := time.NewTimer(5 * time.Second)
	defer startTimeout.Stop()

	for range maxConcurrent {
		select {
		case <-service.started:
		case <-startTimeout.C:
			close(service.release)
			<-done
			t.Fatal("timed out waiting for concurrent checks to start")
		}
	}

	// Allow all checks to finish.
	close(service.release)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not finish")
	}

	if got := service.maxObservedConcurrency(); got != maxConcurrent {
		t.Errorf(
			"maximum observed concurrency = %d, want %d",
			got,
			maxConcurrent,
		)
	}

	checkedIDs := baseService.getCheckedIDs()
	if len(checkedIDs) != endpointCount {
		t.Errorf(
			"checked %d endpoints, want %d",
			len(checkedIDs),
			endpointCount,
		)
	}
}

type trackingChecksService struct {
	base    *fakeChecksService
	started chan struct{}
	release chan struct{}

	mu          sync.Mutex
	active      int
	maxObserved int
}

func (s *trackingChecksService) CheckEndpoint(
	ctx context.Context,
	endpointID int64,
) (checkRepo.CheckResult, error) {
	s.mu.Lock()

	s.active++
	if s.active > s.maxObserved {
		s.maxObserved = s.active
	}

	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.active--
		s.mu.Unlock()
	}()

	s.started <- struct{}{}

	select {
	case <-s.release:
	case <-ctx.Done():
		return checkRepo.CheckResult{}, ctx.Err()
	}

	return s.base.CheckEndpoint(ctx, endpointID)
}

func (s *trackingChecksService) maxObservedConcurrency() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.maxObserved
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
