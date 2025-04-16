package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type ReportStore struct {
	db *sqlx.DB
}

func NewReportStore(db *sql.DB) *ReportStore {
	return &ReportStore{
		db: sqlx.NewDb(db, "postgres"),
	}
}

// nullables should be pointers
type Report struct {
	Id                   uuid.UUID  `db:"id" json:"id"`
	UserId               uuid.UUID  `db:"user_id" json:"user_id"`
	ReportType           string     `db:"report_type" json:"report_type"`
	OutputFilePath       *string    `db:"output_file_path" json:"output_file_path"`
	DownloadUrl          *string    `db:"download_url" json:"download_url"`
	DownloadUrlExpiresAt *time.Time `db:"download_url_expires_at" json:"download_url_expires_at"`
	ErrorMessage         *string    `db:"error_message" json:"error_message"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
	StartedAt            *time.Time `db:"started_at" json:"started_at"`
	FailedAt             *time.Time `db:"failed_at" json:"failed_at"`
	CompletedAt          *time.Time `db:"completed_at" json:"completed_at"`
}

func (s *ReportStore) Create(ctx context.Context, user_id uuid.UUID, reportType string) (*Report, error) {
	const dml = `INSERT INTO reports (user_id, report_type) VALUES ($1, $2) RETURNING *`
	var report Report

	if err := s.db.GetContext(ctx, &report, dml, user_id, reportType); err != nil {
		return nil, fmt.Errorf("failed to create report: %w", err)
	}

	return &report, nil
}

func (s *ReportStore) Update(ctx context.Context, report *Report) (*Report, error) {
	const dml = `UPDATE reports 
							SET 
								output_file_path = $1, 
								download_url = $2, 
								download_url_expires_at = $3, 
								error_message = $4, 
								started_at = $5, 
								completed_at = $6, 
								failed_at = $7 
							WHERE user_id = $8 AND id = $9 RETURNING *`

	var updatedReport Report

	if err := s.db.GetContext(ctx, &updatedReport, dml,
		report.OutputFilePath,
		report.DownloadUrl,
		report.DownloadUrlExpiresAt,
		report.ErrorMessage,
		report.StartedAt,
		report.CompletedAt,
		report.FailedAt,
		report.UserId,
		report.Id,
	); err != nil {
		return nil, fmt.Errorf("failed to update report: %w", err)
	}

	return &updatedReport, nil
}

func (s *ReportStore) ByPrimaryKey(ctx context.Context, id, userId uuid.UUID) (*Report, error) {
	const query = `SELECT * FROM reports WHERE id = $1 AND user_id = $2`

	var report Report

	if err := s.db.GetContext(ctx, &report, query, id, userId); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch report %s record for user %s: %w", id, userId, err)
	}

	return &report, nil
}
