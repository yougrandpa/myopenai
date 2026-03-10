//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type accountRepoStubForAdminList struct {
	accountRepoStub

	listWithFiltersCalls    int
	listWithFiltersParams   pagination.PaginationParams
	listWithFiltersPlatform string
	listWithFiltersType     string
	listWithFiltersStatus   string
	listWithFiltersSearch   string
	listWithFiltersAccounts []Account
	listWithFiltersResult   *pagination.PaginationResult
	listWithFiltersErr      error
}

func (s *accountRepoStubForAdminList) ListWithFilters(_ context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64) ([]Account, *pagination.PaginationResult, error) {
	s.listWithFiltersCalls++
	s.listWithFiltersParams = params
	s.listWithFiltersPlatform = platform
	s.listWithFiltersType = accountType
	s.listWithFiltersStatus = status
	s.listWithFiltersSearch = search

	if s.listWithFiltersErr != nil {
		return nil, nil, s.listWithFiltersErr
	}

	result := s.listWithFiltersResult
	if result == nil {
		result = &pagination.PaginationResult{
			Total:    int64(len(s.listWithFiltersAccounts)),
			Page:     params.Page,
			PageSize: params.PageSize,
		}
	}

	start := 0
	if params.Page > 1 && params.PageSize > 0 {
		start = (params.Page - 1) * params.PageSize
	}
	if start >= len(s.listWithFiltersAccounts) {
		return []Account{}, result, nil
	}
	end := len(s.listWithFiltersAccounts)
	if params.PageSize > 0 {
		limitEnd := start + params.PageSize
		if limitEnd < end {
			end = limitEnd
		}
	}

	return append([]Account(nil), s.listWithFiltersAccounts[start:end]...), result, nil
}

type proxyRepoStubForAdminList struct {
	proxyRepoStub

	listWithFiltersCalls    int
	listWithFiltersParams   pagination.PaginationParams
	listWithFiltersProtocol string
	listWithFiltersStatus   string
	listWithFiltersSearch   string
	listWithFiltersProxies  []Proxy
	listWithFiltersResult   *pagination.PaginationResult
	listWithFiltersErr      error

	listWithFiltersAndAccountCountCalls    int
	listWithFiltersAndAccountCountParams   pagination.PaginationParams
	listWithFiltersAndAccountCountProtocol string
	listWithFiltersAndAccountCountStatus   string
	listWithFiltersAndAccountCountSearch   string
	listWithFiltersAndAccountCountProxies  []ProxyWithAccountCount
	listWithFiltersAndAccountCountResult   *pagination.PaginationResult
	listWithFiltersAndAccountCountErr      error
}

func (s *proxyRepoStubForAdminList) ListWithFilters(_ context.Context, params pagination.PaginationParams, protocol, status, search string) ([]Proxy, *pagination.PaginationResult, error) {
	s.listWithFiltersCalls++
	s.listWithFiltersParams = params
	s.listWithFiltersProtocol = protocol
	s.listWithFiltersStatus = status
	s.listWithFiltersSearch = search

	if s.listWithFiltersErr != nil {
		return nil, nil, s.listWithFiltersErr
	}

	result := s.listWithFiltersResult
	if result == nil {
		result = &pagination.PaginationResult{
			Total:    int64(len(s.listWithFiltersProxies)),
			Page:     params.Page,
			PageSize: params.PageSize,
		}
	}

	return s.listWithFiltersProxies, result, nil
}

func (s *proxyRepoStubForAdminList) ListWithFiltersAndAccountCount(_ context.Context, params pagination.PaginationParams, protocol, status, search string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	s.listWithFiltersAndAccountCountCalls++
	s.listWithFiltersAndAccountCountParams = params
	s.listWithFiltersAndAccountCountProtocol = protocol
	s.listWithFiltersAndAccountCountStatus = status
	s.listWithFiltersAndAccountCountSearch = search

	if s.listWithFiltersAndAccountCountErr != nil {
		return nil, nil, s.listWithFiltersAndAccountCountErr
	}

	result := s.listWithFiltersAndAccountCountResult
	if result == nil {
		result = &pagination.PaginationResult{
			Total:    int64(len(s.listWithFiltersAndAccountCountProxies)),
			Page:     params.Page,
			PageSize: params.PageSize,
		}
	}

	return s.listWithFiltersAndAccountCountProxies, result, nil
}

