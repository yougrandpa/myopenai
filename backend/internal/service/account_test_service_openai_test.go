package service

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestService_testOpenAIAccountConnection_SetsOAuthHeaders(t *testing.T) {
	upstream := &queuedHTTPUpstream{
		responses: []*http.Response{
			newJSONResponse(http.StatusOK, "data: {\"type\":\"response.completed\"}\n\n"),
		},
	}
	svc := &AccountTestService{httpUpstream: upstream}
	account := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "tok_test",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	gin.SetMode(gin.TestMode)
	c, rec := newSoraTestContext()
	err := svc.testOpenAIAccountConnection(c, account, "gpt-5.4")

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	require.Equal(t, chatgptCodexAPIURL, req.URL.String())
	require.Equal(t, "Bearer tok_test", req.Header.Get("Authorization"))
	require.Equal(t, "text/event-stream", req.Header.Get("accept"))
	require.Equal(t, "responses=experimental", req.Header.Get("OpenAI-Beta"))
	require.Equal(t, "codex_cli_rs", req.Header.Get("originator"))
	require.Equal(t, codexCLIUserAgent, req.Header.Get("user-agent"))
	require.Equal(t, "chatgpt-acc", req.Header.Get("chatgpt-account-id"))
	require.Contains(t, rec.Body.String(), `"type":"test_complete","success":true`)
}
