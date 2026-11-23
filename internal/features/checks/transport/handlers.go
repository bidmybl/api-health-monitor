package transport

import (
	"context"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	checkRepo "github.com/bidmybl/api-health-monitor/internal/features/checks/repository"
	endpointRepo "github.com/bidmybl/api-health-monitor/internal/features/endpoints/repository"
)

const (
	defaultHistoryLimit = 20
	maxHistoryLimit     = 100
)

type CheckService interface {
	CheckEndpoint(
		ctx context.Context,
		endpointID int64,
	) (checkRepo.CheckResult, error)

	GetCheckHistory(
		ctx context.Context,
		endpointID int64,
		limit int,
	) ([]checkRepo.CheckResult, error)
}

type Handler struct {
	service CheckService
	logger  *slog.Logger
}

type CheckResultResponse struct {
	ID             int64     `json:"id"`
	EndpointID     int64     `json:"endpoint_id"`
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    time.Time `json:"completed_at"`
	HTTPStatusCode *int      `json:"http_status_code"`
	ResponseTimeMS int       `json:"response_time_ms"`
	IsSuccess      bool      `json:"is_success"`
	ErrorMessage   *string   `json:"error_message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Time  string `json:"time"`
}

func NewHandler(service CheckService, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) CheckEndpoint(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.writeError(w, http.StatusBadRequest, "invalid endpoint ID")
		return
	}

	result, err := h.service.CheckEndpoint(r.Context(), id)
	if err != nil {
		if errors.Is(err, endpointRepo.ErrEndpointNotFound) {
			h.writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}

		h.logger.Error(
			"failed to check endpoint",
			"error", err,
			"endpoint_id", id,
		)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response := toCheckResultResponse(result)

	if err := writeJSON(w, http.StatusOK, response); err != nil {
		h.logger.Error(
			"failed to write check result response",
			"error", err,
			"endpoint_id", id,
		)
	}
}

func (h *Handler) GetCheckHistory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		h.writeError(w, http.StatusBadRequest, "invalid endpoint ID")
		return
	}

	limit := defaultHistoryLimit

	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		limit, err = strconv.Atoi(rawLimit)
		if err != nil || limit < 1 || limit > maxHistoryLimit {
			h.writeError(
				w,
				http.StatusBadRequest,
				"limit must be between 1 and 100",
			)
			return
		}
	}

	results, err := h.service.GetCheckHistory(r.Context(), id, limit)
	if err != nil {
		if errors.Is(err, endpointRepo.ErrEndpointNotFound) {
			h.writeError(w, http.StatusNotFound, "endpoint not found")
			return
		}

		h.logger.Error(
			"failed to get check history",
			"error", err,
			"endpoint_id", id,
		)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response := make([]CheckResultResponse, 0, len(results))
	for _, result := range results {
		response = append(response, toCheckResultResponse(result))
	}

	if err := writeJSON(w, http.StatusOK, response); err != nil {
		h.logger.Error(
			"failed to write check history response",
			"error", err,
			"endpoint_id", id,
		)
	}
}

func toCheckResultResponse(result checkRepo.CheckResult) CheckResultResponse {
	return CheckResultResponse{
		ID:             result.ID,
		EndpointID:     result.EndpointID,
		StartedAt:      result.StartedAt,
		CompletedAt:    result.CompletedAt,
		HTTPStatusCode: result.HTTPStatusCode,
		ResponseTimeMS: result.ResponseTimeMS,
		IsSuccess:      result.IsSuccess,
		ErrorMessage:   result.ErrorMessage,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) error {
	body, err := json.Marshal(value)
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
	err := writeJSON(w, status, ErrorResponse{
		Error: message,
		Time:  time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		h.logger.Error("failed to write error response", "error", err)
	}
}
