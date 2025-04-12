package server_test

import (
	"github.com/victor-devv/report-gen/config"
	"github.com/victor-devv/report-gen/server"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestJwtManager(t *testing.T) {
	conf, err := config.New()
	require.NoError(t, err)

	jwtManager := server.NewJwtManager(conf)

	userId := uuid.New()
	tokenPair, err := jwtManager.GenerateTokenPair(userId)
	require.NoError(t, err)

	require.True(t, jwtManager.IsAccessToken(tokenPair.AccessToken))
	require.False(t, jwtManager.IsAccessToken(tokenPair.RefreshToken))

	accessTokenSubject, err := tokenPair.AccessToken.Claims.GetSubject()
	require.NoError(t, err)
	require.Equal(t, userId.String(), accessTokenSubject)

	refreshTokenSubject, err := tokenPair.RefreshToken.Claims.GetSubject()
	require.NoError(t, err)
	require.Equal(t, userId.String(), refreshTokenSubject)

	issuer := "http://" + conf.ServerHost + ":" + conf.ServerPort

	accessTokenIssuer, err := tokenPair.AccessToken.Claims.GetIssuer()
	require.NoError(t, err)
	require.Equal(t, issuer, accessTokenIssuer)

	refreshTokenIssuer, err := tokenPair.RefreshToken.Claims.GetIssuer()
	require.NoError(t, err)
	require.Equal(t, issuer, refreshTokenIssuer)

	parsedAccessToken, err := jwtManager.Parse(tokenPair.AccessToken.Raw)
	require.NoError(t, err)
	require.Equal(t, tokenPair.AccessToken, parsedAccessToken)

	parsedRefreshToken, err := jwtManager.Parse(tokenPair.RefreshToken.Raw)
	require.NoError(t, err)
	require.Equal(t, tokenPair.RefreshToken, parsedRefreshToken)
}
