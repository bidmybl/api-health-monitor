package repository

import "time"

type CheckResult struct {
	ID             int64     `db:"id"`
	EndpointID     int64     `db:"endpoint_id"`
	StartedAt      time.Time `db:"started_at"`
	CompletedAt    time.Time `db:"completed_at"`
	HTTPStatusCode *int      `db:"http_status_code"`
	ResponseTimeMS int       `db:"response_time_ms"`
	IsSuccess      bool      `db:"is_success"`
	ErrorMessage   *string   `db:"error_message"`
}
