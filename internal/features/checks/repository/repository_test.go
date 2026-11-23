package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bidmybl/api-health-monitor/internal/features/checks"
)

func setupTestRepository(t *testing.T) (*Repository, *pgxpool.Pool) {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	if databaseURL == os.Getenv("DATABASE_URL") {
		t.Fatal("TEST_DATABASE_URL must use a separate database")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create connection pool: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("connect to test database: %v", err)
	}

	t.Cleanup(pool.Close)

	return NewRepository(pool), pool
}

func createTestEndpoint(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()

	ctx := context.Background()

	url := fmt.Sprintf(
		"https://example.test/%s-%d",
		strings.ReplaceAll(t.Name(), "/", "-"),
		time.Now().UnixNano(),
	)

	var endpointID int64

	err := pool.QueryRow(
		ctx,
		`
			INSERT INTO monitored_endpoints (url)
			VALUES ($1)
			RETURNING id;
		`,
		url,
	).Scan(&endpointID)
	if err != nil {
		t.Fatalf("create test endpoint: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			ctx,
			"DELETE FROM monitored_endpoints WHERE id = $1",
			endpointID,
		)
		if err != nil {
			t.Errorf("cleanup test endpoint: %v", err)
		}
	})

	return endpointID
}

func TestRepositoryCreate(t *testing.T) {
	repo, pool := setupTestRepository(t)
	endpointID := createTestEndpoint(t, pool)

	startedAt := time.Now().Add(-time.Second).Truncate(time.Microsecond)
	completedAt := time.Now().Truncate(time.Microsecond)

	statusOK := 200
	statusInternalError := 500
	networkError := "connection refused"

	tests := []struct {
		name        string
		result      checks.Result
		wantSuccess bool
	}{
		{
			name: "successful check",
			result: checks.Result{
				HTTPStatusCode: &statusOK,
				ResponseTimeMS: 120,
				StartedAt:      startedAt,
				CompletedAt:    completedAt,
			},
			wantSuccess: true,
		},
		{
			name: "server error response",
			result: checks.Result{
				HTTPStatusCode: &statusInternalError,
				ResponseTimeMS: 250,
				StartedAt:      startedAt,
				CompletedAt:    completedAt,
			},
			wantSuccess: false,
		},
		{
			name: "network error",
			result: checks.Result{
				ErrorMessage:   &networkError,
				ResponseTimeMS: 1000,
				StartedAt:      startedAt,
				CompletedAt:    completedAt,
			},
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.Create(
				context.Background(),
				endpointID,
				tt.result,
			)
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			if got.ID <= 0 {
				t.Errorf("ID = %d, want positive ID", got.ID)
			}

			if got.EndpointID != endpointID {
				t.Errorf(
					"EndpointID = %d, want %d",
					got.EndpointID,
					endpointID,
				)
			}

			if got.IsSuccess != tt.wantSuccess {
				t.Errorf(
					"IsSuccess = %v, want %v",
					got.IsSuccess,
					tt.wantSuccess,
				)
			}

			if got.ResponseTimeMS != tt.result.ResponseTimeMS {
				t.Errorf(
					"ResponseTimeMS = %d, want %d",
					got.ResponseTimeMS,
					tt.result.ResponseTimeMS,
				)
			}

			if !got.StartedAt.Equal(tt.result.StartedAt) {
				t.Errorf(
					"StartedAt = %v, want %v",
					got.StartedAt,
					tt.result.StartedAt,
				)
			}

			if !got.CompletedAt.Equal(tt.result.CompletedAt) {
				t.Errorf(
					"CompletedAt = %v, want %v",
					got.CompletedAt,
					tt.result.CompletedAt,
				)
			}

			if (got.HTTPStatusCode == nil) != (tt.result.HTTPStatusCode == nil) {
				t.Error("HTTPStatusCode nil state does not match")
			} else if got.HTTPStatusCode != nil &&
				*got.HTTPStatusCode != *tt.result.HTTPStatusCode {
				t.Errorf(
					"HTTPStatusCode = %d, want %d",
					*got.HTTPStatusCode,
					*tt.result.HTTPStatusCode,
				)
			}

			if (got.ErrorMessage == nil) != (tt.result.ErrorMessage == nil) {
				t.Error("ErrorMessage nil state does not match")
			} else if got.ErrorMessage != nil &&
				*got.ErrorMessage != *tt.result.ErrorMessage {
				t.Errorf(
					"ErrorMessage = %q, want %q",
					*got.ErrorMessage,
					*tt.result.ErrorMessage,
				)
			}
		})
	}
}

