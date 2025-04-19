package reports

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/victor-devv/report-gen/config"
)

type Worker struct {
	config      *config.Config
	logger      *slog.Logger
	builder     *ReportBuilder
	sqsClient   *sqs.Client
	channel     chan types.Message
	concurrency int
}

func NewWorker(config *config.Config, logger *slog.Logger, builder *ReportBuilder, sqsClient *sqs.Client, maxConcurrency int) *Worker {
	return &Worker{
		config:      config,
		logger:      logger,
		builder:     builder,
		sqsClient:   sqsClient,
		channel:     make(chan types.Message, maxConcurrency),
		concurrency: maxConcurrency,
	}
}

func (w *Worker) Start(ctx context.Context) error {
	queueUrlOutput, err := w.sqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(w.config.SqsQueue),
	})
	if err != nil {
		return fmt.Errorf("failed to get url for queue %s: %w", w.config.SqsQueue, err)
	}

	w.logger.Info("starting worker", "queue", w.config.SqsQueue, "queue_url", queueUrlOutput.QueueUrl)

	// SET UP CONSUMERS
	// create go routines that matches max concurrency
	for i := 0; i < w.concurrency; i++ {
		go func(id int) {
			for {
				select {
				case <-ctx.Done():
					w.logger.Info("stopping worker", "goroutine_id", id, "error", ctx.Err())
				case message := <-w.channel:
					if err := w.processMessage(ctx, message, queueUrlOutput.QueueUrl); err != nil {
						w.logger.Error("failed to process message", "error", err, "goroutine_id", id)
					}
				}
			}
		}(i)
	}

	// SETUP PRODUCER
	for {
		output, err := w.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            queueUrlOutput.QueueUrl,
			MaxNumberOfMessages: int32(w.concurrency + 1),
		})
		if err != nil {
			w.logger.Error("failed to receive message", "error", err)
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}

		if len(output.Messages) == 0 {
			continue
		}

		for _, message := range output.Messages {
			w.channel <- message
		}
	}
}

func (w *Worker) processMessage(ctx context.Context, message types.Message, queueUrl *string) error {
	if message.Body == nil || *message.Body == "" {
		w.logger.Warn("message body is empty", "message_id", *message.MessageId)
		return nil
	}

	var msg SqsMessage
	if err := json.Unmarshal([]byte(*message.Body), &msg); err != nil {
		w.logger.Warn("message body is invalid", "message_id", *message.MessageId, "body", *message.Body)
		return nil
	}

	// Pass to report builder
	// Handle max processing time for the report (10 secs)
	builderCtx, builderCancel := context.WithTimeout(ctx, time.Second*10)
	defer builderCancel()
	_, err := w.builder.Build(builderCtx, msg.UserId, msg.ReportId)
	if err != nil {
		return fmt.Errorf("failed to build report: %w", err)
	}

	// remove message from queue
	if _, err := w.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      queueUrl,
		ReceiptHandle: message.ReceiptHandle,
	}); err != nil {
		return fmt.Errorf("failed to delete message %s: %w", *message.MessageId, err)
	}

	return nil
}
