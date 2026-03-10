package service

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	adminAccountSortFetchPageSize = 100
	adminUsageSortCategorySpan    = int64(1_000_000)
)

type adminAccountSortKey string

const (
	adminAccountSortName           adminAccountSortKey = "name"
	adminAccountSortStatus         adminAccountSortKey = "status"
	adminAccountSortSchedulable    adminAccountSortKey = "schedulable"
	adminAccountSortUsage          adminAccountSortKey = "usage"
	adminAccountSortPriority       adminAccountSortKey = "priority"
	adminAccountSortRateMultiplier adminAccountSortKey = "rate_multiplier"
	adminAccountSortLastUsedAt     adminAccountSortKey = "last_used_at"
	adminAccountSortExpiresAt      adminAccountSortKey = "expires_at"
)

func normalizeAdminAccountSortKey(raw string) adminAccountSortKey {
	switch strings.TrimSpace(raw) {
	case string(adminAccountSortName):
		return adminAccountSortName
	case string(adminAccountSortStatus):
		return adminAccountSortStatus
	case string(adminAccountSortSchedulable):
		return adminAccountSortSchedulable
	case string(adminAccountSortUsage):
		return adminAccountSortUsage
	case string(adminAccountSortPriority):
		return adminAccountSortPriority
	case string(adminAccountSortRateMultiplier):
		return adminAccountSortRateMultiplier
	case string(adminAccountSortLastUsedAt):
		return adminAccountSortLastUsedAt
	case string(adminAccountSortExpiresAt):
		return adminAccountSortExpiresAt
	default:
		return ""
	}
}

func normalizeAdminAccountSortOrder(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "desc") {
		return "desc"
	}
	return "asc"
}

func (s *adminServiceImpl) listAllAccountsForSort(ctx context.Context, platform, accountType, status, search string, groupID int64) ([]Account, error) {
	page := 1
	all := make([]Account, 0)
	for {
		items, result, err := s.accountRepo.ListWithFilters(ctx, pagination.PaginationParams{Page: page, PageSize: adminAccountSortFetchPageSize}, platform, accountType, status, search, groupID)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) == 0 || result == nil || int64(len(all)) >= result.Total {
			break
		}
		page++
	}
	return all, nil
}

func paginateSortedAccounts(accounts []Account, page, pageSize int) []Account {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	start := (page - 1) * pageSize
	if start >= len(accounts) {
		return []Account{}
	}
	end := start + pageSize
	if end > len(accounts) {
		end = len(accounts)
	}
	return append([]Account(nil), accounts[start:end]...)
}

