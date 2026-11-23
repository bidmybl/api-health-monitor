package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func setupTestRepository(t *testing.T) *Repository {
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

	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("connect to test database: %v", err)
	}

	return NewRepository(pool)
}

func uniqueTestURL(t *testing.T) string {
	t.Helper()

	return fmt.Sprintf(
		"https://example.test/%s-%d",
		strings.ReplaceAll(t.Name(), "/", "-"),
		time.Now().UnixNano(),
	)
}

func TestRepositoryCreateEndpoint(t *testing.T) {
	repo := setupTestRepository(t)

	ctx := context.Background()
	interval := 15

	url := uniqueTestURL(t)

	created, err := repo.Create(ctx, url, &interval)
	if err != nil {
		t.Fatalf("Error creating record in DB, %v", err)
	}

	t.Cleanup(func() {
		_ = repo.DeleteEndpoint(ctx, created.ID)
	})

	// Проверка дубликата
	_, err = repo.Create(ctx, created.URL, &interval)
	if !errors.Is(err, ErrEndpointAlreadyExists) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			ErrEndpointAlreadyExists,
		)
	}

	if created.URL != url {
		t.Errorf("URL = %q, want %q", created.URL, url)
	}

	if created.CheckIntervalSeconds != 15 {
		t.Errorf("CheckIntervalSeconds = %d, want %d", created.CheckIntervalSeconds, 15)
	}
}

func TestRepositoryGetEndpointByID(t *testing.T) {
	repo := setupTestRepository(t)

	ctx := context.Background()

	interval := 15
	created, err := repo.Create(ctx, uniqueTestURL(t), &interval)
	if err != nil {
		t.Fatalf("Error creating record in DB, %v", err)
	}

	t.Cleanup(func() {
		_ = repo.DeleteEndpoint(ctx, created.ID)
	})

	tests := []struct {
		name    string
		id      int64
		wantErr error
	}{
		{
			name:    "existing endpoint",
			id:      created.ID,
			wantErr: nil,
		},
		{
			name:    "non-existing endpoint",
			id:      created.ID + 999,
			wantErr: ErrEndpointNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.GetEndpointByID(ctx, tt.id)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}

			if tt.wantErr != nil {
				return
			}

			if got.ID != created.ID {
				t.Errorf("ID = %d, want %d", got.ID, created.ID)
			}
		})
	}
}

func TestRepositoryGetEndpoints(t *testing.T) {
	repo := setupTestRepository(t)

	ctx := context.Background()

	interval := 15
	created, err := repo.Create(ctx, uniqueTestURL(t), &interval)
	if err != nil {
		t.Fatalf("Error creating record in DB, %v", err)
	}

	t.Cleanup(func() {
		_ = repo.DeleteEndpoint(ctx, created.ID)
	})

	got, err := repo.GetEndpoints(ctx, nil)
	if err != nil {
		t.Fatalf("GetEndpoints() error = %v", err)
	}

	found := false

	for _, endpoint := range got {
		if endpoint.ID == created.ID {
			found = true

			if endpoint.URL != created.URL {
				t.Errorf("URL = %s, want %s", endpoint.URL, created.URL)
			}

			if endpoint.CheckIntervalSeconds != created.CheckIntervalSeconds {
				t.Errorf(
					"CheckIntervalSeconds = %d, want %d",
					endpoint.CheckIntervalSeconds,
					created.CheckIntervalSeconds,
				)
			}

			break
		}

	}
	if !found {
		t.Errorf("endpoint with ID %d not found in result", created.ID)
	}

	active := true

	filtered, err := repo.GetEndpoints(ctx, &active)
	if err != nil {
		t.Fatalf("GetEndpoints(active=true) error = %v", err)
	}

	for _, endpoint := range filtered {
		if !endpoint.IsActive {
			t.Errorf(
				"found inactive endpoint with ID %d in active results",
				endpoint.ID,
			)
		}
	}
}

func TestRepositoryUpdateEndpoint(t *testing.T) {
	repo := setupTestRepository(t)

	ctx := context.Background()

	interval := 15
	created, err := repo.Create(ctx, uniqueTestURL(t), &interval)
	if err != nil {
		t.Fatalf("Error creating record in DB, %v", err)
	}

	t.Cleanup(func() {
		_ = repo.DeleteEndpoint(ctx, created.ID)
	})

	updateInterval := 20
	active := false
	got, err := repo.UpdateEndpoint(ctx, created.ID, &updateInterval, &active)
	if err != nil {
		t.Fatalf("Error updating record id DB: %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("ID = %d, want %d", got.ID, created.ID)
	}

	if got.URL != created.URL {
		t.Errorf("URL = %q, want %q", got.URL, created.URL)
	}

	if got.CheckIntervalSeconds != 20 {
		t.Errorf("CheckIntervalSeconds = %d, want 20", got.CheckIntervalSeconds)
	}

	if got.IsActive {
		t.Errorf("IsActive = %v, want false", got.IsActive)
	}

	active = true

	got, err = repo.UpdateEndpoint(ctx, created.ID, nil, &active)
	if err != nil {
		t.Fatalf("UpdateEndpoint() error = %v", err)
	}

	if got.CheckIntervalSeconds != 20 {
		t.Errorf(
			"CheckIntervalSeconds = %d, want 20",
			got.CheckIntervalSeconds,
		)
	}

	if !got.IsActive {
		t.Errorf("IsActive = %v, want true", got.IsActive)
	}
}

func TestRepositoryDeleteEndpoint(t *testing.T) {
	repo := setupTestRepository(t)

	ctx := context.Background()

	interval := 15
	created, err := repo.Create(ctx, uniqueTestURL(t), &interval)
	if err != nil {
		t.Fatalf("Error creating record in DB, %v", err)
	}

	t.Cleanup(func() {
		if err := repo.DeleteEndpoint(ctx, created.ID); err != nil &&
			!errors.Is(err, ErrEndpointNotFound) {
			t.Errorf("cleanup: %v", err)
		}
	})

	tests := []struct {
		name    string
		id      int64
		wantErr error
	}{
		{
			name:    "existing endpoint",
			id:      created.ID,
			wantErr: nil,
		},
		{
			name:    "non existing endpoint",
			id:      created.ID + 999,
			wantErr: ErrEndpointNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.DeleteEndpoint(ctx, tt.id)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Error deleting endpoint: %v", err)
			}
		})
	}
}
