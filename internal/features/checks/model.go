package checks

import "time"

type Result struct {
	HTTPStatusCode *int
	ResponseTimeMS int
	ErrorMessage   *string
	StartedAt      time.Time
	CompletedAt    time.Time
}