type redeemRepoStubForAdminList struct {
	redeemRepoStub

	listWithFiltersCalls  int
	listWithFiltersParams pagination.PaginationParams
	listWithFiltersType   string
	listWithFiltersStatus string
	listWithFiltersSearch string
	listWithFiltersCodes  []RedeemCode
	listWithFiltersResult *pagination.PaginationResult
	listWithFiltersErr    error
}

func (s *redeemRepoStubForAdminList) ListWithFilters(_ context.Context, params pagination.PaginationParams, codeType, status, search string) ([]RedeemCode, *pagination.PaginationResult, error) {
	s.listWithFiltersCalls++
	s.listWithFiltersParams = params
	s.listWithFiltersType = codeType
	s.listWithFiltersStatus = status
	s.listWithFiltersSearch = search

	if s.listWithFiltersErr != nil {
		return nil, nil, s.listWithFiltersErr
	}

	result := s.listWithFiltersResult
	if result == nil {
		result = &pagination.PaginationResult{
			Total:    int64(len(s.listWithFiltersCodes)),
			Page:     params.Page,
			PageSize: params.PageSize,
		}
	}

	return s.listWithFiltersCodes, result, nil
}

func (s *redeemRepoStubForAdminList) ListByUserPaginated(_ context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}

func (s *redeemRepoStubForAdminList) SumPositiveBalanceByUser(_ context.Context, userID int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}

func TestAdminService_ListAccounts_WithSearch(t *testing.T) {
	t.Run("search 参数正常传递到 repository 层", func(t *testing.T) {
		repo := &accountRepoStubForAdminList{
			listWithFiltersAccounts: []Account{{ID: 1, Name: "acc"}},
			listWithFiltersResult:   &pagination.PaginationResult{Total: 10},
		}
		svc := &adminServiceImpl{accountRepo: repo}

		accounts, total, err := svc.ListAccounts(context.Background(), 1, 20, PlatformGemini, AccountTypeOAuth, StatusActive, "acc", 0, "", "")
		require.NoError(t, err)
		require.Equal(t, int64(10), total)
		require.Equal(t, []Account{{ID: 1, Name: "acc"}}, accounts)

		require.Equal(t, 1, repo.listWithFiltersCalls)
		require.Equal(t, pagination.PaginationParams{Page: 1, PageSize: 20}, repo.listWithFiltersParams)
		require.Equal(t, PlatformGemini, repo.listWithFiltersPlatform)
		require.Equal(t, AccountTypeOAuth, repo.listWithFiltersType)
		require.Equal(t, StatusActive, repo.listWithFiltersStatus)
		require.Equal(t, "acc", repo.listWithFiltersSearch)
	})
}

func TestAdminService_ListAccounts_SortUsageAcrossPages(t *testing.T) {
	repo := &accountRepoStubForAdminList{
		listWithFiltersAccounts: []Account{
			{ID: 4, Name: "error", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusError},
			{ID: 3, Name: "warn", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, SessionWindowStatus: "allowed_warning"},
			{ID: 2, Name: "openai", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Extra: map[string]any{"codex_5h_used_percent": 50.0, "codex_5h_reset_at": time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339)}},
			{ID: 1, Name: "fresh", Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, SessionWindowStatus: "allowed"},
		},
		listWithFiltersResult: &pagination.PaginationResult{Total: 4},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	accounts, total, err := svc.ListAccounts(context.Background(), 2, 2, "", "", "", "", 0, "usage", "asc")
	require.NoError(t, err)
	require.Equal(t, int64(4), total)
	require.Len(t, accounts, 2)
	require.Equal(t, int64(3), accounts[0].ID)
	require.Equal(t, int64(4), accounts[1].ID)
	require.Equal(t, 1, repo.listWithFiltersCalls)
	require.Equal(t, pagination.PaginationParams{Page: 1, PageSize: 100}, repo.listWithFiltersParams)
}

