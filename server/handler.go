package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
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
