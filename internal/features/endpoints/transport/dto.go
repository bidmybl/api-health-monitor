package transport

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/bidmybl/api-health-monitor/internal/features/endpoints/repository"
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
	CheckIntervalSeconds *int   `json:"check_interval_seconds"`
}

type UpdateEndpointDTO struct {
	CheckIntervalSeconds *int  `json:"check_interval_seconds"`
	IsActive             *bool `json:"is_active"`
}

func toEndpointResponse(endpoint repository.Endpoint) EndpointResponseDTO {
	return EndpointResponseDTO{
		ID:                   endpoint.ID,
		URL:                  endpoint.URL,
		CheckIntervalSeconds: endpoint.CheckIntervalSeconds,
		IsActive:             endpoint.IsActive,
		CreatedAt:            endpoint.CreatedAt,
		UpdatedAt:            endpoint.UpdatedAt,
	}
}

func toEndpointResponses(endpoints []repository.Endpoint) []EndpointResponseDTO {
	responses := make([]EndpointResponseDTO, 0, len(endpoints))

	for _, endpoint := range endpoints {
		responses = append(responses, toEndpointResponse(endpoint))
	}

	return responses
}

func validateURL(rawURL string) bool {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false
	}

	if u.Host == "" {
		return false
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	return true
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

func (h *Handler) writeError(
	w http.ResponseWriter,
	status int,
	message string,
) {
	if err := respondJSON(w,
		status,
		ErrorResponse{
			Error: message,
			Time:  time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
		h.logger.Error(
			"failed to write error response",
			"error", err,
			"status", status,
		)
	}
}

func decodeJSON[T any](w http.ResponseWriter, r *http.Request, out *T) error {
	if r.Body == nil {
		return errors.New("request body is missing")
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	return json.UnmarshalRead(r.Body, out)
}
