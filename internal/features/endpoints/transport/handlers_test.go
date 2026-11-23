package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bidmybl/api-health-monitor/internal/features/endpoints/repository"
)

type fakeEndpointRepository struct {
	createFn  func(context.Context, string, *int) (repository.Endpoint, error)
	getByIDFn func(context.Context, int64) (repository.Endpoint, error)
	getAllFn  func(context.Context, *bool) ([]repository.Endpoint, error)
	updateFn  func(context.Context, int64, *int, *bool) (repository.Endpoint, error)
	deleteFn  func(context.Context, int64) error

	createCalls  int
	getByIDCalls int
	getAllCalls  int
	updateCalls  int
	deleteCalls  int
}

func (f *fakeEndpointRepository) Create(
	ctx context.Context,
	url string,
	interval *int,
) (repository.Endpoint, error) {
	f.createCalls++

	if f.createFn == nil {
		return repository.Endpoint{}, errors.New("unexpected Create call")
	}

	return f.createFn(ctx, url, interval)
}

func (f *fakeEndpointRepository) GetEndpointByID(
	ctx context.Context,
	id int64,
) (repository.Endpoint, error) {
	f.getByIDCalls++

	if f.getByIDFn == nil {
		return repository.Endpoint{}, errors.New("unexpected GetEndpointByID call")
	}

	return f.getByIDFn(ctx, id)
}

func (f *fakeEndpointRepository) GetEndpoints(
	ctx context.Context,
	isActive *bool,
) ([]repository.Endpoint, error) {
	f.getAllCalls++

	if f.getAllFn == nil {
		return nil, errors.New("unexpected GetEndpoints call")
	}

	return f.getAllFn(ctx, isActive)
}

func (f *fakeEndpointRepository) UpdateEndpoint(
	ctx context.Context,
	id int64,
	interval *int,
	isActive *bool,
) (repository.Endpoint, error) {
	f.updateCalls++

	if f.updateFn == nil {
		return repository.Endpoint{}, errors.New("unexpected UpdateEndpoint call")
	}

	return f.updateFn(ctx, id, interval, isActive)
}

func (f *fakeEndpointRepository) DeleteEndpoint(
	ctx context.Context,
	id int64,
) error {
	f.deleteCalls++

	if f.deleteFn == nil {
		return errors.New("unexpected DeleteEndpoint call")
	}

	return f.deleteFn(ctx, id)
}

func newTestHandler(repo EndpointRepository) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(repo, logger)
}

