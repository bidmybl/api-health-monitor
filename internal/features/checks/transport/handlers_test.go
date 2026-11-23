package transport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	checkRepo "github.com/bidmybl/api-health-monitor/internal/features/checks/repository"
	endpointRepo "github.com/bidmybl/api-health-monitor/internal/features/endpoints/repository"
)

type fakeCheckService struct {
	result     checkRepo.CheckResult
	err        error
	calls      int
	endpointID int64

	history           []checkRepo.CheckResult
	historyErr        error
	historyCalls      int
	historyEndpointID int64
	historyLimit      int
}

func (f *fakeCheckService) CheckEndpoint(
	ctx context.Context,
	endpointID int64,
) (checkRepo.CheckResult, error) {
	f.calls++
	f.endpointID = endpointID

	if f.err != nil {
		return checkRepo.CheckResult{}, f.err
	}

	return f.result, nil
}

func (f *fakeCheckService) GetCheckHistory(
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

func newTestHandler(service CheckService) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(service, logger)
}

func newTestRouter(handler *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /endpoints/{id}/checks", handler.CheckEndpoint)
	mux.HandleFunc("GET /endpoints/{id}/checks", handler.GetCheckHistory)
	return mux
}

func TestHandlerCheckEndpoint(t *testing.T) {
	now := time.Now().UTC()
	statusCode := http.StatusOK

	tests := []struct {
		name           string
		endpointID     string
		serviceError   error
		wantStatusCode int
		wantBody       string
		wantCalls      int
		wantEndpointID int64
	}{
		{
			name:           "success",
			endpointID:     "42",
			wantStatusCode: http.StatusOK,
			wantBody:       `"is_success":true`,
			wantCalls:      1,
			wantEndpointID: 42,
		},
		{
			name:           "invalid endpoint ID",
			endpointID:     "abc",
			wantStatusCode: http.StatusBadRequest,
			wantBody:       `"error":"invalid endpoint ID"`,
			wantCalls:      0,
		},
		{
			name:           "endpoint not found",
			endpointID:     "999",
			serviceError:   endpointRepo.ErrEndpointNotFound,
			wantStatusCode: http.StatusNotFound,
			wantBody:       `"error":"endpoint not found"`,
			wantCalls:      1,
			wantEndpointID: 999,
		},
		{
			name:           "internal error",
			endpointID:     "42",
			serviceError:   errors.New("database unavailable"),
			wantStatusCode: http.StatusInternalServerError,
			wantBody:       `"error":"internal server error"`,
			wantCalls:      1,
			wantEndpointID: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeCheckService{
				result: checkRepo.CheckResult{
					ID:             10,
					EndpointID:     42,
					StartedAt:      now,
					CompletedAt:    now.Add(100 * time.Millisecond),
					HTTPStatusCode: &statusCode,
					ResponseTimeMS: 100,
					IsSuccess:      true,
				},
				err: tt.serviceError,
			}

			handler := newTestHandler(service)
			router := newTestRouter(handler)

			request := httptest.NewRequest(
				http.MethodPost,
				"/endpoints/"+tt.endpointID+"/checks",
				nil,
			)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatusCode {
				t.Errorf("status code = %d, want %d", recorder.Code, tt.wantStatusCode)
			}

			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Errorf(
					"response body = %s, want it to contain %s",
					recorder.Body.String(),
					tt.wantBody,
				)
			}

			if service.calls != tt.wantCalls {
				t.Errorf("service calls = %d, want %d", service.calls, tt.wantCalls)
			}

			if service.endpointID != tt.wantEndpointID {
				t.Errorf("endpoint ID = %d, want %d", service.endpointID, tt.wantEndpointID)
			}
		})
	}
}

