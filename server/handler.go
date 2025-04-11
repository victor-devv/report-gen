package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
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

		user, err := s.store.Users.CreateUser(r.Context(), req.Email, req.Password)
		if err != nil {
			return NewErrWithStatus(err, http.StatusInternalServerError)
		}

		successResponse(w, http.StatusCreated, "user created successfully", &user)
		return nil
	})
}
