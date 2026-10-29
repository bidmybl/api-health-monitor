package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrEndpointNotFound = errors.New("endpoint not found")
var ErrEndpointAlreadyExists = errors.New("endpoint already exists")

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
	url string,
	checkIntervalSeconds *int,
) (Endpoint, error) {
	var (
		rows pgx.Rows
		err  error
	)

	if checkIntervalSeconds == nil {
		rows, err = r.pool.Query(ctx, `
        INSERT INTO monitored_endpoints (url)
        VALUES ($1)
        RETURNING id, url, check_interval_seconds, is_active, created_at, updated_at;
    `, url)
	} else {
		rows, err = r.pool.Query(ctx, `
        INSERT INTO monitored_endpoints (url, check_interval_seconds)
        VALUES ($1, $2)
        RETURNING id, url, check_interval_seconds, is_active, created_at, updated_at;
    `, url, *checkIntervalSeconds)
	}

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Endpoint{}, ErrEndpointAlreadyExists
		}
		return Endpoint{}, fmt.Errorf("insert monitored endpoint: %w", err)
	}

	endpoint, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Endpoint])
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Endpoint{}, ErrEndpointAlreadyExists
		}

		return Endpoint{}, fmt.Errorf("collect created endpoint: %w", err)
	}

	return endpoint, nil
}

func (r *Repository) GetEndpointByID(ctx context.Context, id int64) (Endpoint, error) {
	query := `
	SELECT id, url, check_interval_seconds, is_active, created_at, updated_at
	FROM monitored_endpoints
	WHERE id=$1;
	`
	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return Endpoint{}, fmt.Errorf("get endpoint by ID: %w", err)
	}

	endpoint, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Endpoint])
	if errors.Is(err, pgx.ErrNoRows) {
		return Endpoint{}, ErrEndpointNotFound
	}
	if err != nil {
		return Endpoint{}, fmt.Errorf("collect endpoint by ID: %w", err)
	}

	return endpoint, nil
}

func (r *Repository) GetEndpoints(ctx context.Context, isActive *bool) ([]Endpoint, error) {
	query := `
	SELECT id, url, check_interval_seconds, is_active, created_at, updated_at 
	FROM monitored_endpoints
	WHERE $1::boolean IS NULL OR is_active = $1
	ORDER BY id;
	`

	rows, err := r.pool.Query(ctx, query, isActive)
	if err != nil {
		return nil, fmt.Errorf("get endpoints: %w", err)
	}

	endpoints, err := pgx.CollectRows(rows, pgx.RowToStructByName[Endpoint])
	if err != nil {
		return nil, fmt.Errorf("collect endpoints: %w", err)
	}

	return endpoints, err
}

func (r *Repository) UpdateEndpoint(ctx context.Context, id int64, checkIntervalSeconds *int, isActive *bool) (Endpoint, error) {
	query := `
	UPDATE 	monitored_endpoints
	SET 
		check_interval_seconds = COALESCE($1, check_interval_seconds), 
		is_active = COALESCE($2, is_active), 
		updated_at = now()
	WHERE id = $3
	RETURNING id, url, check_interval_seconds, is_active, created_at, updated_at;
	`

	rows, err := r.pool.Query(ctx, query, checkIntervalSeconds, isActive, id)
	if err != nil {
		return Endpoint{}, fmt.Errorf("update endpoint: %w", err)
	}

	endpoint, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Endpoint])
	if errors.Is(err, pgx.ErrNoRows) {
		return Endpoint{}, ErrEndpointNotFound
	}
	if err != nil {
		return Endpoint{}, fmt.Errorf("collect updated endpoint: %w", err)
	}

	return endpoint, nil
}

func (r *Repository) DeleteEndpoint(ctx context.Context, id int64) error {
	query := `
	DELETE FROM monitored_endpoints
	WHERE id = $1;
	`

	cmdTag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrEndpointNotFound
	}

	return nil
}
