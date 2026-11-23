package repository

import (
	"context"
	"testing"
	"time"
)

func TestRepositoryGetEndpointsDueForCheck(t *testing.T) {
	repo := setupTestRepository(t)
	ctx := context.Background()

	createEndpoint := func(interval int) Endpoint {
		t.Helper()

		endpoint, err := repo.Create(
			ctx,
			uniqueTestURL(t),
			&interval,
		)
		if err != nil {
			t.Fatalf("create endpoint: %v", err)
		}

		t.Cleanup(func() {
			if err := repo.DeleteEndpoint(ctx, endpoint.ID); err != nil &&
				err != ErrEndpointNotFound {
				t.Errorf("cleanup endpoint: %v", err)
			}
		})

		return endpoint
	}

	createCheck := func(endpointID int64, completedAt time.Time) {
		t.Helper()

		_, err := repo.pool.Exec(
			ctx,
			`
				INSERT INTO check_results (
					endpoint_id,
					started_at,
					completed_at,
					http_status_code,
					response_time_ms
				)
				VALUES ($1, $2, $2, 200, 10);
			`,
			endpointID,
			completedAt,
		)
		if err != nil {
			t.Fatalf("create check result: %v", err)
		}
	}

	newEndpoint := createEndpoint(60)

	inactiveEndpoint := createEndpoint(60)
	inactive := false

	_, err := repo.UpdateEndpoint(
		ctx,
		inactiveEndpoint.ID,
		nil,
		&inactive,
	)
	if err != nil {
		t.Fatalf("deactivate endpoint: %v", err)
	}

	recentEndpoint := createEndpoint(3600)
	createCheck(recentEndpoint.ID, time.Now())

	dueEndpoint := createEndpoint(1)
	createCheck(dueEndpoint.ID, time.Now().Add(-2*time.Second))

	got, err := repo.GetEndpointsDueForCheck(ctx)
	if err != nil {
		t.Fatalf("GetEndpointsDueForCheck() error = %v", err)
	}

	found := make(map[int64]bool, len(got))
	for _, endpoint := range got {
		found[endpoint.ID] = true
	}

	tests := []struct {
		name    string
		id      int64
		wantDue bool
	}{
		{
			name:    "endpoint without checks",
			id:      newEndpoint.ID,
			wantDue: true,
		},
		{
			name:    "inactive endpoint",
			id:      inactiveEndpoint.ID,
			wantDue: false,
		},
		{
			name:    "endpoint checked recently",
			id:      recentEndpoint.ID,
			wantDue: false,
		},
		{
			name:    "endpoint whose interval has elapsed",
			id:      dueEndpoint.ID,
			wantDue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if found[tt.id] != tt.wantDue {
				t.Errorf(
					"endpoint found = %v, want %v",
					found[tt.id],
					tt.wantDue,
				)
			}
		})
	}
}
