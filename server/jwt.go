package server

import (
	"fmt"
	"github.com/victor-devv/report-gen/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var signingMethod = jwt.SigningMethodHS256

type JwtManager struct {
	config *config.Config
}

func NewJwtManager(config *config.Config) *JwtManager {
	return &JwtManager{config}
}

type TokenPair struct {
	AccessToken  *jwt.Token `json:"access_token"`
	RefreshToken *jwt.Token `json:"refresh_token"`
}

type CustomClams struct {
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

func (j *JwtManager) GenerateTokenPair(userId uuid.UUID) (*TokenPair, error) {
	now := time.Now()
	issuer := "http://" + j.config.ServerHost + ":" + j.config.ServerPort
	key := []byte(j.config.JwtSecret)
	var err error

	accessToken := jwt.NewWithClaims(signingMethod, CustomClams{
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userId.String(),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 15)),
		},
	})

	signedAccessToken, err := accessToken.SignedString(key)
	if err != nil {
		return nil, fmt.Errorf("error signing access token: %w", err)
	}

	accessTokenStr, err := j.Parse(signedAccessToken)
	if err != nil {
		return nil, fmt.Errorf("error parsing access token: %w", err)
	}

	refreshToken := jwt.NewWithClaims(signingMethod, CustomClams{
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userId.String(),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour * 24 * 30)),
		},
	})

	signedRefreshToken, err := refreshToken.SignedString(key)
	if err != nil {
		return nil, fmt.Errorf("error signing refresh token: %w", err)
	}

	refreshTokenStr, err := j.Parse(signedRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("error parsing refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenStr,
		RefreshToken: refreshTokenStr,
	}, nil
}

func (j *JwtManager) Parse(token string) (*jwt.Token, error) {
	parser := jwt.NewParser()

	parsedToken, err := parser.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if t.Method != signingMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(j.config.JwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}
	return parsedToken, nil
}

func (j *JwtManager) IsAccessToken(token *jwt.Token) bool {
	//Assert that token.Claims is of type jwt.MapClaims, and give me access to it as that type.
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false
	}

	if tokenType, ok := claims["token_type"]; ok {
		return tokenType == "access"
	}
	return false
}
