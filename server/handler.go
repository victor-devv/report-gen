package server

import (
	"database/sql"
	"encoding/json"
	"errors"
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

func (s *Server) signupHandler(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorResponse(w, Error, "an error occurred: "+err.Error(), http.StatusBadRequest, (*struct{})(nil))
		return
	}

	defer r.Body.Close()

	if err := req.Validate(); err != nil {
		ErrorResponse(w, Error, "a validation error occurred: "+err.Error(), http.StatusBadRequest, (*struct{})(nil))
		return
	}

	userExists, err := s.store.Users.ByEmail(r.Context(), req.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		ErrorResponse(w, Error, "an error occurred: "+err.Error(), http.StatusInternalServerError, (*struct{})(nil))
		return
	}

	if userExists != nil {
		ErrorResponse(w, Error, "a user with matching details already exists", http.StatusConflict, (*struct{})(nil))
		return
	}

	user, err := s.store.Users.CreateUser(r.Context(), req.Email, req.Password)
	if err != nil {
		ErrorResponse(w, Error, err.Error(), http.StatusConflict, (*struct{})(nil))
		return
	}

	SuccessResponse(w, http.StatusCreated, "user created successfully", &user)
}
