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

type fakeCheckRepository struct {
	result     checkRepo.CheckResult
	err        error
	calls      int
	endpointID int64
	input      checks.Result

	history           []checkRepo.CheckResult
	historyErr        error
	historyCalls      int
	historyEndpointID int64
	historyLimit      int
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

func (f *fakeCheckRepository) GetByEndpointID(
	ctx context.Context,
	endpointID int64,
	limit int,
) ([]checkRepo.CheckResult, error) {
	f.historyCalls++
	f.historyEndpointID = endpointID
	f.historyLimit = limit

	if f.historyErr != nil {
		return nil, f.historyErr
	}

	return f.history, nil
}

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
		t.Errorf("endpointRepository calls = %d, want 1", endpointRepository.calls)
	}

	if checker.calls != 1 {
		t.Errorf("checker calls = %d, want 1", checker.calls)
	}

	if checker.url != endpoint.URL {
		t.Errorf("checker URL = %q, want %q", checker.url, endpoint.URL)
	}

	if checkRepository.calls != 1 {
		t.Errorf("checkRepository calls = %d, want 1", checkRepository.calls)
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
		t.Errorf("error = %v, want %v", err, endpointRepo.ErrEndpointNotFound)
	}

	if endpointRepository.calls != 1 {
		t.Errorf("endpointRepository calls = %d, want 1", endpointRepository.calls)
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
		t.Errorf("endpointRepository calls = %d, want 1", endpointRepository.calls)
	}

	if checker.calls != 1 {
		t.Errorf("checker calls = %d, want 1", checker.calls)
	}

	if checkRepository.calls != 1 {
		t.Errorf("checkRepository calls = %d, want 1", checkRepository.calls)
	}
}

func TestServiceGetCheckHistory(t *testing.T) {
	endpoint := endpointRepo.Endpoint{
		ID:  42,
		URL: "https://example.com",
	}

	now := time.Now()
	statusCode := http.StatusOK

	history := []checkRepo.CheckResult{
		{
			ID:             2,
			EndpointID:     endpoint.ID,
			StartedAt:      now,
			CompletedAt:    now.Add(100 * time.Millisecond),
			HTTPStatusCode: &statusCode,
			ResponseTimeMS: 100,
			IsSuccess:      true,
		},
		{
			ID:             1,
			EndpointID:     endpoint.ID,
			StartedAt:      now.Add(-time.Minute),
			CompletedAt:    now.Add(-time.Minute + 100*time.Millisecond),
			HTTPStatusCode: &statusCode,
			ResponseTimeMS: 100,
			IsSuccess:      true,
		},
	}

	endpointRepository := &fakeEndpointRepository{
		endpoint: endpoint,
	}

	checkRepository := &fakeCheckRepository{
		history: history,
	}

	svc := NewService(
		endpointRepository,
		&fakeChecker{},
		checkRepository,
	)

	result, err := svc.GetCheckHistory(context.Background(), endpoint.ID, 10)
	if err != nil {
		t.Fatalf("GetCheckHistory() error = %v", err)
	}

	if len(result) != len(history) {
		t.Fatalf("result length = %d, want %d", len(result), len(history))
	}

	if result[0].ID != history[0].ID {
		t.Errorf("result[0].ID = %d, want %d", result[0].ID, history[0].ID)
	}

	if endpointRepository.calls != 1 {
		t.Errorf("endpointRepository calls = %d, want 1", endpointRepository.calls)
	}

	if checkRepository.historyCalls != 1 {
		t.Errorf("checkRepository history calls = %d, want 1", checkRepository.historyCalls)
	}

	if checkRepository.historyEndpointID != endpoint.ID {
		t.Errorf(
			"history endpoint ID = %d, want %d",
			checkRepository.historyEndpointID,
			endpoint.ID,
		)
	}

	if checkRepository.historyLimit != 10 {
		t.Errorf("history limit = %d, want 10", checkRepository.historyLimit)
	}
}

func TestServiceGetCheckHistoryWhenEndpointNotFound(t *testing.T) {
	endpointRepository := &fakeEndpointRepository{
		err: endpointRepo.ErrEndpointNotFound,
	}

	checkRepository := &fakeCheckRepository{}

	svc := NewService(
		endpointRepository,
		&fakeChecker{},
		checkRepository,
	)

	_, err := svc.GetCheckHistory(context.Background(), 999, 10)

	if !errors.Is(err, endpointRepo.ErrEndpointNotFound) {
		t.Errorf("error = %v, want %v", err, endpointRepo.ErrEndpointNotFound)
	}

	if endpointRepository.calls != 1 {
		t.Errorf("endpointRepository calls = %d, want 1", endpointRepository.calls)
	}

	if checkRepository.historyCalls != 0 {
		t.Errorf("checkRepository history calls = %d, want 0", checkRepository.historyCalls)
	}
}

func TestServiceGetCheckHistoryWhenRepositoryFails(t *testing.T) {
	dbErr := errors.New("database unavailable")

	endpointRepository := &fakeEndpointRepository{
		endpoint: endpointRepo.Endpoint{
			ID:  42,
			URL: "https://example.com",
		},
	}

	checkRepository := &fakeCheckRepository{
		historyErr: dbErr,
	}

	svc := NewService(
		endpointRepository,
		&fakeChecker{},
		checkRepository,
	)

	_, err := svc.GetCheckHistory(context.Background(), 42, 10)

	if !errors.Is(err, dbErr) {
		t.Errorf("error = %v, want %v", err, dbErr)
	}

	if endpointRepository.calls != 1 {
		t.Errorf("endpointRepository calls = %d, want 1", endpointRepository.calls)
	}

	if checkRepository.historyCalls != 1 {
		t.Errorf("checkRepository history calls = %d, want 1", checkRepository.historyCalls)
	}
}
