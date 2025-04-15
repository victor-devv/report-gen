package server

import (
	"context"
	"github.com/victor-devv/report-gen/config"
	"github.com/victor-devv/report-gen/store"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	config     *config.Config
	logger     *slog.Logger
	store      *store.Store
	jwtManager *JwtManager
}

func New(config *config.Config, logger *slog.Logger, store *store.Store, jwtManager *JwtManager) *Server {
	return &Server{
		config:     config,
		logger:     logger,
		store:      store,
		jwtManager: jwtManager,
	}
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

type Middleware func(http.Handler) http.Handler

// Chain creates a single middleware from multiple middlewares
func Chain(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", s.ping)
	mux.HandleFunc("POST /api/v1/auth/signup", s.signupHandler())
	mux.HandleFunc("POST /api/v1/auth/signin", s.signInHandler())
	mux.HandleFunc("POST /api/v1/auth/token/refresh", s.refreshTokenHandler())

	loggerMiddleware := NewLoggerMiddleware(s.logger)
	authMiddleware := NewAuthMiddleware(s.jwtManager, s.store.Users)

	server := &http.Server{
		Addr: net.JoinHostPort(s.config.ServerHost, s.config.ServerPort),
		Handler: Chain(
			loggerMiddleware,
			authMiddleware,
		)(mux),
	}

	go func() {
		s.logger.Info("starting http server", "port", s.config.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("http server failed to listen and serve", "error", err)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("failed to shut down http server", "error", err)
		}
	}()

	wg.Wait()
	return nil
}