func testEndpoint() repository.Endpoint {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

	return repository.Endpoint{
		ID:                   1,
		URL:                  "https://example.com",
		CheckIntervalSeconds: 60,
		IsActive:             true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func newRequestWithID(method, target, id, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.SetPathValue("id", id)
	return req
}

func TestCreateEndpoint(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		body           string
		createFn       func(context.Context, string, *int) (repository.Endpoint, error)
		wantStatus     int
		wantCreateCall int
	}{
		{
			name: "successful creation",
			body: `{"url":"https://example.com","check_interval_seconds":60}`,
			createFn: func(
				ctx context.Context,
				url string,
				interval *int,
			) (repository.Endpoint, error) {
				return repository.Endpoint{
					ID:                   1,
					URL:                  url,
					CheckIntervalSeconds: *interval,
					IsActive:             true,
					CreatedAt:            now,
					UpdatedAt:            now,
				}, nil
			},
			wantStatus:     http.StatusCreated,
			wantCreateCall: 1,
		},
		{
			name:           "invalid JSON",
			body:           `{"url":`,
			wantStatus:     http.StatusBadRequest,
			wantCreateCall: 0,
		},
		{
			name:           "invalid URL",
			body:           `{"url":"not-a-url"}`,
			wantStatus:     http.StatusBadRequest,
			wantCreateCall: 0,
		},
		{
			name:           "URL with credentials",
			body:           `{"url":"https://user:pass@example.com"}`,
			wantStatus:     http.StatusBadRequest,
			wantCreateCall: 0,
		},
		{
			name:           "invalid interval",
			body:           `{"url":"https://example.com","check_interval_seconds":-1}`,
			wantStatus:     http.StatusBadRequest,
			wantCreateCall: 0,
		},
		{
			name: "endpoint already exists",
			body: `{"url":"https://example.com"}`,
			createFn: func(
				ctx context.Context,
				url string,
				interval *int,
			) (repository.Endpoint, error) {
				return repository.Endpoint{}, repository.ErrEndpointAlreadyExists
			},
			wantStatus:     http.StatusConflict,
			wantCreateCall: 1,
		},
		{
			name: "repository error",
			body: `{"url":"https://example.com"}`,
			createFn: func(
				ctx context.Context,
				url string,
				interval *int,
			) (repository.Endpoint, error) {
				return repository.Endpoint{}, errors.New("database error")
			},
			wantStatus:     http.StatusInternalServerError,
			wantCreateCall: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeRepo := &fakeEndpointRepository{
				createFn: tt.createFn,
			}

			handler := newTestHandler(fakeRepo)

			req := httptest.NewRequest(
				http.MethodPost,
				"/endpoints",
				strings.NewReader(tt.body),
			)
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()

			handler.CreateEndpoint(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body: %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			if fakeRepo.createCalls != tt.wantCreateCall {
				t.Errorf(
					"Create calls = %d, want %d",
					fakeRepo.createCalls,
					tt.wantCreateCall,
				)
			}

			if tt.wantStatus == http.StatusCreated {
				var got EndpointResponseDTO

				if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if got.ID != 1 {
					t.Errorf("ID = %d, want 1", got.ID)
				}

				if got.URL != "https://example.com" {
					t.Errorf(
						"URL = %q, want %q",
						got.URL,
						"https://example.com",
					)
				}

				if got.CheckIntervalSeconds != 60 {
					t.Errorf(
						"CheckIntervalSeconds = %d, want 60",
						got.CheckIntervalSeconds,
					)
				}

				if !got.IsActive {
					t.Error("IsActive = false, want true")
				}
			}
		})
	}
}

func TestGetEndpoint(t *testing.T) {
	tests := []struct {
		name            string
		id              string
		getByIDFn       func(context.Context, int64) (repository.Endpoint, error)
		wantStatus      int
		wantGetByIDCall int
	}{
		{
			name: "successful retrieval",
			id:   "1",
			getByIDFn: func(ctx context.Context, id int64) (repository.Endpoint, error) {
				if id != 1 {
					t.Errorf("ID = %d, want 1", id)
				}
				return testEndpoint(), nil
			},
			wantStatus:      http.StatusOK,
			wantGetByIDCall: 1,
		},
		{
			name:            "invalid ID",
			id:              "abc",
			wantStatus:      http.StatusBadRequest,
			wantGetByIDCall: 0,
		},
		{
			name: "endpoint not found",
			id:   "999",
			getByIDFn: func(ctx context.Context, id int64) (repository.Endpoint, error) {
				return repository.Endpoint{}, repository.ErrEndpointNotFound
			},
			wantStatus:      http.StatusNotFound,
			wantGetByIDCall: 1,
		},
		{
			name: "repository error",
			id:   "1",
			getByIDFn: func(ctx context.Context, id int64) (repository.Endpoint, error) {
				return repository.Endpoint{}, errors.New("database error")
			},
			wantStatus:      http.StatusInternalServerError,
			wantGetByIDCall: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeEndpointRepository{
				getByIDFn: tt.getByIDFn,
			}
			handler := newTestHandler(repo)

			req := newRequestWithID(
				http.MethodGet,
				"/endpoints/"+tt.id,
				tt.id,
				"",
			)
			rec := httptest.NewRecorder()

			handler.GetEndpoint(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body: %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			if repo.getByIDCalls != tt.wantGetByIDCall {
				t.Errorf(
					"GetEndpointByID calls = %d, want %d",
					repo.getByIDCalls,
					tt.wantGetByIDCall,
				)
			}

			if tt.wantStatus == http.StatusOK {
				var got EndpointResponseDTO

				if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if got.ID != 1 {
					t.Errorf("ID = %d, want 1", got.ID)
				}

				if got.URL != "https://example.com" {
					t.Errorf("URL = %q, want https://example.com", got.URL)
				}
			}
		})
	}
}

func TestGetAllEndpoints(t *testing.T) {
	tests := []struct {
		name           string
		target         string
		getAllFn       func(context.Context, *bool) ([]repository.Endpoint, error)
		wantStatus     int
		wantGetAllCall int
	}{
		{
			name:   "successful retrieval without filter",
			target: "/endpoints",
			getAllFn: func(ctx context.Context, isActive *bool) ([]repository.Endpoint, error) {
				if isActive != nil {
					t.Errorf("isActive = %v, want nil", *isActive)
				}
				return []repository.Endpoint{testEndpoint()}, nil
			},
			wantStatus:     http.StatusOK,
			wantGetAllCall: 1,
		},
		{
			name:   "filter active endpoints",
			target: "/endpoints?is_active=true",
			getAllFn: func(ctx context.Context, isActive *bool) ([]repository.Endpoint, error) {
				if isActive == nil || !*isActive {
					t.Error("isActive filter = nil or false, want true")
				}
				return []repository.Endpoint{testEndpoint()}, nil
			},
			wantStatus:     http.StatusOK,
			wantGetAllCall: 1,
		},
		{
			name:   "filter inactive endpoints",
			target: "/endpoints?is_active=false",
			getAllFn: func(ctx context.Context, isActive *bool) ([]repository.Endpoint, error) {
				if isActive == nil || *isActive {
					t.Error("isActive filter = nil or true, want false")
				}
				return []repository.Endpoint{}, nil
			},
			wantStatus:     http.StatusOK,
			wantGetAllCall: 1,
		},
		{
			name:           "invalid filter value",
			target:         "/endpoints?is_active=invalid",
			wantStatus:     http.StatusBadRequest,
			wantGetAllCall: 0,
		},
		{
			name:           "duplicate filter values",
			target:         "/endpoints?is_active=true&is_active=false",
			wantStatus:     http.StatusBadRequest,
			wantGetAllCall: 0,
		},
		{
			name:   "repository error",
			target: "/endpoints",
			getAllFn: func(ctx context.Context, isActive *bool) ([]repository.Endpoint, error) {
				return nil, errors.New("database error")
			},
			wantStatus:     http.StatusInternalServerError,
			wantGetAllCall: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeEndpointRepository{
				getAllFn: tt.getAllFn,
			}
			handler := newTestHandler(repo)

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			rec := httptest.NewRecorder()

			handler.GetAllEndpoints(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body: %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			if repo.getAllCalls != tt.wantGetAllCall {
				t.Errorf(
					"GetEndpoints calls = %d, want %d",
					repo.getAllCalls,
					tt.wantGetAllCall,
				)
			}

			if tt.wantStatus == http.StatusOK {
				var got []EndpointResponseDTO

				if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if got == nil {
					t.Fatal("response = nil, want JSON array")
				}
			}
		})
	}
}

func TestUpdateEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		body       string
		updateFn   func(context.Context, int64, *int, *bool) (repository.Endpoint, error)
		wantStatus int
		wantCalls  int
	}{
		{
			name: "update check interval",
			id:   "1",
			body: `{"check_interval_seconds":120}`,
			updateFn: func(ctx context.Context, id int64, interval *int, isActive *bool) (repository.Endpoint, error) {
				if id != 1 {
					t.Errorf("ID = %d, want 1", id)
				}
				if interval == nil || *interval != 120 {
					t.Errorf("interval = %v, want 120", interval)
				}
				if isActive != nil {
					t.Errorf("isActive = %v, want nil", *isActive)
				}

				endpoint := testEndpoint()
				endpoint.CheckIntervalSeconds = 120
				return endpoint, nil
			},
			wantStatus: http.StatusOK,
			wantCalls:  1,
		},
		{
			name: "update active status to false",
			id:   "1",
			body: `{"is_active":false}`,
			updateFn: func(ctx context.Context, id int64, interval *int, isActive *bool) (repository.Endpoint, error) {
				if interval != nil {
					t.Errorf("interval = %v, want nil", *interval)
				}
				if isActive == nil || *isActive {
					t.Error("isActive = nil or true, want false")
				}

				endpoint := testEndpoint()
				endpoint.IsActive = false
				return endpoint, nil
			},
			wantStatus: http.StatusOK,
			wantCalls:  1,
		},
		{
			name: "update both fields",
			id:   "1",
			body: `{"check_interval_seconds":120,"is_active":false}`,
			updateFn: func(ctx context.Context, id int64, interval *int, isActive *bool) (repository.Endpoint, error) {
				if interval == nil || *interval != 120 {
					t.Errorf("interval = %v, want 120", interval)
				}
				if isActive == nil || *isActive {
					t.Error("isActive = nil or true, want false")
				}
				return testEndpoint(), nil
			},
			wantStatus: http.StatusOK,
			wantCalls:  1,
		},
		{
			name:       "invalid ID",
			id:         "abc",
			body:       `{"is_active":false}`,
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name:       "invalid JSON",
			id:         "1",
			body:       `{"is_active":`,
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name:       "empty update",
			id:         "1",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name:       "invalid interval",
			id:         "1",
			body:       `{"check_interval_seconds":0}`,
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name: "endpoint not found",
			id:   "999",
			body: `{"is_active":false}`,
			updateFn: func(ctx context.Context, id int64, interval *int, isActive *bool) (repository.Endpoint, error) {
				return repository.Endpoint{}, repository.ErrEndpointNotFound
			},
			wantStatus: http.StatusNotFound,
			wantCalls:  1,
		},
		{
			name: "repository error",
			id:   "1",
			body: `{"is_active":false}`,
			updateFn: func(ctx context.Context, id int64, interval *int, isActive *bool) (repository.Endpoint, error) {
				return repository.Endpoint{}, errors.New("database error")
			},
			wantStatus: http.StatusInternalServerError,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeEndpointRepository{
				updateFn: tt.updateFn,
			}
			handler := newTestHandler(repo)

			req := newRequestWithID(
				http.MethodPatch,
				"/endpoints/"+tt.id,
				tt.id,
				tt.body,
			)
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()

			handler.UpdateEndpoint(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body: %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			if repo.updateCalls != tt.wantCalls {
				t.Errorf(
					"UpdateEndpoint calls = %d, want %d",
					repo.updateCalls,
					tt.wantCalls,
				)
			}

			if tt.wantStatus == http.StatusOK {
				var got EndpointResponseDTO

				if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				if got.ID != 1 {
					t.Errorf("ID = %d, want 1", got.ID)
				}
			}
		})
	}
}

func TestDeleteEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		deleteFn   func(context.Context, int64) error
		wantStatus int
		wantCalls  int
	}{
		{
			name: "successful deletion",
			id:   "1",
			deleteFn: func(ctx context.Context, id int64) error {
				if id != 1 {
					t.Errorf("ID = %d, want 1", id)
				}
				return nil
			},
			wantStatus: http.StatusNoContent,
			wantCalls:  1,
		},
		{
			name:       "invalid ID",
			id:         "abc",
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name: "endpoint not found",
			id:   "999",
			deleteFn: func(ctx context.Context, id int64) error {
				return repository.ErrEndpointNotFound
			},
			wantStatus: http.StatusNotFound,
			wantCalls:  1,
		},
		{
			name: "repository error",
			id:   "1",
			deleteFn: func(ctx context.Context, id int64) error {
				return errors.New("database error")
			},
			wantStatus: http.StatusInternalServerError,
			wantCalls:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeEndpointRepository{
				deleteFn: tt.deleteFn,
			}
			handler := newTestHandler(repo)

			req := newRequestWithID(
				http.MethodDelete,
				"/endpoints/"+tt.id,
				tt.id,
				"",
			)
			rec := httptest.NewRecorder()

			handler.DeleteEndpoint(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"status = %d, want %d; body: %s",
					rec.Code,
					tt.wantStatus,
					rec.Body.String(),
				)
			}

			if repo.deleteCalls != tt.wantCalls {
				t.Errorf(
					"DeleteEndpoint calls = %d, want %d",
					repo.deleteCalls,
					tt.wantCalls,
				)
			}

			if tt.wantStatus == http.StatusNoContent && rec.Body.Len() != 0 {
				t.Errorf(
					"response body = %q, want empty body",
					rec.Body.String(),
				)
			}
		})
	}
}
