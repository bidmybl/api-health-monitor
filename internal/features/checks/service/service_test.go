package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/bidmybl/api-health-monitor/internal/features/checks"
	checkRepo "github.com/bidmybl/api-health-monitor/internal/features/checks/repository"
	endpointRepo "github.com/bidmybl/api-health-monitor/internal/features/endpoints/repository"
)

// Fake endpoint repository

type fakeEndpointRepository struct {
	endpoint endpointRepo.Endpoint
	err      error
	calls    int
}

func (f *fakeEndpointRepository) GetEndpointByID(
	ctx context.Context,
	id int64,
) (endpointRepo.Endpoint, error) {
	f.calls++

	if f.err != nil {
		return endpointRepo.Endpoint{}, f.err
	}

	return f.endpoint, nil
}

// Fake checker

type fakeChecker struct {
	result checks.Result
	calls  int
	url    string
}

func (f *fakeChecker) Check(
	ctx context.Context,
	rawURL string,
) checks.Result {
	f.calls++
	f.url = rawURL

	return f.result
}

// Fake check repository

type fakeCheckRepository struct {
	result     checkRepo.CheckResult
	err        error
	calls      int
	endpointID int64
	input      checks.Result
}

func (f *fakeCheckRepository) Create(
	ctx context.Context,
	endpointID int64,
	result checks.Result,
) (checkRepo.CheckResult, error) {
	f.calls++
	f.endpointID = endpointID
	f.input = result

	if f.err != nil {
		return checkRepo.CheckResult{}, f.err
	}

	return f.result, nil
}

// Tests

func TestServiceCheckEndpoint(t *testing.T) {
	now := time.Now()
	statusCode := http.StatusOK

	endpoint := endpointRepo.Endpoint{
		ID:  1,
		URL: "https://example.com",
	}

	checkResult := checks.Result{
		HTTPStatusCode: &statusCode,
		ResponseTimeMS: 100,
		StartedAt:      now,
		CompletedAt:    now.Add(100 * time.Millisecond),
	}

	savedResult := checkRepo.CheckResult{
		ID:             10,
		EndpointID:     endpoint.ID,
		HTTPStatusCode: &statusCode,
		ResponseTimeMS: 100,
		IsSuccess:      true,
		StartedAt:      checkResult.StartedAt,
		CompletedAt:    checkResult.CompletedAt,
	}

	endpointRepository := &fakeEndpointRepository{
		endpoint: endpoint,
	}

	checker := &fakeChecker{
		result: checkResult,
	}

	checkRepository := &fakeCheckRepository{
		result: savedResult,
	}

	svc := NewService(
		endpointRepository,
		checker,
		checkRepository,
	)

	result, err := svc.CheckEndpoint(context.Background(), endpoint.ID)
	if err != nil {
		t.Fatalf("CheckEndpoint() error = %v", err)
	}

	if result.ID != savedResult.ID {
		t.Errorf("result.ID = %d, want %d", result.ID, savedResult.ID)
	}

	if result.EndpointID != endpoint.ID {
		t.Errorf("result.EndpointID = %d, want %d", result.EndpointID, endpoint.ID)
	}

	if endpointRepository.calls != 1 {
		t.Errorf(
			"endpointRepository calls = %d, want 1",
			endpointRepository.calls,
		)
	}

	if checker.calls != 1 {
		t.Errorf("checker calls = %d, want 1", checker.calls)
	}

	if checker.url != endpoint.URL {
		t.Errorf("checker URL = %q, want %q", checker.url, endpoint.URL)
	}

	if checkRepository.calls != 1 {
		t.Errorf(
			"checkRepository calls = %d, want 1",
			checkRepository.calls,
		)
	}

	if checkRepository.endpointID != endpoint.ID {
		t.Errorf(
			"saved endpoint ID = %d, want %d",
			checkRepository.endpointID,
			endpoint.ID,
		)
	}

	if checkRepository.input.ResponseTimeMS != checkResult.ResponseTimeMS {
		t.Errorf(
			"saved response time = %d, want %d",
			checkRepository.input.ResponseTimeMS,
			checkResult.ResponseTimeMS,
		)
	}
}

func TestServiceCheckEndpointWhenEndpointNotFound(t *testing.T) {
	endpointRepository := &fakeEndpointRepository{
		err: endpointRepo.ErrEndpointNotFound,
	}

	checker := &fakeChecker{}

	checkRepository := &fakeCheckRepository{}

	svc := NewService(
		endpointRepository,
		checker,
		checkRepository,
	)

	_, err := svc.CheckEndpoint(context.Background(), 999)

	if !errors.Is(err, endpointRepo.ErrEndpointNotFound) {
		t.Errorf(
			"error = %v, want %v",
			err,
			endpointRepo.ErrEndpointNotFound,
		)
	}

	if endpointRepository.calls != 1 {
		t.Errorf(
			"endpointRepository calls = %d, want 1",
			endpointRepository.calls,
		)
	}

	if checker.calls != 0 {
		t.Errorf("checker calls = %d, want 0", checker.calls)
	}

	if checkRepository.calls != 0 {
		t.Errorf("checkRepository calls = %d, want 0", checkRepository.calls)
	}
}

func TestServiceCheckEndpointWhenSavingFails(t *testing.T) {
	now := time.Now()
	statusCode := http.StatusOK

	endpoint := endpointRepo.Endpoint{
		ID:  1,
		URL: "https://example.com",
	}

	checkResult := checks.Result{
		HTTPStatusCode: &statusCode,
		ResponseTimeMS: 100,
		StartedAt:      now,
		CompletedAt:    now.Add(100 * time.Millisecond),
	}

	dbErr := errors.New("database unavailable")

	endpointRepository := &fakeEndpointRepository{
		endpoint: endpoint,
	}

	checker := &fakeChecker{
		result: checkResult,
	}

	checkRepository := &fakeCheckRepository{
		err: dbErr,
	}

	svc := NewService(
		endpointRepository,
		checker,
		checkRepository,
	)

	_, err := svc.CheckEndpoint(context.Background(), endpoint.ID)

	if !errors.Is(err, dbErr) {
		t.Errorf("error = %v, want %v", err, dbErr)
	}

	if endpointRepository.calls != 1 {
		t.Errorf(
			"endpointRepository calls = %d, want 1",
			endpointRepository.calls,
		)
	}

	if checker.calls != 1 {
		t.Errorf("checker calls = %d, want 1", checker.calls)
	}

	if checkRepository.calls != 1 {
		t.Errorf(
			"checkRepository calls = %d, want 1",
			checkRepository.calls,
		)
	}
}
