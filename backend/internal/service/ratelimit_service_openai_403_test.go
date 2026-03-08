//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRateLimitService_HandleUpstreamError_OpenAIOAuth403EdgeBlockSetsTempUnschedulable(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	svc := NewRateLimitService(repo, nil, &config.Config{
		RateLimit: config.RateLimitConfig{OAuth401CooldownMinutes: 7},
	}, nil, nil)
	account := &Account{ID: 200, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	headers := http.Header{}
	headers.Set("cf-ray", "test-ray")

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, 403, headers, nil)

	require.True(t, shouldDisable)
	require.Equal(t, 0, repo.setErrorCalls)
	require.Equal(t, 1, repo.tempCalls)
	require.Contains(t, repo.lastTempReason, "cf-ray")
}

func TestRateLimitService_HandleUpstreamError_OpenAIOAuth403WithMessageSetsError(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{ID: 201, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	headers := http.Header{}
	headers.Set("cf-ray", "test-ray")

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, 403, headers, []byte(`{"error":{"message":"insufficient permissions"}}`))

	require.True(t, shouldDisable)
	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, 0, repo.tempCalls)
	require.Contains(t, repo.lastErrorMsg, "insufficient permissions")
}
