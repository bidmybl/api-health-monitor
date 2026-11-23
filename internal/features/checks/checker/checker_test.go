package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheckerCheck(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		expectedStatus int
	}{
		{
			name:           "successful response",
			statusCode:     http.StatusOK,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "server error response",
			statusCode:     http.StatusInternalServerError,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "not found response",
			statusCode:     http.StatusNotFound,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
				}),
			)
			defer server.Close()

			c := NewChecker(time.Second)
			result := c.Check(context.Background(), server.URL)

			if result.HTTPStatusCode == nil {
				t.Fatal("expected HTTP status code, got nil")
			}

			if *result.HTTPStatusCode != tt.expectedStatus {
				t.Errorf(
					"status code = %d, want %d",
					*result.HTTPStatusCode,
					tt.expectedStatus,
				)
			}

			if result.ErrorMessage != nil {
				t.Errorf("unexpected error: %s", *result.ErrorMessage)
			}

			if result.CompletedAt.Before(result.StartedAt) {
				t.Error("completed time is before started time")
			}

			if result.ResponseTimeMS < 0 {
				t.Errorf("negative response time: %d", result.ResponseTimeMS)
			}
		})
	}
}

func TestCheckerNetworkError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	serverURL := server.URL
	server.Close()

	c := NewChecker(time.Second)
	result := c.Check(context.Background(), serverURL)

	if result.HTTPStatusCode != nil {
		t.Errorf("expected nil status code, got %d", *result.HTTPStatusCode)
	}

	if result.ErrorMessage == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestCheckerInvalidURL(t *testing.T) {
	tests := []string{
		"",
		"ftp://example.com",
		"http://",
		"http://user:password@example.com",
	}

	c := NewChecker(time.Second)

	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			result := c.Check(context.Background(), rawURL)

			if result.ErrorMessage == nil {
				t.Fatal("expected validation error, got nil")
			}

			if result.HTTPStatusCode != nil {
				t.Errorf("expected nil status code, got %d", *result.HTTPStatusCode)
			}
		})
	}
}

func TestCheckerContextCanceled(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewChecker(time.Second)
	result := c.Check(ctx, server.URL)

	if result.ErrorMessage == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}

func TestCheckerDoesNotFollowRedirects(t *testing.T) {
	var targetRequests atomic.Int32

	target := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			targetRequests.Add(1)
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer target.Close()

	source := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL, http.StatusFound)
		}),
	)
	defer source.Close()

	c := NewChecker(time.Second)
	result := c.Check(context.Background(), source.URL)

	if result.HTTPStatusCode == nil {
		t.Fatal("expected redirect status code, got nil")
	}

	if *result.HTTPStatusCode != http.StatusFound {
		t.Errorf(
			"status code = %d, want %d",
			*result.HTTPStatusCode,
			http.StatusFound,
		)
	}

	if targetRequests.Load() != 0 {
		t.Errorf("redirect target received %d requests, want 0", targetRequests.Load())
	}
}
