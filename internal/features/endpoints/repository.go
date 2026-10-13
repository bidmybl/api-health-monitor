package endpoints

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	url string,
	checkIntervalSeconds int,
) (Endpoint, error) {
	query := `
	INSERT INTO monitored_endpoints (url, check_interval_seconds)
	VALUES ($1, $2)
	RETURNING id, url, check_interval_seconds, is_active, created_at, updated_at;
	`

	rows, err := r.pool.Query(ctx, query, url, checkIntervalSeconds)
	if err != nil {
		return Endpoint{}, err
	}

	endpoint, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Endpoint])

	return endpoint, err
}

func (r *Repository) GetEndpointByID(ctx context.Context, id int64) (Endpoint, error) {
	query := `
	SELECT id, url, check_interval_seconds, is_active, created_at, updated_at
	FROM monitored_endpoints
	WHERE id=$1;
	`
	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return Endpoint{}, err
	}

	endpoint, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Endpoint])
	if errors.Is(err, pgx.ErrNoRows) {
		return Endpoint{}, ErrEndpointNotFound
	}

	return endpoint, err // посмотрев вызываемые функции понял, что вернется либо пустая структура и ошибка, либо заполненная структура и nil
}

func (r *Repository) GetEndpoints(ctx context.Context) ([]Endpoint, error) {
	query := `
	SELECT id, url, check_interval_seconds, is_active, created_at, updated_at 
	FROM monitored_endpoints 
	ORDER BY id;
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	endpoints, err := pgx.CollectRows(rows, pgx.RowToStructByName[Endpoint])

	return endpoints, err
}

func (r *Repository) UpdateEndpoint(ctx context.Context, id int64, checkIntervalSeconds int, isActive bool) (Endpoint, error) {
	query := `
	UPDATE 	monitored_endpoints
	SET check_interval_seconds = $1, is_active = $2, updated_at = now()
	WHERE id = $3
	RETURNING id, url, check_interval_seconds, is_active, created_at, updated_at;
	`

	rows, err := r.pool.Query(ctx, query, checkIntervalSeconds, isActive, id)
	if err != nil {
		return Endpoint{}, err
	}

	endpoint, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Endpoint])
	if errors.Is(err, pgx.ErrNoRows) {
		return Endpoint{}, ErrEndpointNotFound
	}

	return endpoint, err
}

func (r *Repository) DeleteEndpoint(ctx context.Context, id int64) error {
	query := `
	DELETE FROM monitored_endpoints
	WHERE id = $1;
	`

	cmdTag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return ErrEndpointNotFound
	}

	return nil
}
