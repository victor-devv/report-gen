package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/victor-devv/report-gen/fixtures"
	"github.com/victor-devv/report-gen/store"
)

func TestReportStore(t *testing.T) {
	env := fixtures.NewTestEnv(t)
	cleanup := env.SetupDb(t)
	t.Cleanup(func() {
		cleanup(t)
	})

	ctx := context.Background()

	reportStore := store.NewReportStore(env.Db)
	userStore := store.NewUserStore(env.Db)

	user, err := userStore.Create(ctx, "test@testemail.com", "testPassword")
	require.NoError(t, err)

	now := time.Now().UTC()
	report, err := reportStore.Create(ctx, user.Id, "monsters")
	after := time.Now().UTC()
	require.NoError(t, err)
	require.Equal(t, user.Id, report.UserId)
	require.Equal(t, "monsters", report.ReportType)

	// VERY FLAKY
	// TODO DEBUG
	// require.LessOrEqual(t, now.UnixNano(), report.CreatedAt.UnixNano())
	require.True(t, (report.CreatedAt.After(now) || report.CreatedAt.Equal(now)) &&
		(report.CreatedAt.Before(after) || report.CreatedAt.Equal(after)),
		"CreatedAt should be between before and after timestamps")
	//END FLAKY

	startedAt := report.CreatedAt.Add(time.Second)
	completedAt := report.CreatedAt.Add(2 * time.Second)
	failedAt := report.CreatedAt.Add(3 * time.Second)
	errMsg := "an error occurred"
	downloadUrl := "https://example.com/reports/123/download"
	outputPath := "s3://reports-test/reports"
	downloadUrlExpiresAt := report.CreatedAt.Add(4 * time.Second)

	report.ReportType = "food"
	report.StartedAt = &startedAt
	report.CompletedAt = &completedAt
	report.FailedAt = &failedAt
	report.ErrorMessage = &errMsg
	report.DownloadUrl = &downloadUrl
	report.OutputFilePath = &outputPath
	report.DownloadUrlExpiresAt = &downloadUrlExpiresAt

	updatedReport, err := reportStore.Update(ctx, report)
	require.NoError(t, err)

	require.Equal(t, report.UserId, updatedReport.UserId)
	require.Equal(t, report.Id, updatedReport.Id)
	require.Equal(t, "monsters", updatedReport.ReportType) //report type should not be updated
	require.Equal(t, report.CreatedAt.UnixNano(), updatedReport.CreatedAt.UnixNano())
	require.Equal(t, report.StartedAt.UnixNano(), updatedReport.StartedAt.UnixNano())
	require.Equal(t, report.CompletedAt.UnixNano(), updatedReport.CompletedAt.UnixNano())
	require.Equal(t, report.FailedAt.UnixNano(), updatedReport.FailedAt.UnixNano())
	require.Equal(t, &errMsg, report.ErrorMessage)
	require.Equal(t, &downloadUrl, report.DownloadUrl)
	require.Equal(t, &outputPath, report.OutputFilePath)
	require.Equal(t, (&downloadUrlExpiresAt).UnixNano(), report.DownloadUrlExpiresAt.UnixNano())

	report3, err := reportStore.ByPrimaryKey(ctx, report.Id, report.UserId)
	require.NoError(t, err)

	require.Equal(t, report.UserId, report3.UserId)
	require.Equal(t, report.Id, report3.Id)
	require.Equal(t, "monsters", report3.ReportType) //report type should not be updated
	require.Equal(t, report.CreatedAt.UnixNano(), report3.CreatedAt.UnixNano())
	require.Equal(t, report.StartedAt.UnixNano(), report3.StartedAt.UnixNano())
	require.Equal(t, report.CompletedAt.UnixNano(), report3.CompletedAt.UnixNano())
	require.Equal(t, report.FailedAt.UnixNano(), report3.FailedAt.UnixNano())
	require.Equal(t, &errMsg, report3.ErrorMessage)
	require.Equal(t, &downloadUrl, report3.DownloadUrl)
	require.Equal(t, &outputPath, report3.OutputFilePath)
	require.Equal(t, (&downloadUrlExpiresAt).UnixNano(), report3.DownloadUrlExpiresAt.UnixNano())
}
