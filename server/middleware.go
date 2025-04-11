package server

import (
	"log/slog"
	"net/http"
)

func NewLoggerMiddleware(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("HTTP request", "method", r.Method, "path", r.URL.EscapedPath())
			next.ServeHTTP(w, r)
		})
	}
}
