package reports

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/victor-devv/report-gen/config"
	"github.com/victor-devv/report-gen/store"
)

type ReportBuilder struct {
	config      *config.Config
	logger      *slog.Logger
	reportStore *store.ReportStore
	lozClient   *LozClient
	s3Client    *s3.Client
}

func NewReportBuilder(config *config.Config, logger *slog.Logger, reportStore *store.ReportStore, lozClient *LozClient, s3Client *s3.Client) *ReportBuilder {
	return &ReportBuilder{
		config,
		logger,
		reportStore,
		lozClient,
		s3Client,
	}
}

func (b *ReportBuilder) Build(ctx context.Context, userId, reportId uuid.UUID) (report *store.Report, err error) {
	report, err = b.reportStore.ByPrimaryKey(ctx, reportId, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get report %s for user %s: %w", reportId, userId, err)
	}

	if report.StartedAt != nil {
		return report, nil
	}

	defer func() {
		if err != nil {
			now := time.Now()
			errMsg := err.Error()
			report.FailedAt = &now
			report.ErrorMessage = &errMsg
			if _, updateErr := b.reportStore.Update(ctx, report); updateErr != nil {
				b.logger.Error("failed to update report", "error", err.Error())
			}
		}
	}()

	now := time.Now()
	report.StartedAt = &now
	report.CompletedAt = nil
	report.FailedAt = nil
	report.ErrorMessage = nil
	report.OutputFilePath = nil
	report.DownloadUrl = nil
	report.DownloadUrlExpiresAt = nil
	report, err = b.reportStore.Update(ctx, report)
	if err != nil {
		return nil, fmt.Errorf("failed to update report %s for user %s: %w", reportId, userId, err)
	}

	//we assume that report type will always be monsters for this project
	resp, err := b.lozClient.GetMonsters()
	if err != nil {
		return nil, fmt.Errorf("failed to get monsters data: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no monsters data found")
	}

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)

	csvWriter := csv.NewWriter(gzipWriter)

	header := []string{"id", "name", "category", "description", "image", "common_locations", "drops", "dlc"}
	if err := csvWriter.Write(header); err != nil {
		return nil, fmt.Errorf("failed to write header to csv: %w", err)
	}

	for _, monster := range resp.Data {
		row := []string{
			fmt.Sprintf("%d", monster.Id),
			monster.Name,
			monster.Category,
			monster.Description,
			monster.Image,
			strings.Join(monster.CommonLocations, ", "),
			strings.Join(monster.Drops, ", "),
			strconv.FormatBool(monster.Dlc),
		}

		if err := csvWriter.Write(row); err != nil {
			return nil, fmt.Errorf("failed to write row to csv: %w", err)
		}

		if err := csvWriter.Error(); err != nil {
			return nil, fmt.Errorf("failed to write row to csv: %w", err)
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return nil, fmt.Errorf("failed to flush csv writer: %w", err)
	}

	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Upload the CSV file to S3
	key := "/users/" + userId.String() + "/reports/" + reportId.String() + "csv.gz"
	_, err = b.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Key:    aws.String(key),
		Bucket: aws.String(b.config.S3Bucket),
		Body:   bytes.NewReader(buffer.Bytes()),
	})

	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to upload report to %s: %w", key, err)
	}

	now = time.Now()
	report.OutputFilePath = &key
	report.CompletedAt = &now
	report, err = b.reportStore.Update(ctx, report)
	if err != nil {
		return nil, fmt.Errorf("failed to update report %s for user %s: %w", reportId, userId, err)
	}

	b.logger.Info("report generated successfully", "report_id", reportId.String(), "user_id", userId.String(), "path", key)

	return report, nil
}
