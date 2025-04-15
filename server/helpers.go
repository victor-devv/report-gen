package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

type Status int

const (
	Success Status = iota
	Error
	Fail
)

func (s Status) String() string {
	return [...]string{"success", "error", "fail"}[s]
}

type ErrWithStatus struct {
	status int
	err    error
}

func (e *ErrWithStatus) Error() string {
	return e.err.Error()
}

func NewErrWithStatus(err error, status int) *ErrWithStatus {
	return &ErrWithStatus{err: err, status: status}
}

type ApiResponse[T any] struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Data    *T     `json:"data"`
	Code    int    `json:"code,omitempty"`
}

func encode[T any](v ApiResponse[T], httpStatusCode int, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpStatusCode)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("error encoding response", "error", err, "status", httpStatusCode)
		http.Error(w, err.Error(), httpStatusCode)
	}
}

type Validator interface {
	Validate() error
}

func decode[T Validator](r *http.Request) (T, error) {
	var t T
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		return t, fmt.Errorf("error decoding request body: %w", err)
	}

	if err := t.Validate(); err != nil {
		return t, err
	}
	return t, nil
}

func errorResponse[T any](w http.ResponseWriter, status Status, message string, httpStatusCode int, data T) {
	errorResponse := ApiResponse[T]{
		Status:  status.String(),
		Message: message,
		Code:    httpStatusCode,
		Data:    &data,
	}

	encode(errorResponse, httpStatusCode, w)
}

func successResponse[T any](w http.ResponseWriter, httpStatusCode int, message string, data T) {
	response := ApiResponse[T]{
		Status:  Success.String(),
		Message: message,
		Data:    &data,
	}

	encode(response, httpStatusCode, w)
}

func handleWithError(f func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			status := http.StatusInternalServerError
			msg := http.StatusText(status)

			if e, ok := err.(*ErrWithStatus); ok {
				status = e.status
				msg = http.StatusText(status)

				if status == http.StatusBadRequest || status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusConflict {
					msg = e.err.Error()
				}
			}
			slog.Error("error executing handler", "error", err, "status", status, "message", msg)
			errorResponse(w, Error, err.Error(), status, (*struct{})(nil))
		}
	}
}
