package main

import (
	"context"
	"github.com/victor-devv/report-gen/config"
	"github.com/victor-devv/report-gen/server"
	"github.com/victor-devv/report-gen/store"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// close context on graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	conf, err := config.New()
	if err != nil {
		return err
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, nil)
	logger := slog.New(jsonHandler)

	db, err := store.NewPostgresDb(conf)
	if err != nil {
		return err
	}
	store := store.New(db)

	server := server.New(conf, logger, store)
	if err := server.Start(ctx); err != nil {
		return err
	}
	return nil
}