func TestAdminService_ListAccounts_SortAcrossRepositoryPages(t *testing.T) {
	allAccounts := make([]Account, 0, 150)
	for i := 0; i < 150; i++ {
		allAccounts = append(allAccounts, Account{
			ID:       int64(150 - i),
			Name:     "acc-" + strconv.Itoa(150-i),
			Priority: 150 - i,
			Status:   StatusActive,
		})
	}

	repo := &accountRepoStubForAdminList{
		listWithFiltersAccounts: allAccounts,
		listWithFiltersResult:   &pagination.PaginationResult{Total: int64(len(allAccounts))},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	accounts, total, err := svc.ListAccounts(context.Background(), 3, 20, "", "", "", "", 0, "priority", "asc")
	require.NoError(t, err)
	require.Equal(t, int64(150), total)
	require.Len(t, accounts, 20)
	require.Equal(t, int64(41), accounts[0].ID)
	require.Equal(t, int64(60), accounts[len(accounts)-1].ID)
	require.Equal(t, 2, repo.listWithFiltersCalls)
	require.Equal(t, pagination.PaginationParams{Page: 2, PageSize: 100}, repo.listWithFiltersParams)
}

func TestAdminService_ListProxies_WithSearch(t *testing.T) {
	t.Run("search 参数正常传递到 repository 层", func(t *testing.T) {
		repo := &proxyRepoStubForAdminList{
			listWithFiltersProxies: []Proxy{{ID: 2, Name: "p1"}},
			listWithFiltersResult:  &pagination.PaginationResult{Total: 7},
		}
		svc := &adminServiceImpl{proxyRepo: repo}

		proxies, total, err := svc.ListProxies(context.Background(), 3, 50, "http", StatusActive, "p1")
		require.NoError(t, err)
		require.Equal(t, int64(7), total)
		require.Equal(t, []Proxy{{ID: 2, Name: "p1"}}, proxies)

		require.Equal(t, 1, repo.listWithFiltersCalls)
		require.Equal(t, pagination.PaginationParams{Page: 3, PageSize: 50}, repo.listWithFiltersParams)
		require.Equal(t, "http", repo.listWithFiltersProtocol)
		require.Equal(t, StatusActive, repo.listWithFiltersStatus)
		require.Equal(t, "p1", repo.listWithFiltersSearch)
	})
}

func TestAdminService_ListProxiesWithAccountCount_WithSearch(t *testing.T) {
	t.Run("search 参数正常传递到 repository 层", func(t *testing.T) {
		repo := &proxyRepoStubForAdminList{
			listWithFiltersAndAccountCountProxies: []ProxyWithAccountCount{{Proxy: Proxy{ID: 3, Name: "p2"}, AccountCount: 5}},
			listWithFiltersAndAccountCountResult:  &pagination.PaginationResult{Total: 9},
		}
		svc := &adminServiceImpl{proxyRepo: repo}

		proxies, total, err := svc.ListProxiesWithAccountCount(context.Background(), 2, 10, "socks5", StatusDisabled, "p2")
		require.NoError(t, err)
		require.Equal(t, int64(9), total)
		require.Equal(t, []ProxyWithAccountCount{{Proxy: Proxy{ID: 3, Name: "p2"}, AccountCount: 5}}, proxies)

		require.Equal(t, 1, repo.listWithFiltersAndAccountCountCalls)
		require.Equal(t, pagination.PaginationParams{Page: 2, PageSize: 10}, repo.listWithFiltersAndAccountCountParams)
		require.Equal(t, "socks5", repo.listWithFiltersAndAccountCountProtocol)
		require.Equal(t, StatusDisabled, repo.listWithFiltersAndAccountCountStatus)
		require.Equal(t, "p2", repo.listWithFiltersAndAccountCountSearch)
	})
}

func TestAdminService_ListRedeemCodes_WithSearch(t *testing.T) {
	t.Run("search 参数正常传递到 repository 层", func(t *testing.T) {
		repo := &redeemRepoStubForAdminList{
			listWithFiltersCodes:  []RedeemCode{{ID: 4, Code: "ABC"}},
			listWithFiltersResult: &pagination.PaginationResult{Total: 3},
		}
		svc := &adminServiceImpl{redeemCodeRepo: repo}

		codes, total, err := svc.ListRedeemCodes(context.Background(), 1, 20, RedeemTypeBalance, StatusUnused, "ABC")
		require.NoError(t, err)
		require.Equal(t, int64(3), total)
		require.Equal(t, []RedeemCode{{ID: 4, Code: "ABC"}}, codes)

		require.Equal(t, 1, repo.listWithFiltersCalls)
		require.Equal(t, pagination.PaginationParams{Page: 1, PageSize: 20}, repo.listWithFiltersParams)
		require.Equal(t, RedeemTypeBalance, repo.listWithFiltersType)
		require.Equal(t, StatusUnused, repo.listWithFiltersStatus)
		require.Equal(t, "ABC", repo.listWithFiltersSearch)
	})
}