func TestRepositoryCreateWithNonExistingEndpoint(t *testing.T) {
	repo, _ := setupTestRepository(t)

	now := time.Now()

	result := checks.Result{
		HTTPStatusCode: nil,
		ErrorMessage:   stringPointer("connection refused"),
		StartedAt:      now,
		CompletedAt:    now,
	}

	_, err := repo.Create(
		context.Background(),
		-1,
		result,
	)

	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) {
		t.Fatalf("expected PostgreSQL error, got %v", err)
	}

	if pgErr.Code != "23503" {
		t.Errorf("error code = %s, want 23503 (foreign_key_violation)", pgErr.Code)
	}
}

func TestRepositoryGetByEndpointID(t *testing.T) {
	repo, pool := setupTestRepository(t)

	endpointID := createTestEndpoint(t, pool)
	otherEndpointID := createTestEndpoint(t, pool)
	emptyEndpointID := createTestEndpoint(t, pool)

	ctx := context.Background()
	baseTime := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)
	statusOK := 200

	createdResults := make([]CheckResult, 0, 3)

	for i := 0; i < 3; i++ {
		startedAt := baseTime.Add(time.Duration(i) * time.Minute)

		result, err := repo.Create(
			ctx,
			endpointID,
			checks.Result{
				HTTPStatusCode: &statusOK,
				ResponseTimeMS: 100 + i,
				StartedAt:      startedAt,
				CompletedAt:    startedAt.Add(100 * time.Millisecond),
			},
		)
		if err != nil {
			t.Fatalf("create check result %d: %v", i, err)
		}

		createdResults = append(createdResults, result)
	}

	otherStartedAt := baseTime.Add(10 * time.Minute)

	_, err := repo.Create(
		ctx,
		otherEndpointID,
		checks.Result{
			HTTPStatusCode: &statusOK,
			ResponseTimeMS: 200,
			StartedAt:      otherStartedAt,
			CompletedAt:    otherStartedAt.Add(100 * time.Millisecond),
		},
	)
	if err != nil {
		t.Fatalf("create check result for another endpoint: %v", err)
	}

	t.Run("returns newest results first and respects limit", func(t *testing.T) {
		got, err := repo.GetByEndpointID(ctx, endpointID, 2)
		if err != nil {
			t.Fatalf("GetByEndpointID() error = %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("results length = %d, want 2", len(got))
		}

		wantIDs := []int64{
			createdResults[2].ID,
			createdResults[1].ID,
		}

		for i, wantID := range wantIDs {
			if got[i].ID != wantID {
				t.Errorf(
					"results[%d].ID = %d, want %d",
					i,
					got[i].ID,
					wantID,
				)
			}
		}

		for _, result := range got {
			if result.EndpointID != endpointID {
				t.Errorf(
					"EndpointID = %d, want %d",
					result.EndpointID,
					endpointID,
				)
			}
		}
	})

	t.Run("returns all available results when limit is larger", func(t *testing.T) {
		got, err := repo.GetByEndpointID(ctx, endpointID, 10)
		if err != nil {
			t.Fatalf("GetByEndpointID() error = %v", err)
		}

		if len(got) != 3 {
			t.Fatalf("results length = %d, want 3", len(got))
		}
	})

	t.Run("returns empty history", func(t *testing.T) {
		got, err := repo.GetByEndpointID(ctx, emptyEndpointID, 20)
		if err != nil {
			t.Fatalf("GetByEndpointID() error = %v", err)
		}

		if got == nil {
			t.Fatal("results = nil, want empty slice")
		}

		if len(got) != 0 {
			t.Errorf("results length = %d, want 0", len(got))
		}
	})
}

func stringPointer(value string) *string {
	return &value
}
