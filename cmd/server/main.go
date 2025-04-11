package main

import (
	"asyncapi/config"
	"asyncapi/server"
	"context"
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

	server := server.New(conf, logger)
	if err := server.Start(ctx); err != nil {
		return err
	}
	return nil
}
