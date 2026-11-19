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

	createCalls int
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
	if f.getByIDFn == nil {
		return repository.Endpoint{}, errors.New("unexpected GetEndpointByID call")
	}

	return f.getByIDFn(ctx, id)
}

func (f *fakeEndpointRepository) GetEndpoints(
	ctx context.Context,
	isActive *bool,
) ([]repository.Endpoint, error) {
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
	if f.updateFn == nil {
		return repository.Endpoint{}, errors.New("unexpected UpdateEndpoint call")
	}

	return f.updateFn(ctx, id, interval, isActive)
}

func (f *fakeEndpointRepository) DeleteEndpoint(
	ctx context.Context,
	id int64,
) error {
	if f.deleteFn == nil {
		return errors.New("unexpected DeleteEndpoint call")
	}

	return f.deleteFn(ctx, id)
}

func newTestHandler(repo EndpointRepository) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(repo, logger)
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
