package repository

import "time"

type Endpoint struct {
	ID                   int64     `db:"id"`
	URL                  string    `db:"url"`
	CheckIntervalSeconds int       `db:"check_interval_seconds"`
	IsActive             bool      `db:"is_active"`
	CreatedAt            time.Time `db:"created_at"`
	UpdatedAt            time.Time `db:"updated_at"`
}
