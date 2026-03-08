package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newTestOpenAIQuotaAccount(id int64, usedPercent float64) Account {
	return Account{
		ID:          id,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			"codex_5h_used_percent": usedPercent,
		},
	}
}

func TestAccount_ShouldDeprioritizeForLowQuotaThreshold(t *testing.T) {
	t.Run("exactly five percent remaining is deprioritized", func(t *testing.T) {
		account := newTestOpenAIQuotaAccount(1, 95)

		remainingPercent, ok := account.OpenAIQuotaRemainingPercent()
		require.True(t, ok)
		require.InDelta(t, 5.0, remainingPercent, 0.001)
		require.True(t, account.ShouldDeprioritizeForLowQuota())
	})

	t.Run("more than five percent remaining stays preferred", func(t *testing.T) {
		account := newTestOpenAIQuotaAccount(2, 94.9)

		remainingPercent, ok := account.OpenAIQuotaRemainingPercent()
		require.True(t, ok)
		require.InDelta(t, 5.1, remainingPercent, 0.001)
		require.False(t, account.ShouldDeprioritizeForLowQuota())
	})

	t.Run("highest usage window wins", func(t *testing.T) {
		account := newTestOpenAIQuotaAccount(3, 60)
		account.Extra["codex_7d_used_percent"] = 98.0
		account.Extra["codex_primary_used_percent"] = 90.0

		remainingPercent, ok := account.OpenAIQuotaRemainingPercent()
		require.True(t, ok)
		require.InDelta(t, 2.0, remainingPercent, 0.001)
		require.True(t, account.ShouldDeprioritizeForLowQuota())
	})
}

func TestSortAccountsByPriorityAndLastUsed_DeprioritizesLowQuota(t *testing.T) {
	healthy := newTestOpenAIQuotaAccount(101, 80)
	lowQuota := newTestOpenAIQuotaAccount(102, 97)
	now := time.Now()
	healthy.LastUsedAt = &now
	lowQuota.LastUsedAt = &now

	accounts := []*Account{&lowQuota, &healthy}
	sortAccountsByPriorityAndLastUsed(accounts, false)

	require.Equal(t, int64(101), accounts[0].ID)
	require.Equal(t, int64(102), accounts[1].ID)
}

func TestOpenAIGatewayService_IsBetterAccount_DeprioritizesLowQuota(t *testing.T) {
	svc := &OpenAIGatewayService{}
	healthy := newTestOpenAIQuotaAccount(201, 70)
	lowQuota := newTestOpenAIQuotaAccount(202, 97)
	lowQuota.Priority = -10
	now := time.Now()
	healthy.LastUsedAt = &now
	lowQuota.LastUsedAt = &now

	require.True(t, svc.isBetterAccount(&healthy, &lowQuota))
	require.False(t, svc.isBetterAccount(&lowQuota, &healthy))
}

func TestDefaultOpenAIAccountScheduler_SelectByLoadBalance_LowQuotaBehavior(t *testing.T) {
	ctx := context.Background()

	newScheduler := func(accounts []Account, loadMap map[int64]*AccountLoadInfo) *defaultOpenAIAccountScheduler {
		cfg := &config.Config{}
		cfg.Gateway.OpenAIWS.LBTopK = 1
		cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
		cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
		cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 1
		cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 1
		cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 1

		svc := &OpenAIGatewayService{
			accountRepo: stubOpenAIAccountRepo{accounts: accounts},
			cfg:        cfg,
			concurrencyService: NewConcurrencyService(stubConcurrencyCache{
				loadMap: loadMap,
			}),
		}
		schedulerAny := newDefaultOpenAIAccountScheduler(svc, newOpenAIAccountRuntimeStats())
		scheduler, ok := schedulerAny.(*defaultOpenAIAccountScheduler)
		require.True(t, ok)
		return scheduler
	}

	t.Run("healthy account wins when everything else is equal", func(t *testing.T) {
		healthy := newTestOpenAIQuotaAccount(301, 75)
		lowQuota := newTestOpenAIQuotaAccount(302, 98)
		scheduler := newScheduler(
			[]Account{lowQuota, healthy},
			map[int64]*AccountLoadInfo{
				301: {AccountID: 301, LoadRate: 20, WaitingCount: 1},
				302: {AccountID: 302, LoadRate: 20, WaitingCount: 1},
			},
		)

		selection, candidateCount, topK, _, err := scheduler.selectByLoadBalance(ctx, OpenAIAccountScheduleRequest{})
		require.NoError(t, err)
		require.Equal(t, 2, candidateCount)
		require.Equal(t, 1, topK)
		require.NotNil(t, selection)
		require.NotNil(t, selection.Account)
		require.Equal(t, int64(301), selection.Account.ID)
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
	})

	t.Run("all low quota accounts still keep a fallback winner", func(t *testing.T) {
		preferred := newTestOpenAIQuotaAccount(401, 97)
		fallback := newTestOpenAIQuotaAccount(402, 99)
		fallback.Priority = 5
		scheduler := newScheduler(
			[]Account{preferred, fallback},
			map[int64]*AccountLoadInfo{
				401: {AccountID: 401, LoadRate: 10, WaitingCount: 0},
				402: {AccountID: 402, LoadRate: 10, WaitingCount: 0},
			},
		)

		selection, candidateCount, topK, _, err := scheduler.selectByLoadBalance(ctx, OpenAIAccountScheduleRequest{})
		require.NoError(t, err)
		require.Equal(t, 2, candidateCount)
		require.Equal(t, 1, topK)
		require.NotNil(t, selection)
		require.NotNil(t, selection.Account)
		require.Equal(t, int64(401), selection.Account.ID)
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
	})
}
