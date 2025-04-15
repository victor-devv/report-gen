package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/victor-devv/report-gen/store"
)

func sanitizeHeaders(headers http.Header) map[string]string {
	safeHeaders := make(map[string]string)

	for k, v := range headers {
		switch strings.ToLower(k) {
		case "authorization", "cookie", "set-cookie":
			safeHeaders[k] = "[REDACTED]"
		default:
			// Join multiple header values with comma
			safeHeaders[k] = strings.Join(v, ", ")
		}
	}

	return safeHeaders
}

type responseWriter struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
		body:           &bytes.Buffer{},
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

type reqIdCtxKey struct{}

func ContextWithReqId(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, reqIdCtxKey{}, requestID)
}

func NewLoggerMiddleware(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := strings.ReplaceAll(uuid.New().String(), "-", "")

			var requestBody []byte
			if r.Body != nil {
				requestBody, _ = io.ReadAll(r.Body)
				// Restore the body
				r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			}

			wrappedWriter := newResponseWriter(w)

			startTime := time.Now()

			requestBodyStr := string(requestBody)

			logger.Info("HTTP request",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.EscapedPath(),
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
				"headers", sanitizeHeaders(r.Header),
				"body", requestBodyStr,
			)
			next.ServeHTTP(wrappedWriter, r.WithContext(ContextWithReqId(r.Context(), requestID)))

			duration := time.Since(startTime)

			responseBodyStr := wrappedWriter.body.String()

			logger.Info("HTTP response",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.EscapedPath(),
				"status", wrappedWriter.status,
				"duration_ms", duration.Milliseconds(),
				"duration", duration.String(),
				"body", responseBodyStr,
			)
		})
	}
}

type userCtxKey struct{}

func ContextWithUser(ctx context.Context, user *store.User) context.Context {
	return context.WithValue(ctx, userCtxKey{}, user)
}

func NewAuthMiddleware(jwtManager *JwtManager, userStore *store.UserStore) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix((r.URL.EscapedPath()), "/api/v1/auth") {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			var access_token string

			if parts := strings.Split(authHeader, "Bearer "); len(parts) == 2 {
				access_token = parts[1]
			}

			if access_token == "" {
				errorResponse(w, Error, "Unauthorized!", http.StatusUnauthorized, (*struct{})(nil))
				return
			}

			parsedToken, err := jwtManager.Parse(access_token)
			if err != nil {
				slog.Error("failed to parse token", "error", err)
				errorResponse(w, Error, "Unauthorized!", http.StatusUnauthorized, (*struct{})(nil))
				return
			}

			if !jwtManager.IsAccessToken(parsedToken) {
				errorResponse(w, Error, "Unauthorized!", http.StatusUnauthorized, (*struct{})(nil))
				return
			}

			userIdStr, err := parsedToken.Claims.GetSubject()
			if err != nil {
				slog.Error("failed to extract token subject claim", "error", err)
				errorResponse(w, Error, "Unauthorized!", http.StatusUnauthorized, (*struct{})(nil))
				return
			}

			userId, err := uuid.Parse(userIdStr)
			if err != nil {
				slog.Error("token subject is not a valid uuid", "error", err)
				errorResponse(w, Error, "Unauthorized!", http.StatusUnauthorized, (*struct{})(nil))
				return
			}

			// TODO: cache this layer
			user, err := userStore.ById(r.Context(), userId)
			if err != nil {
				slog.Error("failed to get user", "error", err)
				errorResponse(w, Error, "Unauthorized!", http.StatusUnauthorized, (*struct{})(nil))
				return
			}

			next.ServeHTTP(w, r.WithContext(ContextWithUser(r.Context(), user)))
		})
	}
}
