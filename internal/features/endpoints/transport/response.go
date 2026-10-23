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

// Вот тут стоит большой вопрос нужен ли еще и здесь логгер и че как это лучше всего вообще обрабатывать
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

// Крч, я так прикинул и понял, что будет куда удобнее записывать логи прямо в функциях
// Из плюсов - это позволит писать меньше шаблонного кода
// Из минусов - нужно скорее всего возвращать типа ok bool, чтобы убеждаться, что все проходит хорошо
// (но это не точно, стоит обдумать)
// Обудмал - не нужно
// P.S.: возможно все-таки стоит сделать эту функцию методом хэндлера
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
