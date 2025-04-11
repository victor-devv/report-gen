package server

import (
	"asyncapi/config"
	"asyncapi/store"
	"context"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	config *config.Config
	logger *slog.Logger
	store  *store.Store
}

func New(config *config.Config, logger *slog.Logger, store *store.Store) *Server {
	return &Server{
		config: config,
		logger: logger,
		store:  store,
	}
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", s.ping)
	mux.HandleFunc("POST /auth/signup", s.signupHandler())

	middleware := NewLoggerMiddleware(s.logger)

	server := &http.Server{
		Addr:    net.JoinHostPort(s.config.ServerHost, s.config.ServerPort),
		Handler: middleware(mux),
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
