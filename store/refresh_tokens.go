package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type RefreshTokenStore struct {
	db *sqlx.DB
}

func NewRefreshTokenStore(db *sql.DB) *RefreshTokenStore {
	return &RefreshTokenStore{
		db: sqlx.NewDb(db, "postgres"),
	}
}

type RefreshToken struct {
	UserId      uuid.UUID `db:"user_id"`
	HashedToken string    `db:"hashed_token"`
	CreatedAt   time.Time `db:"created_at"`
	ExpiresAt   time.Time `db:"expires_at"`
}

func (s *RefreshTokenStore) getBase64TokenHash(token *jwt.Token) (string, error) {
	h := sha256.New()
	h.Write([]byte(token.Raw))
	hashedToken := h.Sum(nil)

	base64TokenHash := base64.StdEncoding.EncodeToString(hashedToken)

	return base64TokenHash, nil
}

func (s *RefreshTokenStore) Create(ctx context.Context, userId uuid.UUID, token *jwt.Token) (*RefreshToken, error) {
	const insert = `INSERT INTO refresh_tokens (user_id, hashed_token, expires_at) VALUES ($1, $2, $3) RETURNING *`

	base64TokenHash, err := s.getBase64TokenHash(token)
	if err != nil {
		return nil, err
	}

	expiresAt, err := token.Claims.GetExpirationTime()
	if err != nil {
		return nil, fmt.Errorf("failed to extract token expiration: %w", err)
	}

	var refreshToken RefreshToken

	if err := s.db.GetContext(ctx, &refreshToken, insert, userId, base64TokenHash, expiresAt.Time); err != nil {
		return nil, fmt.Errorf("failed to create refresh token: %w", err)
	}

	return &refreshToken, nil
}

func (s *RefreshTokenStore) ByPrimaryKey(ctx context.Context, userId uuid.UUID, token *jwt.Token) (*RefreshToken, error) {
	const query = `SELECT * FROM refresh_tokens WHERE user_id = $1 AND hashed_token = $2`

	base64TokenHash, err := s.getBase64TokenHash(token)
	if err != nil {
		return nil, err
	}

	var refreshToken RefreshToken

	if err := s.db.GetContext(ctx, &refreshToken, query, userId, base64TokenHash); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("failed to fetch hashed_token %s record for user %s: %w", base64TokenHash, userId, err)
	}

	return &refreshToken, nil
}

func (s *RefreshTokenStore) Delete(ctx context.Context, userId uuid.UUID) (sql.Result, error) {
	const query = `DELETE FROM refresh_tokens WHERE user_id = $1`

	result, err := s.db.ExecContext(ctx, query, userId)

	if err != nil {
		return result, fmt.Errorf("failed to delete refresh token records for user %s: %w", userId, err)
	}

	return result, nil
}
