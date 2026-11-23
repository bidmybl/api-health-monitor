package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

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
	checkedIDs []int64
	errors     map[int64]error
}

func (f *fakeChecksService) CheckEndpoint(
	ctx context.Context,
	endpointID int64,
) (checkRepo.CheckResult, error) {
	f.checkedIDs = append(f.checkedIDs, endpointID)

	if err := f.errors[endpointID]; err != nil {
		return checkRepo.CheckResult{}, err
	}

	return checkRepo.CheckResult{}, nil
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

	repository := &fakeEndpointRepository{
		endpoints: endpoints,
	}

	service := &fakeChecksService{
		errors: make(map[int64]error),
	}

	scheduler := New(
		repository,
		service,
		newTestLogger(),
		0,
	)

	scheduler.runDueChecks(context.Background())

	if len(service.checkedIDs) != len(endpoints) {
		t.Fatalf(
			"checked %d endpoints, want %d",
			len(service.checkedIDs),
			len(endpoints),
		)
	}

	for i, endpoint := range endpoints {
		if service.checkedIDs[i] != endpoint.ID {
			t.Errorf(
				"checkedIDs[%d] = %d, want %d",
				i,
				service.checkedIDs[i],
				endpoint.ID,
			)
		}
	}
}

func TestSchedulerContinuesAfterCheckError(t *testing.T) {
	endpoints := []endpointRepo.Endpoint{
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}

	repository := &fakeEndpointRepository{
		endpoints: endpoints,
	}

	service := &fakeChecksService{
		errors: map[int64]error{
			2: errors.New("check failed"),
		},
	}

	scheduler := New(
		repository,
		service,
		newTestLogger(),
		0,
	)

	scheduler.runDueChecks(context.Background())

	if len(service.checkedIDs) != 3 {
		t.Fatalf(
			"checked %d endpoints, want 3",
			len(service.checkedIDs),
		)
	}

	if service.checkedIDs[2] != 3 {
		t.Errorf(
			"last checked endpoint ID = %d, want 3",
			service.checkedIDs[2],
		)
	}
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

	service := &fakeChecksService{
		errors: make(map[int64]error),
	}

	scheduler := New(
		repository,
		service,
		newTestLogger(),
		0,
	)

	scheduler.runDueChecks(ctx)

	if len(service.checkedIDs) != 0 {
		t.Errorf(
			"checked %d endpoints, want 0",
			len(service.checkedIDs),
		)
	}
}