func TestHandlerGetCheckHistory(t *testing.T) {
	now := time.Now().UTC()
	statusCode := http.StatusOK

	tests := []struct {
		name             string
		endpointID       string
		query            string
		history          []checkRepo.CheckResult
		serviceError     error
		wantStatusCode   int
		wantBody         string
		wantHistoryCalls int
		wantEndpointID   int64
		wantHistoryLimit int
	}{
		{
			name:             "success with default limit",
			endpointID:       "42",
			history:          []checkRepo.CheckResult{},
			wantStatusCode:   http.StatusOK,
			wantBody:         "[]",
			wantHistoryCalls: 1,
			wantEndpointID:   42,
			wantHistoryLimit: 20,
		},
		{
			name:       "success with custom limit",
			endpointID: "42",
			query:      "?limit=5",
			history: []checkRepo.CheckResult{
				{
					ID:             10,
					EndpointID:     42,
					StartedAt:      now,
					CompletedAt:    now.Add(100 * time.Millisecond),
					HTTPStatusCode: &statusCode,
					ResponseTimeMS: 100,
					IsSuccess:      true,
				},
			},
			wantStatusCode:   http.StatusOK,
			wantBody:         `"id":10`,
			wantHistoryCalls: 1,
			wantEndpointID:   42,
			wantHistoryLimit: 5,
		},
		{
			name:           "invalid endpoint ID",
			endpointID:     "abc",
			wantStatusCode: http.StatusBadRequest,
			wantBody:       `"error":"invalid endpoint ID"`,
		},
		{
			name:           "zero limit",
			endpointID:     "42",
			query:          "?limit=0",
			wantStatusCode: http.StatusBadRequest,
			wantBody:       `"error":"limit must be between 1 and 100"`,
		},
		{
			name:           "limit above maximum",
			endpointID:     "42",
			query:          "?limit=500",
			wantStatusCode: http.StatusBadRequest,
			wantBody:       `"error":"limit must be between 1 and 100"`,
		},
		{
			name:           "non-numeric limit",
			endpointID:     "42",
			query:          "?limit=abc",
			wantStatusCode: http.StatusBadRequest,
			wantBody:       `"error":"limit must be between 1 and 100"`,
		},
		{
			name:             "endpoint not found",
			endpointID:       "999",
			serviceError:     endpointRepo.ErrEndpointNotFound,
			wantStatusCode:   http.StatusNotFound,
			wantBody:         `"error":"endpoint not found"`,
			wantHistoryCalls: 1,
			wantEndpointID:   999,
			wantHistoryLimit: 20,
		},
		{
			name:             "internal error",
			endpointID:       "42",
			serviceError:     errors.New("database unavailable"),
			wantStatusCode:   http.StatusInternalServerError,
			wantBody:         `"error":"internal server error"`,
			wantHistoryCalls: 1,
			wantEndpointID:   42,
			wantHistoryLimit: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeCheckService{
				history:    tt.history,
				historyErr: tt.serviceError,
			}

			handler := newTestHandler(service)
			router := newTestRouter(handler)

			request := httptest.NewRequest(
				http.MethodGet,
				"/endpoints/"+tt.endpointID+"/checks"+tt.query,
				nil,
			)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatusCode {
				t.Errorf("status code = %d, want %d", recorder.Code, tt.wantStatusCode)
			}

			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Errorf(
					"response body = %s, want it to contain %s",
					recorder.Body.String(),
					tt.wantBody,
				)
			}

			if service.historyCalls != tt.wantHistoryCalls {
				t.Errorf(
					"service history calls = %d, want %d",
					service.historyCalls,
					tt.wantHistoryCalls,
				)
			}

			if service.historyEndpointID != tt.wantEndpointID {
				t.Errorf(
					"history endpoint ID = %d, want %d",
					service.historyEndpointID,
					tt.wantEndpointID,
				)
			}

			if service.historyLimit != tt.wantHistoryLimit {
				t.Errorf(
					"history limit = %d, want %d",
					service.historyLimit,
					tt.wantHistoryLimit,
				)
			}
		})
	}
}
