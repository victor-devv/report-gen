package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/victor-devv/report-gen/config"
	"github.com/victor-devv/report-gen/server"
	"github.com/victor-devv/report-gen/store"
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

	jwtManager := server.NewJwtManager(conf)

	sdkConfig, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	sqsClient := sqs.NewFromConfig(sdkConfig, func(options *sqs.Options) {
		if conf.Env != config.Env_Prod {
			options.BaseEndpoint = aws.String(conf.SqsEndpoint)
		}
	})

	db, err := store.NewPostgresDb(conf)
	if err != nil {
		return err
	}
	store := store.New(db)

	server := server.New(conf, logger, store, jwtManager, sqsClient)
	if err := server.Start(ctx); err != nil {
		return err
	}
	return nil
}
