package transport

import (
	"encoding/json/v2"
	"log/slog"
	"net/http"
	"time"
)

type EndpointResponseDTO struct {
	ID                   int64     `json:"id"`
	URL                  string    `json:"url"`
	CheckIntervalSeconds int       `json:"check_interval_seconds"`
	IsActive             bool      `json:"is_active"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Time  string `json:"time"`
}

type CreateEndpointDTO struct {
	URL                  string `json:"url"`
	CheckIntervalSeconds int    `json:"check_interval_seconds"`
}

type UpdateEndpointDTO struct {
	CheckIntervalSeconds int  `json:"check_interval_seconds"`
	IsActive             bool `json:"is_active"`
}

func respondJSON(w http.ResponseWriter, status int, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_, err = w.Write(body)
	return err
}

func writeError(
	w http.ResponseWriter,
	status int,
	message string,
	logger *slog.Logger,
) {
	if err := respondJSON(w,
		status,
		ErrorResponse{
			Error: message,
			Time:  time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
		logger.Error(
			"failed to write error response",
			"error", err,
			"status", status,
		)
	}
}

func decodeJSON[T any](w http.ResponseWriter, r *http.Request, out *T, logger *slog.Logger) error {
	if r.Body == nil {
		logger.Error(
			"request body is empty",
		)
		writeError(w, http.StatusBadRequest, "request body is empty", logger)
		return http.ErrBodyNotAllowed
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			logger.Error(
				"failed to close request body",
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "failed to close request body", logger)
		}
	}()

	err := json.UnmarshalRead(r.Body, out)
	if err != nil {
		logger.Error(
			"failed to decode JSON body",
			"error", err,
		)
		writeError(w, http.StatusBadRequest, "invalid JSON body", logger)
		return err
	}
	return nil
}
