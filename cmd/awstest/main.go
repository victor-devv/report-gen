package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/victor-devv/report-gen/config"
)

func main() {
	ctx := context.Background()
	sdkConfig, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		fmt.Println("Couldn't load default configuration. Have you set up your AWS account?")
		fmt.Println(err)
		return
	}

	conf, err := config.New()
	if err != nil {
		log.Fatal(err)
	}

	s3Client := s3.NewFromConfig(sdkConfig, func(options *s3.Options) {
		if conf.Env != config.Env_Prod {
			options.BaseEndpoint = aws.String(conf.S3Endpoint)
			options.UsePathStyle = true
		}
	})

	out, err := s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		log.Fatal(err)
	}

	for _, bucket := range out.Buckets {
		fmt.Println(*bucket.Name)
	}

	sqsClient := sqs.NewFromConfig(sdkConfig, func(options *sqs.Options) {
		if conf.Env != config.Env_Prod {
			options.BaseEndpoint = aws.String(conf.SqsEndpoint)
		}
	})

	sqsOut, err := sqsClient.ListQueues(ctx, &sqs.ListQueuesInput{})
	if err != nil {
		log.Fatal(err)
	}

	for _, queue := range sqsOut.QueueUrls {
		fmt.Println(queue)
	}
}
