package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
	"github.com/victor-devv/report-gen/reports"
	"github.com/victor-devv/report-gen/store"
)

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r SignupRequest) Validate() error {
	if r.Email == "" {
		return errors.New("email is required")
	}

	if r.Password == "" {
		return errors.New("password is required")
	}

	return nil
}

func (s *Server) signupHandler() http.HandlerFunc {
	return handleWithError(func(w http.ResponseWriter, r *http.Request) error {
		req, err := decode[SignupRequest](r)
		if err != nil {
			return NewErrWithStatus(err, http.StatusBadRequest)
		}

		userExists, err := s.store.Users.ByEmail(r.Context(), req.Email)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		if userExists != nil {
			return NewErrWithStatus(fmt.Errorf("a user with matching details already exists: %v", err), http.StatusConflict)
		}

		user, err := s.store.Users.Create(r.Context(), req.Email, req.Password)
		if err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		successResponse(w, http.StatusCreated, "user created successfully", &user)
		return nil
	})
}

type SigninRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SigninResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	User         *store.User `json:"user"`
}

func (r SigninRequest) Validate() error {
	if r.Email == "" {
		return errors.New("email is required")
	}

	if r.Password == "" {
		return errors.New("password is required")
	}

	return nil
}

func (s *Server) signInHandler() http.HandlerFunc {
	return handleWithError(func(w http.ResponseWriter, r *http.Request) error {
		req, err := decode[SignupRequest](r)
		if err != nil {
			return NewErrWithStatus(err, http.StatusBadRequest)
		}

		user, err := s.store.Users.ByEmail(r.Context(), req.Email)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		if user == nil {
			return NewErrWithStatus(fmt.Errorf("user not found: %v", err), http.StatusBadRequest)
		}

		if err := user.ComparePassword(req.Password); err != nil {
			return NewErrWithStatus(err, http.StatusUnauthorized)
		}

		tokenPair, err := s.jwtManager.GenerateTokenPair(user.Id)
		if err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		_, err = s.store.RefreshToken.Delete(r.Context(), user.Id)
		if err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		_, err = s.store.RefreshToken.Create(r.Context(), user.Id, tokenPair.RefreshToken)
		if err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		successResponse(w, http.StatusOK, "", SigninResponse{
			AccessToken:  tokenPair.AccessToken.Raw,
			RefreshToken: tokenPair.RefreshToken.Raw,
			User:         user,
		})
		return nil
	})
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (r RefreshTokenRequest) Validate() error {
	if r.RefreshToken == "" {
		return errors.New("refresh_token is required")
	}

	return nil
}

func (s *Server) refreshTokenHandler() http.HandlerFunc {
	return handleWithError(func(w http.ResponseWriter, r *http.Request) error {
		req, err := decode[RefreshTokenRequest](r)
		if err != nil {
			return NewErrWithStatus(err, http.StatusBadRequest)
		}

		currentRefreshToken, err := s.jwtManager.Parse(req.RefreshToken)
		if err != nil {
			return NewErrWithStatus(err, http.StatusUnauthorized)
		}

		userIdStr, err := currentRefreshToken.Claims.GetSubject()
		if err != nil {
			return NewErrWithStatus(err, http.StatusUnauthorized)
		}

		userId, err := uuid.Parse(userIdStr)
		if err != nil {
			return NewErrWithStatus(err, http.StatusUnauthorized)
		}

		currentRefreshTokeRecord, err := s.store.RefreshToken.ByPrimaryKey(r.Context(), userId, currentRefreshToken)
		if err != nil {
			status := http.StatusInsufficientStorage
			if errors.Is(err, sql.ErrNoRows) {
				status = http.StatusUnauthorized
			}
			return NewErrWithStatus(err, status)
		}

		if currentRefreshTokeRecord.ExpiresAt.Before(time.Now()) {
			return NewErrWithStatus(fmt.Errorf("refresh token expired: %w", err), http.StatusUnauthorized)
		}

		tokenPair, err := s.jwtManager.GenerateTokenPair(userId)
		if err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		_, err = s.store.RefreshToken.Delete(r.Context(), userId)
		if err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		if _, err := s.store.RefreshToken.Create(r.Context(), userId, tokenPair.RefreshToken); err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		successResponse(w, http.StatusOK, "", RefreshTokenResponse{
			AccessToken:  tokenPair.AccessToken.Raw,
			RefreshToken: tokenPair.RefreshToken.Raw,
		})

		return nil
	})
}

type CreateReportRequest struct {
	ReportType string `json:"report_type"`
}

type CreateReportResponse struct {
	Id                   uuid.UUID  `json:"id"`
	ReportType           string     `json:"report_type,omitempty"`
	OutputFilePath       *string    `json:"output_file_path,omitempty"`
	DownloadUrl          *string    `json:"download_url,omitempty"`
	DownloadUrlExpiresAt *time.Time `json:"download_url_expires_at,omitempty"`
	ErrorMessage         *string    `json:"error_message,omitempty"`
	CreatedAt            time.Time  `json:"created_at,omitempty"`
	StartedAt            *time.Time `json:"started_at,omitempty"`
	FailedAt             *time.Time `json:"failed_at,omitempty"`
	CompletedAt          *time.Time `json:"completed_at,omitempty"`
	Status               string     `json:"status,omitempty"`
}

func (r CreateReportRequest) Validate() error {
	if r.ReportType == "" {
		return errors.New("report_type is required")
	}

	return nil
}

func (s *Server) createReportHandler() http.HandlerFunc {
	return handleWithError(func(w http.ResponseWriter, r *http.Request) error {
		req, err := decode[CreateReportRequest](r)
		if err != nil {
			return NewErrWithStatus(err, http.StatusBadRequest)
		}

		user, ok := GetUserFromContext(r.Context())
		if !ok {
			return NewErrWithStatus(err, http.StatusUnauthorized)
		}

		report, err := s.store.Reports.Create(r.Context(), user.Id, req.ReportType)
		if err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		// send to SQS to beb picked up by worker
		sqsMessage := reports.SqsMessage{
			UserId:   user.Id,
			ReportId: report.Id,
		}

		bytes, err := json.Marshal(sqsMessage)
		if err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		queueUrlOut, err := s.sqsClient.GetQueueUrl(r.Context(), &sqs.GetQueueUrlInput{
			QueueName: aws.String(s.config.SqsQueue),
		})
		if err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		_, err = s.sqsClient.SendMessage(r.Context(), &sqs.SendMessageInput{
			QueueUrl:    queueUrlOut.QueueUrl,
			MessageBody: aws.String(string(bytes)),
		})
		if err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		successResponse(w, http.StatusCreated, "", CreateReportResponse{
			Id:                   report.Id,
			ReportType:           report.ReportType,
			OutputFilePath:       report.OutputFilePath,
			DownloadUrl:          report.DownloadUrl,
			DownloadUrlExpiresAt: report.DownloadUrlExpiresAt,
			ErrorMessage:         report.ErrorMessage,
			CreatedAt:            report.CreatedAt,
			StartedAt:            report.StartedAt,
			FailedAt:             report.FailedAt,
			CompletedAt:          report.CompletedAt,
			Status:               report.Status(),
		})

		return nil
	})
}

func (s *Server) getReportHandler() http.HandlerFunc {
	return handleWithError(func(w http.ResponseWriter, r *http.Request) error {
		reportIdStr := r.PathValue("report")
		reportId, err := uuid.Parse(reportIdStr)
		if err != nil {
			return NewErrWithStatus(err, http.StatusBadRequest)
		}

		user, ok := GetUserFromContext(r.Context())
		if !ok {
			return NewErrWithStatus(err, http.StatusUnauthorized)
		}

		report, err := s.store.Reports.ByPrimaryKey(r.Context(), reportId, user.Id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return NewErrWithStatus(err, http.StatusNotFound)
			}
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		if report.CompletedAt != nil {
			needsRefresh := report.DownloadUrlExpiresAt != nil && report.DownloadUrlExpiresAt.Before(time.Now())

			if report.DownloadUrl == nil || needsRefresh {
				// CREATE PRESIGNED URL
				expiresAt := time.Now().Add(10 * time.Second)
				signedUrl, err := s.preSignClient.PresignGetObject(r.Context(), &s3.GetObjectInput{
					Bucket: aws.String(s.config.S3Bucket),
					Key:    report.OutputFilePath,
				}, func(options *s3.PresignOptions) {
					options.Expires = time.Until(expiresAt)
				})
				if err != nil {
					return NewErrWithStatus(err, http.StatusInternalServerError)
				}

				report.DownloadUrl = &signedUrl.URL
				report.DownloadUrlExpiresAt = &expiresAt
				report, err = s.store.Reports.Update(r.Context(), report)
				if err != nil {
					return NewErrWithStatus(err, http.StatusInternalServerError)
				}
			}
		}

		successResponse(w, http.StatusOK, "", CreateReportResponse{
			Id:                   report.Id,
			ReportType:           report.ReportType,
			OutputFilePath:       report.OutputFilePath,
			DownloadUrl:          report.DownloadUrl,
			DownloadUrlExpiresAt: report.DownloadUrlExpiresAt,
			ErrorMessage:         report.ErrorMessage,
			CreatedAt:            report.CreatedAt,
			StartedAt:            report.StartedAt,
			FailedAt:             report.FailedAt,
			CompletedAt:          report.CompletedAt,
			Status:               report.Status(),
		})

		return nil
	})
}
