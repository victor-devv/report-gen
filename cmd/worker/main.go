package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/victor-devv/report-gen/config"
	"github.com/victor-devv/report-gen/reports"
	"github.com/victor-devv/report-gen/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
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

	awsConf, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	s3Client := s3.NewFromConfig(awsConf, func(options *s3.Options) {
		if conf.Env != config.Env_Prod {
			options.BaseEndpoint = aws.String(conf.S3Endpoint)
			options.UsePathStyle = true
		}
	})

	sqsClient := sqs.NewFromConfig(awsConf, func(options *sqs.Options) {
		if conf.Env != config.Env_Prod {
			options.BaseEndpoint = aws.String(conf.SqsEndpoint)
		}
	})

	lozClient := reports.NewLozClient(&http.Client{Timeout: time.Second * 10})

	builder := reports.NewReportBuilder(conf, logger, store.Reports, lozClient, s3Client)

	maxConcurrency := 2
	worker := reports.NewWorker(conf, logger, builder, sqsClient, maxConcurrency)

	if err := worker.Start(ctx); err != nil {
		return err
	}

	return nil
}
