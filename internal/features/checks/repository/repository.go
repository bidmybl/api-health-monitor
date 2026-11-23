package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bidmybl/api-health-monitor/internal/features/checks"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

func (r *Repository) Create(
	ctx context.Context,
	endpointID int64,
	result checks.Result,
) (CheckResult, error) {
	query := `
		INSERT INTO check_results (
			endpoint_id,
			started_at,
			completed_at,
			http_status_code,
			response_time_ms,
			error_message
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING
			id,
			endpoint_id,
			started_at,
			completed_at,
			http_status_code,
			response_time_ms,
			is_success,
			error_message;
	`

	var record CheckResult

	err := r.pool.QueryRow(
		ctx,
		query,
		endpointID,
		result.StartedAt,
		result.CompletedAt,
		result.HTTPStatusCode,
		result.ResponseTimeMS,
		result.ErrorMessage,
	).Scan(
		&record.ID,
		&record.EndpointID,
		&record.StartedAt,
		&record.CompletedAt,
		&record.HTTPStatusCode,
		&record.ResponseTimeMS,
		&record.IsSuccess,
		&record.ErrorMessage,
	)
	if err != nil {
		return CheckResult{}, fmt.Errorf("create check result: %w", err)
	}

	return record, nil
}
