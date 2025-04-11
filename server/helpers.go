package server

import (
	"encoding/json"
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

type ApiResponse[T any] struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Data    *T     `json:"data"`
	Code    int    `json:"code,omitempty"`
}

func ErrorResponse[T any](w http.ResponseWriter, status Status, message string, httpStatusCode int, data T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)

	errorResponse := ApiResponse[T]{
		Status:  status.String(),
		Message: message,
		Code:    httpStatusCode,
		Data:    &data,
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		// Fall back to plain text error if JSON encoding fails
		http.Error(w, err.Error(), httpStatusCode)
	}
}

func SuccessResponse[T any](w http.ResponseWriter, httpStatusCode int, message string, data T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatusCode)

	response := ApiResponse[T]{
		Status:  Success.String(),
		Message: message,
		Data:    &data,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Fall back to error response if encoding fails
		ErrorResponse(w, Error, "an error occurred: "+err.Error(), http.StatusInternalServerError, (*struct{})(nil))
		return
	}
}
