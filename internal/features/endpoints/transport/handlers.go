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
	Create(ctx context.Context, url string, checkIntervalSeconds *int) (repository.Endpoint, error)
	GetEndpointByID(ctx context.Context, id int64) (repository.Endpoint, error)
	GetEndpoints(ctx context.Context, isActive *bool) ([]repository.Endpoint, error)
	UpdateEndpoint(ctx context.Context, id int64, checkIntervalSeconds *int, isActive *bool) (repository.Endpoint, error)
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
func (h *Handler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	var request CreateEndpointDTO
	if err := decodeJSON(w, r, &request); err != nil {
		h.logger.Warn("invalid JSON body", "error", err)
		h.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if !validateURL(request.URL) {
		h.writeError(w, http.StatusBadRequest, "invalid URL")
		return
	}

	if request.CheckIntervalSeconds != nil && *request.CheckIntervalSeconds <= 0 {
		h.writeError(w, http.StatusBadRequest, "check_interval_seconds must be positive")
		return
	}

	endpoint, err := h.repo.Create(r.Context(), request.URL, request.CheckIntervalSeconds)
	if err != nil {
		if errors.Is(err, repository.ErrEndpointAlreadyExists) {
			h.writeError(
				w,
				http.StatusConflict,
				"endpoint URL already exists",
			)
			return
		}

		h.logger.Error("failed to create endpoint", "error", err)
		h.writeError(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
		return
	}

	if err := respondJSON(w, http.StatusCreated, toEndpointResponse(endpoint)); err != nil {
		h.logger.Error(
			"failed to write endpoint response",
			"error", err,
		)
	}
}

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
		h.writeError(w, http.StatusBadRequest, "invalid endpoint ID")
		return
	}

	endpoint, err := h.repo.GetEndpointByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrEndpointNotFound) {
			h.writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}
		h.logger.Error(
			"failed to get endpoint by ID",
			"error", err,
			"endpoint_id", id,
		)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := respondJSON(w, http.StatusOK, toEndpointResponse(endpoint)); err != nil {
		h.logger.Error(
			"failed to write endpoint response",
			"error", err,
			"endpoint_id", id,
		)
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
	var isActive *bool
	if values, ok := r.URL.Query()["is_active"]; ok {
		if len(values) != 1 ||
			(values[0] != "true" && values[0] != "false") {
			h.writeError(
				w,
				http.StatusBadRequest,
				"is_active must be true or false",
			)
			return
		}
		value := values[0] == "true"
		isActive = &value
	}
	endpoints, err := h.repo.GetEndpoints(r.Context(), isActive)
	if err != nil {
		h.logger.Error("failed to get all endpoints", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := respondJSON(w, http.StatusOK, toEndpointResponses(endpoints)); err != nil {
		h.logger.Error("failed to write endpoints response", "error", err)
	}
}

/* GetAllActiveEndpoints
pattern: /endpoints?is_active=true
method: GET
info: query params

succeed:
  - status code: 200 OK
  - response body: JSON represented found endpoints

failed:
  - status code: 500, ...
  - response body: JSON with error + time
*/

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
func (h *Handler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.logger.Warn(
			"invalid endpoint ID",
			"id", r.PathValue("id"),
		)
		h.writeError(w, http.StatusBadRequest, "invalid endpoint ID")
		return
	}

	var request UpdateEndpointDTO
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeError(
			w,
			http.StatusBadRequest,
			"invalid JSON body",
		)
		return
	}

	if request.CheckIntervalSeconds == nil && request.IsActive == nil {
		h.writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	if request.CheckIntervalSeconds != nil && *request.CheckIntervalSeconds <= 0 {
		h.writeError(w, http.StatusBadRequest, "check interval must be positive")
		return
	}

	endpoint, err := h.repo.UpdateEndpoint(r.Context(), id, request.CheckIntervalSeconds, request.IsActive)
	if err != nil {
		if errors.Is(err, repository.ErrEndpointNotFound) {
			h.logger.Warn(
				"endpoint not found",
				"endpoint_id", id,
			)
			h.writeError(w, http.StatusNotFound, "endpoint not found")
			return
		} else {
			h.logger.Error(
				"failed to update endpoint",
				"error", err,
				"endpoint_id", id,
			)
			h.writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if err := respondJSON(w, http.StatusOK, toEndpointResponse(endpoint)); err != nil {
		h.logger.Error(
			"failed to write endpoint response",
			"error", err,
			"endpoint_id", id,
		)
	}
}

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
		h.writeError(w, http.StatusBadRequest, "invalid endpoint ID")
		return
	}

	if err := h.repo.DeleteEndpoint(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrEndpointNotFound) {
			h.writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}
		h.logger.Error(
			"failed to delete endpoint",
			"error", err,
			"id", id,
		)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