func sortAdminAccounts(accounts []Account, sortKey adminAccountSortKey, sortOrder string) {
	if len(accounts) < 2 || sortKey == "" {
		return
	}
	order := normalizeAdminAccountSortOrder(sortOrder)
	now := time.Now()
	sort.SliceStable(accounts, func(i, j int) bool {
		cmp := compareAdminAccounts(accounts[i], accounts[j], sortKey, now)
		if order == "desc" {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareAdminAccounts(left, right Account, sortKey adminAccountSortKey, now time.Time) int {
	switch sortKey {
	case adminAccountSortName:
		return compareSortableStrings(left.Name, right.Name)
	case adminAccountSortStatus:
		return compareSortableStrings(left.Status, right.Status)
	case adminAccountSortSchedulable:
		return compareSortableBools(left.Schedulable, right.Schedulable)
	case adminAccountSortUsage:
		return compareSortableInt64(adminAccountUsageSortValue(&left, now), adminAccountUsageSortValue(&right, now))
	case adminAccountSortPriority:
		return compareSortableInts(left.Priority, right.Priority)
	case adminAccountSortRateMultiplier:
		return compareSortableFloat64(left.BillingRateMultiplier(), right.BillingRateMultiplier())
	case adminAccountSortLastUsedAt:
		return compareSortableTimes(left.LastUsedAt, right.LastUsedAt)
	case adminAccountSortExpiresAt:
		return compareSortableTimes(left.ExpiresAt, right.ExpiresAt)
	default:
		return 0
	}
}

func compareSortableStrings(left, right string) int {
	left = strings.TrimSpace(strings.ToLower(left))
	right = strings.TrimSpace(strings.ToLower(right))
	if left == "" && right == "" {
		return 0
	}
	if left == "" {
		return 1
	}
	if right == "" {
		return -1
	}
	if left == right {
		return 0
	}
	if left < right {
		return -1
	}
	return 1
}

func compareSortableBools(left, right bool) int {
	leftNum := 0
	if left {
		leftNum = 1
	}
	rightNum := 0
	if right {
		rightNum = 1
	}
	return compareSortableInts(leftNum, rightNum)
}

func compareSortableInts(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareSortableInt64(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareSortableFloat64(left, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareSortableTimes(left, right *time.Time) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return 1
	}
	if right == nil {
		return -1
	}
	switch {
	case left.Before(*right):
		return -1
	case left.After(*right):
		return 1
	default:
		return 0
	}
}

func adminAccountUsageSortValue(account *Account, now time.Time) int64 {
	if account == nil {
		return buildAdminUsageSortValue(6, 0)
	}
	if account.Status == StatusError {
		return buildAdminUsageSortValue(6, 0)
	}
	if account.Status != StatusActive || !account.Schedulable || isAdminAccountExpiredForScheduling(account, now) {
		return buildAdminUsageSortValue(5, 0)
	}
	if hasFutureAdminTime(account.TempUnschedulableUntil, now) {
		return buildAdminUsageSortValue(4, remainingAdminSeconds(account.TempUnschedulableUntil, now))
	}
	if hasFutureAdminTime(account.OverloadUntil, now) {
		return buildAdminUsageSortValue(4, 100_000+remainingAdminSeconds(account.OverloadUntil, now))
	}
	if hasFutureAdminTime(account.RateLimitResetAt, now) {
		return buildAdminUsageSortValue(4, 200_000+remainingAdminSeconds(account.RateLimitResetAt, now))
	}
	if account.SessionWindowStatus == "rejected" && hasFutureAdminTime(account.SessionWindowEnd, now) {
		return buildAdminUsageSortValue(4, 300_000+remainingAdminSeconds(account.SessionWindowEnd, now))
	}
	if used, ok := resolveAdminSessionWindowUsedPercent(account); ok {
		detail := int64(math.Round(used * 1000))
		if account.SessionWindowStatus == "allowed_warning" {
			return buildAdminUsageSortValue(2, detail)
		}
		return buildAdminUsageSortValue(0, detail)
	}
	if used, ok := resolveAdminOpenAIUsedPercent(account, now); ok {
		return buildAdminUsageSortValue(1, int64(math.Round(used*1000)))
	}
	return buildAdminUsageSortValue(3, 0)
}

func buildAdminUsageSortValue(category int64, detail int64) int64 {
	return category*adminUsageSortCategorySpan + detail
}

func isAdminAccountExpiredForScheduling(account *Account, now time.Time) bool {
	if account == nil || !account.AutoPauseOnExpired || account.ExpiresAt == nil {
		return false
	}
	return !account.ExpiresAt.After(now)
}

func hasFutureAdminTime(value *time.Time, now time.Time) bool {
	return value != nil && value.After(now)
}

func remainingAdminSeconds(value *time.Time, now time.Time) int64 {
	if value == nil {
		return 0
	}
	remaining := int64(value.Sub(now).Seconds())
	if remaining < 0 {
		return 0
	}
	return remaining
}

func resolveAdminSessionWindowUsedPercent(account *Account) (float64, bool) {
	if account == nil {
		return 0, false
	}
	if utilization, ok := adminMapFloat(account.Extra, "session_window_utilization"); ok {
		if utilization <= 1 {
			utilization *= 100
		}
		return clampAdminPercent(utilization), true
	}
	switch account.SessionWindowStatus {
	case "rejected":
		return 100, true
	case "allowed_warning":
		return 85, true
	case "allowed":
		return 0, true
	default:
		return 0, false
	}
}

func resolveAdminOpenAIUsedPercent(account *Account, now time.Time) (float64, bool) {
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return 0, false
	}
	fiveHourUsed, okFiveHour := resolveAdminCodexUsageWindow(account.Extra, "5h", now)
	sevenDayUsed, okSevenDay := resolveAdminCodexUsageWindow(account.Extra, "7d", now)
	if !okFiveHour && !okSevenDay {
		return 0, false
	}
	if !okFiveHour {
		return sevenDayUsed, true
	}
	if !okSevenDay {
		return fiveHourUsed, true
	}
	if fiveHourUsed >= sevenDayUsed {
		return fiveHourUsed, true
	}
	return sevenDayUsed, true
}

func resolveAdminCodexUsageWindow(extra map[string]any, window string, now time.Time) (float64, bool) {
	if len(extra) == 0 {
		return 0, false
	}
	var usedPercent float64
	var usedOK bool
	var resetAfterSeconds float64
	var resetAfterOK bool
	var resetAt *time.Time

	switch window {
	case "5h":
		usedPercent, usedOK = adminMapFloat(extra, "codex_5h_used_percent")
		resetAfterSeconds, resetAfterOK = adminMapFloat(extra, "codex_5h_reset_after_seconds")
		resetAt = adminMapTime(extra, "codex_5h_reset_at")
		if !usedOK || (!resetAfterOK && resetAt == nil) {
			legacyUsed, legacyResetAfter := resolveAdminLegacyCodexWindow(extra, true)
			if !usedOK && legacyUsed != nil {
				usedPercent = *legacyUsed
				usedOK = true
			}
			if !resetAfterOK && legacyResetAfter != nil {
				resetAfterSeconds = *legacyResetAfter
				resetAfterOK = true
			}
		}
	case "7d":
		usedPercent, usedOK = adminMapFloat(extra, "codex_7d_used_percent")
		resetAfterSeconds, resetAfterOK = adminMapFloat(extra, "codex_7d_reset_after_seconds")
		resetAt = adminMapTime(extra, "codex_7d_reset_at")
		if !usedOK || (!resetAfterOK && resetAt == nil) {
			legacyUsed, legacyResetAfter := resolveAdminLegacyCodexWindow(extra, false)
			if !usedOK && legacyUsed != nil {
				usedPercent = *legacyUsed
				usedOK = true
			}
			if !resetAfterOK && legacyResetAfter != nil {
				resetAfterSeconds = *legacyResetAfter
				resetAfterOK = true
			}
		}
	default:
		return 0, false
	}

	if resetAt == nil && resetAfterOK {
		resetAt = resolveAdminCodexResetAt(extra, resetAfterSeconds, now)
	}
	if usedOK && resetAt != nil && !resetAt.After(now) {
		return 0, true
	}
	if !usedOK {
		return 0, false
	}
	return clampAdminPercent(usedPercent), true
}

func resolveAdminLegacyCodexWindow(extra map[string]any, wantFiveHour bool) (*float64, *float64) {
	primaryWindow, primaryWindowOK := adminMapFloat(extra, "codex_primary_window_minutes")
	secondaryWindow, secondaryWindowOK := adminMapFloat(extra, "codex_secondary_window_minutes")
	primaryUsed, primaryUsedOK := adminMapFloat(extra, "codex_primary_used_percent")
	secondaryUsed, secondaryUsedOK := adminMapFloat(extra, "codex_secondary_used_percent")
	primaryReset, primaryResetOK := adminMapFloat(extra, "codex_primary_reset_after_seconds")
	secondaryReset, secondaryResetOK := adminMapFloat(extra, "codex_secondary_reset_after_seconds")

	pick := func(used float64, usedOK bool, reset float64, resetOK bool) (*float64, *float64) {
		var usedPtr *float64
		var resetPtr *float64
		if usedOK {
			usedCopy := used
			usedPtr = &usedCopy
		}
		if resetOK {
			resetCopy := reset
			resetPtr = &resetCopy
		}
		return usedPtr, resetPtr
	}

	if wantFiveHour {
		if primaryWindowOK && primaryWindow <= 360 {
			return pick(primaryUsed, primaryUsedOK, primaryReset, primaryResetOK)
		}
		if secondaryWindowOK && secondaryWindow <= 360 {
			return pick(secondaryUsed, secondaryUsedOK, secondaryReset, secondaryResetOK)
		}
		return pick(secondaryUsed, secondaryUsedOK, secondaryReset, secondaryResetOK)
	}

	if primaryWindowOK && primaryWindow >= 10000 {
		return pick(primaryUsed, primaryUsedOK, primaryReset, primaryResetOK)
	}
	if secondaryWindowOK && secondaryWindow >= 10000 {
		return pick(secondaryUsed, secondaryUsedOK, secondaryReset, secondaryResetOK)
	}
	return pick(primaryUsed, primaryUsedOK, primaryReset, primaryResetOK)
}

func resolveAdminCodexResetAt(extra map[string]any, resetAfterSeconds float64, now time.Time) *time.Time {
	base := now
	if parsed := adminMapTime(extra, "codex_usage_updated_at"); parsed != nil {
		base = *parsed
	}
	if resetAfterSeconds < 0 {
		resetAfterSeconds = 0
	}
	resetAt := base.Add(time.Duration(resetAfterSeconds * float64(time.Second)))
	return &resetAt
}

func clampAdminPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func adminMapFloat(values map[string]any, key string) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	value, ok := values[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func adminMapTime(values map[string]any, key string) *time.Time {
	if len(values) == 0 {
		return nil
	}
	value, ok := values[key]
	if !ok || value == nil {
		return nil
	}
	trimmed, ok := value.(string)
	if !ok {
		return nil
	}
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339Nano, trimmed)
		if err != nil {
			return nil
		}
	}
	return &parsed
}
