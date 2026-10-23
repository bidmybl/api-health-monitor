package transport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bidmybl/api-health-monitor/internal/features/endpoints/repository"
)

type Handler struct {
	repo   EndpointRepository
	logger *slog.Logger
}

type EndpointRepository interface {
	Create(ctx context.Context, url string, checkIntervalSeconds int) (repository.Endpoint, error)
	GetEndpointByID(ctx context.Context, id int64) (repository.Endpoint, error)
	GetEndpoints(ctx context.Context) ([]repository.Endpoint, error)
	GetActiveEndpoints(ctx context.Context) ([]repository.Endpoint, error)
	UpdateEndpoint(ctx context.Context, id int64, checkIntervalSeconds int, isActive bool) (repository.Endpoint, error)
	DeleteEndpoint(ctx context.Context, id int64) error
}

func NewHandler(repo EndpointRepository, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		repo:   repo,
		logger: logger,
	}
}

/*
pattern: /endpoints
method: POST
info: JSON in HTTP request body

succeed:
  - status code: 201 Created
  - response body: JSON represent created endpoint

failed:
  - status code: 400, 409, 500, ...
  - response body: JSON with error + time
*/
func (h *Handler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {}

/*
pattern: /endpoints/{id}
method: GET
info: pattern

succeed:
  - status code: 200 OK
  - response body: JSON represented found endpoint

failed:
  - status code: 400, 404, 500, ...
  - response body: JSON with error + time
*/
func (h *Handler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.logger.Warn(
			"invalid endpoint ID",
			"id", r.PathValue("id"),
		)
		writeError(w, http.StatusBadRequest, "invalid endpoint ID", h.logger)
		return
	}

	endpoint, err := h.repo.GetEndpointByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrEndpointNotFound) {
			h.logger.Warn(
				"endpoint not found",
				"endpoint_id", id,
			)
			writeError(w, http.StatusNotFound, "endpoint not found", h.logger)
		} else {
			h.logger.Error(
				"failed to get endpoint by ID",
				"error", err,
				"endpoint_id", id,
			)
			writeError(w, http.StatusInternalServerError, "internal server error", h.logger)
		}
	}

	if err := respondJSON(w, http.StatusOK, endpoint); err != nil {
		h.logger.Error(
			"failed to write endpoint response",
			"error", err,
			"endpoint_id", id,
		)
		return
	}
}

/*
pattern: /endpoints
method: GET
info: -

succeed:
  - status code: 200 OK
  - response body: JSON represented found endpoints

failed:
  - status code: 500, ...
  - response body: JSON with error + time
*/
func (h *Handler) GetAllEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := h.repo.GetEndpoints(r.Context())
	if err != nil {
		h.logger.Error(
			"failed to get all endpoints",
			"error", err,
		)
		writeError(w, http.StatusInternalServerError, "internal server error", h.logger)
		return
	}

	if err := respondJSON(w, http.StatusOK, endpoints); err != nil {
		h.logger.Error(
			"failed to write endpoints response",
			"error", err,
		)
	}
}

/*
pattern: /endpoints?is_active=true
method: GET
info: query params

succeed:
  - status code: 200 OK
  - response body: JSON represented found endpoints

failed:
  - status code: 400, 500, ...
  - response body: JSON with error + time
*/
func (h *Handler) GetAllActiveEndpoints(w http.ResponseWriter, r *http.Request) {
	isActive := r.URL.Query().Get("is_active")
	if isActive == "true" {
		endpoints, err := h.repo.GetActiveEndpoints(r.Context())
		if err != nil {
			h.logger.Error(
				"failed to get all endpoints",
				"error", err,
			)
			writeError(w, http.StatusInternalServerError, "internal server error", h.logger)
			return
		}

		if err := respondJSON(w, http.StatusOK, endpoints); err != nil {
			h.logger.Error(
				"failed to write endpoints response",
				"error", err,
			)
		}
	} else {
		h.GetAllEndpoints(w, r)
	}
}

/*
pattern: /endpoints/{id}
method: PATCH
info: pattern + JSON in request body

succeed:
  - status code: 200 OK
  - response body: JSON represented updated endpoint

failed:
  - status code: 400, 409, 500, ...
  - response body: JSON with error + time
*/
func (h *Handler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {}

/*
pattern: /endpoints/{id}
method: DELETE
info: pattern

succeed:
  - status code: 204 No Content
  - response body: -

failed:
  - status code: 400, 404, 500, ...
  - response body: JSON with error + time
*/
func (h *Handler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.logger.Warn(
			"invalid endpoint ID",
			"id", r.PathValue("id"),
		)
		writeError(w, http.StatusBadRequest, "invalid endpoint ID", h.logger)
		return
	}

	if err := h.repo.DeleteEndpoint(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrEndpointNotFound) {
			h.logger.Warn(
				"endpoint not found",
				"id", id,
			)
			writeError(w, http.StatusNotFound, "endpoint not found", h.logger)
		} else {
			h.logger.Error(
				"failed to delete endpoint",
				"error", err,
				"id", id,
			)
			writeError(w, http.StatusInternalServerError, "internal server error", h.logger)
		}
	}
}
