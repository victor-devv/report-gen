package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

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
