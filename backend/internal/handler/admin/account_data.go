package admin

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	dataType       = "sub2api-data"
	legacyDataType = "sub2api-bundle"
	dataVersion    = 1
	dataPageCap    = 1000
)

type DataPayload struct {
	Type       string        `json:"type,omitempty"`
	Version    int           `json:"version,omitempty"`
	ExportedAt string        `json:"exported_at"`
	Proxies    []DataProxy   `json:"proxies"`
	Accounts   []DataAccount `json:"accounts"`
}

type DataProxy struct {
	ProxyKey string `json:"proxy_key"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Status   string `json:"status"`
}

type DataAccount struct {
	Name               string         `json:"name"`
	Notes              *string        `json:"notes,omitempty"`
	Platform           string         `json:"platform"`
	Type               string         `json:"type"`
	Credentials        map[string]any `json:"credentials"`
	Extra              map[string]any `json:"extra,omitempty"`
	ProxyKey           *string        `json:"proxy_key,omitempty"`
	Concurrency        int            `json:"concurrency"`
	Priority           int            `json:"priority"`
	RateMultiplier     *float64       `json:"rate_multiplier,omitempty"`
	ExpiresAt          *int64         `json:"expires_at,omitempty"`
	AutoPauseOnExpired *bool          `json:"auto_pause_on_expired,omitempty"`
}

type DataImportRequest struct {
	Data                 DataPayload `json:"data"`
	SkipDefaultGroupBind *bool       `json:"skip_default_group_bind"`
	GroupIDs             []int64     `json:"group_ids"`
}

type DataImportResult struct {
	ProxyCreated   int               `json:"proxy_created"`
	ProxyReused    int               `json:"proxy_reused"`
	ProxyFailed    int               `json:"proxy_failed"`
	AccountCreated int               `json:"account_created"`
	AccountUpdated int               `json:"account_updated"`
	AccountFailed  int               `json:"account_failed"`
	Errors         []DataImportError `json:"errors,omitempty"`
}

type DataImportError struct {
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
	ProxyKey string `json:"proxy_key,omitempty"`
	Message  string `json:"message"`
}

func buildProxyKey(protocol, host string, port int, username, password string) string {
	return fmt.Sprintf("%s|%s|%d|%s|%s", strings.TrimSpace(protocol), strings.TrimSpace(host), port, strings.TrimSpace(username), strings.TrimSpace(password))
}

func (h *AccountHandler) ExportData(c *gin.Context) {
	ctx := c.Request.Context()

	selectedIDs, err := parseAccountIDs(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	accounts, err := h.resolveExportAccounts(ctx, selectedIDs, c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	includeProxies, err := parseIncludeProxies(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var proxies []service.Proxy
	if includeProxies {
		proxies, err = h.resolveExportProxies(ctx, accounts)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	} else {
		proxies = []service.Proxy{}
	}

	proxyKeyByID := make(map[int64]string, len(proxies))
	dataProxies := make([]DataProxy, 0, len(proxies))
	for i := range proxies {
		p := proxies[i]
		key := buildProxyKey(p.Protocol, p.Host, p.Port, p.Username, p.Password)
		proxyKeyByID[p.ID] = key
		dataProxies = append(dataProxies, DataProxy{
			ProxyKey: key,
			Name:     p.Name,
			Protocol: p.Protocol,
			Host:     p.Host,
			Port:     p.Port,
			Username: p.Username,
			Password: p.Password,
			Status:   p.Status,
		})
	}

	dataAccounts := make([]DataAccount, 0, len(accounts))
	for i := range accounts {
		acc := accounts[i]
		var proxyKey *string
		if acc.ProxyID != nil {
			if key, ok := proxyKeyByID[*acc.ProxyID]; ok {
				proxyKey = &key
			}
		}
		var expiresAt *int64
		if acc.ExpiresAt != nil {
			v := acc.ExpiresAt.Unix()
			expiresAt = &v
		}
		dataAccounts = append(dataAccounts, DataAccount{
			Name:               acc.Name,
			Notes:              acc.Notes,
			Platform:           acc.Platform,
			Type:               acc.Type,
			Credentials:        acc.Credentials,
			Extra:              acc.Extra,
			ProxyKey:           proxyKey,
			Concurrency:        acc.Concurrency,
			Priority:           acc.Priority,
			RateMultiplier:     acc.RateMultiplier,
			ExpiresAt:          expiresAt,
			AutoPauseOnExpired: &acc.AutoPauseOnExpired,
		})
	}

	payload := DataPayload{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Proxies:    dataProxies,
		Accounts:   dataAccounts,
	}

	response.Success(c, payload)
}

func (h *AccountHandler) ImportData(c *gin.Context) {
	var req DataImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := validateDataHeader(req.Data); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_data", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importData(ctx, req)
	})
}

func (h *AccountHandler) importData(ctx context.Context, req DataImportRequest) (DataImportResult, error) {
	skipDefaultGroupBind := true
	if req.SkipDefaultGroupBind != nil {
		skipDefaultGroupBind = *req.SkipDefaultGroupBind
	}

	dataPayload := req.Data
	result := DataImportResult{}

	existingProxies, err := h.listAllProxies(ctx)
	if err != nil {
		return result, err
	}

	existingAccounts, err := h.listAccountsFiltered(ctx, "", "", "", "")
	if err != nil {
		return result, err
	}

	proxyKeyToID := make(map[string]int64, len(existingProxies))
	for i := range existingProxies {
		p := existingProxies[i]
		key := buildProxyKey(p.Protocol, p.Host, p.Port, p.Username, p.Password)
		proxyKeyToID[key] = p.ID
	}

	accountByID := make(map[int64]service.Account, len(existingAccounts))
	accountIdentityIndex := make(map[string]int64, len(existingAccounts)*4)
	for i := range existingAccounts {
		account := existingAccounts[i]
		accountByID[account.ID] = account
		addAccountIdentityKeys(accountIdentityIndex, account)
	}

	for i := range dataPayload.Proxies {
		item := dataPayload.Proxies[i]
		key := item.ProxyKey
		if key == "" {
			key = buildProxyKey(item.Protocol, item.Host, item.Port, item.Username, item.Password)
		}
		if err := validateDataProxy(item); err != nil {
			result.ProxyFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:     "proxy",
				Name:     item.Name,
				ProxyKey: key,
				Message:  err.Error(),
			})
			continue
		}
		normalizedStatus := normalizeProxyStatus(item.Status)
		if existingID, ok := proxyKeyToID[key]; ok {
			proxyKeyToID[key] = existingID
			result.ProxyReused++
			if normalizedStatus != "" {
				if proxy, getErr := h.adminService.GetProxy(ctx, existingID); getErr == nil && proxy != nil && proxy.Status != normalizedStatus {
					_, _ = h.adminService.UpdateProxy(ctx, existingID, &service.UpdateProxyInput{
						Status: normalizedStatus,
					})
				}
			}
			continue
		}

		created, createErr := h.adminService.CreateProxy(ctx, &service.CreateProxyInput{
			Name:     defaultProxyName(item.Name),
			Protocol: item.Protocol,
			Host:     item.Host,
			Port:     item.Port,
			Username: item.Username,
			Password: item.Password,
		})
		if createErr != nil {
			result.ProxyFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:     "proxy",
				Name:     item.Name,
				ProxyKey: key,
				Message:  createErr.Error(),
			})
			continue
		}
		proxyKeyToID[key] = created.ID
		result.ProxyCreated++

		if normalizedStatus != "" && normalizedStatus != created.Status {
			_, _ = h.adminService.UpdateProxy(ctx, created.ID, &service.UpdateProxyInput{
				Status: normalizedStatus,
			})
		}
	}

	for i := range dataPayload.Accounts {
		item := dataPayload.Accounts[i]
		if err := validateDataAccount(item); err != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:    "account",
				Name:    item.Name,
				Message: err.Error(),
			})
			continue
		}

		var proxyID *int64
		if item.ProxyKey != nil && *item.ProxyKey != "" {
			if id, ok := proxyKeyToID[*item.ProxyKey]; ok {
				proxyID = &id
			} else {
				result.AccountFailed++
				result.Errors = append(result.Errors, DataImportError{
					Kind:     "account",
					Name:     item.Name,
					ProxyKey: *item.ProxyKey,
					Message:  "proxy_key not found",
				})
				continue
			}
		}

		duplicateID, ambiguousMatch := findMatchingImportedAccount(accountIdentityIndex, item)
		if duplicateID > 0 {
			existing := accountByID[duplicateID]
			updateInput := buildDuplicateAccountUpdateInput(existing, item, proxyID, req.GroupIDs)
			if _, err := h.adminService.UpdateAccount(ctx, duplicateID, updateInput); err != nil {
				result.AccountFailed++
				result.Errors = append(result.Errors, DataImportError{
					Kind:    "account",
					Name:    item.Name,
					Message: err.Error(),
				})
				continue
			}
			result.AccountUpdated++
			updatedSnapshot := applyImportedDuplicateAccountUpdate(existing, item, proxyID, req.GroupIDs)
			accountByID[duplicateID] = updatedSnapshot
			addAccountIdentityKeys(accountIdentityIndex, updatedSnapshot)
			continue
		}
		if ambiguousMatch {
			result.AccountFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:    "account",
				Name:    item.Name,
				Message: "multiple existing accounts matched this import; please clean up duplicates first",
			})
			continue
		}

		accountInput := &service.CreateAccountInput{
			Name:                 item.Name,
			Notes:                item.Notes,
			Platform:             item.Platform,
			Type:                 item.Type,
			Credentials:          item.Credentials,
			Extra:                item.Extra,
			ProxyID:              proxyID,
			Concurrency:          item.Concurrency,
			Priority:             item.Priority,
			RateMultiplier:       item.RateMultiplier,
			GroupIDs:             append([]int64(nil), req.GroupIDs...),
			ExpiresAt:            item.ExpiresAt,
			AutoPauseOnExpired:   item.AutoPauseOnExpired,
			SkipDefaultGroupBind: skipDefaultGroupBind,
		}

		created, err := h.adminService.CreateAccount(ctx, accountInput)
		if err != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, DataImportError{
				Kind:    "account",
				Name:    item.Name,
				Message: err.Error(),
			})
			continue
		}
		result.AccountCreated++

		createdSnapshot := buildImportedAccountSnapshot(created.ID, item, proxyID, req.GroupIDs)
		accountByID[created.ID] = createdSnapshot
		addAccountIdentityKeys(accountIdentityIndex, createdSnapshot)
	}

	return result, nil
}

func (h *AccountHandler) listAllProxies(ctx context.Context) ([]service.Proxy, error) {
	page := 1
	pageSize := dataPageCap
	var out []service.Proxy
	for {
		items, total, err := h.adminService.ListProxies(ctx, page, pageSize, "", "", "")
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(out) >= int(total) || len(items) == 0 {
			break
		}
		page++
	}
	return out, nil
}

func (h *AccountHandler) listAccountsFiltered(ctx context.Context, platform, accountType, status, search string) ([]service.Account, error) {
	page := 1
	pageSize := dataPageCap
	var out []service.Account
	for {
		items, total, err := h.adminService.ListAccounts(ctx, page, pageSize, platform, accountType, status, search, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(out) >= int(total) || len(items) == 0 {
			break
		}
		page++
	}
	return out, nil
}

func (h *AccountHandler) resolveExportAccounts(ctx context.Context, ids []int64, c *gin.Context) ([]service.Account, error) {
	if len(ids) > 0 {
		accounts, err := h.adminService.GetAccountsByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		out := make([]service.Account, 0, len(accounts))
		for _, acc := range accounts {
			if acc == nil {
				continue
			}
			out = append(out, *acc)
		}
		return out, nil
	}

	platform := c.Query("platform")
	accountType := c.Query("type")
	status := c.Query("status")
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		search = search[:100]
	}
	return h.listAccountsFiltered(ctx, platform, accountType, status, search)
}

func (h *AccountHandler) resolveExportProxies(ctx context.Context, accounts []service.Account) ([]service.Proxy, error) {
	if len(accounts) == 0 {
		return []service.Proxy{}, nil
	}

	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for i := range accounts {
		if accounts[i].ProxyID == nil {
			continue
		}
		id := *accounts[i].ProxyID
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return []service.Proxy{}, nil
	}

	return h.adminService.GetProxiesByIDs(ctx, ids)
}

func buildImportedAccountSnapshot(id int64, item DataAccount, proxyID *int64, groupIDs []int64) service.Account {
	snapshot := service.Account{
		ID:          id,
		Name:        item.Name,
		Notes:       item.Notes,
		Platform:    item.Platform,
		Type:        item.Type,
		Credentials: cloneStringAnyMap(item.Credentials),
		Extra:       cloneStringAnyMap(item.Extra),
		ProxyID:     cloneInt64Ptr(proxyID),
		Concurrency: item.Concurrency,
		Priority:    item.Priority,
		GroupIDs:    append([]int64(nil), groupIDs...),
	}
	if item.RateMultiplier != nil {
		v := *item.RateMultiplier
		snapshot.RateMultiplier = &v
	}
	if item.ExpiresAt != nil && *item.ExpiresAt > 0 {
		expiresAt := time.Unix(*item.ExpiresAt, 0)
		snapshot.ExpiresAt = &expiresAt
	}
	if item.AutoPauseOnExpired != nil {
		snapshot.AutoPauseOnExpired = *item.AutoPauseOnExpired
	}
	return snapshot
}

func buildDuplicateAccountUpdateInput(existing service.Account, item DataAccount, proxyID *int64, selectedGroupIDs []int64) *service.UpdateAccountInput {
	mergedCredentials := mergeStringAnyMap(existing.Credentials, item.Credentials)
	mergedExtra := mergeStringAnyMap(existing.Extra, item.Extra)
	input := &service.UpdateAccountInput{
		Credentials: mergedCredentials,
	}
	if len(mergedExtra) > 0 {
		input.Extra = mergedExtra
	}
	if item.Notes != nil {
		input.Notes = item.Notes
	}
	if item.ExpiresAt != nil {
		input.ExpiresAt = item.ExpiresAt
	}
	if item.AutoPauseOnExpired != nil {
		input.AutoPauseOnExpired = item.AutoPauseOnExpired
	}
	if proxyUpdate := buildProxyIDUpdate(item, proxyID); proxyUpdate != nil {
		input.ProxyID = proxyUpdate
	}
	if len(selectedGroupIDs) > 0 {
		mergedGroups := mergeGroupIDs(existing.GroupIDs, selectedGroupIDs)
		input.GroupIDs = &mergedGroups
	}
	return input
}

func applyImportedDuplicateAccountUpdate(existing service.Account, item DataAccount, proxyID *int64, selectedGroupIDs []int64) service.Account {
	updated := existing
	updated.Credentials = mergeStringAnyMap(existing.Credentials, item.Credentials)
	updated.Extra = mergeStringAnyMap(existing.Extra, item.Extra)
	if item.Notes != nil {
		updated.Notes = item.Notes
	}
	if proxyUpdate := buildProxyIDUpdate(item, proxyID); proxyUpdate != nil {
		if *proxyUpdate == 0 {
			updated.ProxyID = nil
		} else {
			updated.ProxyID = cloneInt64Ptr(proxyUpdate)
		}
	}
	if item.ExpiresAt != nil {
		if *item.ExpiresAt <= 0 {
			updated.ExpiresAt = nil
		} else {
			expiresAt := time.Unix(*item.ExpiresAt, 0)
			updated.ExpiresAt = &expiresAt
		}
	}
	if item.AutoPauseOnExpired != nil {
		updated.AutoPauseOnExpired = *item.AutoPauseOnExpired
	}
	if len(selectedGroupIDs) > 0 {
		updated.GroupIDs = mergeGroupIDs(existing.GroupIDs, selectedGroupIDs)
	}
	return updated
}

func buildProxyIDUpdate(item DataAccount, proxyID *int64) *int64 {
	if item.ProxyKey == nil {
		return nil
	}
	if strings.TrimSpace(*item.ProxyKey) == "" {
		zero := int64(0)
		return &zero
	}
	return cloneInt64Ptr(proxyID)
}

func mergeStringAnyMap(base map[string]any, patch map[string]any) map[string]any {
	if len(base) == 0 && len(patch) == 0 {
		return nil
	}
	merged := make(map[string]any, len(base)+len(patch))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range patch {
		merged[k] = v
	}
	return merged
}

func cloneStringAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func mergeGroupIDs(existing []int64, selected []int64) []int64 {
	merged := make([]int64, 0, len(existing)+len(selected))
	seen := make(map[int64]struct{}, len(existing)+len(selected))
	for _, id := range existing {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		merged = append(merged, id)
	}
	for _, id := range selected {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		merged = append(merged, id)
	}
	return merged
}

func findMatchingImportedAccount(identityIndex map[string]int64, item DataAccount) (int64, bool) {
	ambiguous := false
	for _, key := range buildAccountIdentityKeys(item.Platform, item.Type, item.Name, item.Credentials, item.Extra) {
		id, ok := identityIndex[key]
		if !ok {
			continue
		}
		if id > 0 {
			return id, false
		}
		ambiguous = true
	}
	return 0, ambiguous
}

func addAccountIdentityKeys(identityIndex map[string]int64, account service.Account) {
	for _, key := range buildAccountIdentityKeys(account.Platform, account.Type, account.Name, account.Credentials, account.Extra) {
		if key == "" {
			continue
		}
		if existingID, ok := identityIndex[key]; ok {
			if existingID != account.ID {
				identityIndex[key] = 0
			}
			continue
		}
		identityIndex[key] = account.ID
	}
}

func buildAccountIdentityKeys(platform, accountType, name string, credentials, extra map[string]any) []string {
	prefix := strings.ToLower(strings.TrimSpace(platform)) + "|" + strings.ToLower(strings.TrimSpace(accountType)) + "|"
	keys := make([]string, 0, 12)
	seen := make(map[string]struct{}, 12)
	addKey := func(scope, key string, fold bool, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if fold {
			value = strings.ToLower(value)
		}
		identityKey := prefix + scope + ":" + key + ":" + value
		if _, ok := seen[identityKey]; ok {
			return
		}
		seen[identityKey] = struct{}{}
		keys = append(keys, identityKey)
	}

	addKey("credentials", "chatgpt_account_id", false, lookupAnyString(credentials, "chatgpt_account_id"))
	addKey("credentials", "chatgpt_user_id", false, lookupAnyString(credentials, "chatgpt_user_id"))
	addKey("extra", "crs_account_id", false, lookupAnyString(extra, "crs_account_id"))
	addKey("credentials", "project_id", false, lookupAnyString(credentials, "project_id"))
	addKey("credentials", "account_uuid", false, lookupAnyString(credentials, "account_uuid"))
	addKey("credentials", "org_uuid", false, lookupAnyString(credentials, "org_uuid"))
	addKey("credentials", "refresh_token", false, lookupAnyString(credentials, "refresh_token"))
	addKey("credentials", "api_key", false, lookupAnyString(credentials, "api_key"))
	addKey("credentials", "session_key", false, lookupAnyString(credentials, "session_key"))
	addKey("credentials", "token", false, lookupAnyString(credentials, "token"))
	addKey("credentials", "access_token", false, lookupAnyString(credentials, "access_token"))
	addKey("extra", "email", true, lookupAnyString(extra, "email"))
	addKey("account", "name", true, name)

	return keys
}

func lookupAnyString(record map[string]any, key string) string {
	if len(record) == 0 {
		return ""
	}
	value, ok := record[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case float32:
		return strconv.FormatInt(int64(typed), 10)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	default:
		return ""
	}
}

func parseAccountIDs(c *gin.Context) ([]int64, error) {
	values := c.QueryArray("ids")
	if len(values) == 0 {
		raw := strings.TrimSpace(c.Query("ids"))
		if raw != "" {
			values = []string{raw}
		}
	}
	if len(values) == 0 {
		return nil, nil
	}

	ids := make([]int64, 0, len(values))
	for _, item := range values {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil || id <= 0 {
				return nil, fmt.Errorf("invalid account id: %s", part)
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func parseIncludeProxies(c *gin.Context) (bool, error) {
	raw := strings.TrimSpace(strings.ToLower(c.Query("include_proxies")))
	if raw == "" {
		return true, nil
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return true, fmt.Errorf("invalid include_proxies value: %s", raw)
	}
}

func validateDataHeader(payload DataPayload) error {
	if payload.Type != "" && payload.Type != dataType && payload.Type != legacyDataType {
		return fmt.Errorf("unsupported data type: %s", payload.Type)
	}
	if payload.Version != 0 && payload.Version != dataVersion {
		return fmt.Errorf("unsupported data version: %d", payload.Version)
	}
	if payload.Proxies == nil {
		return errors.New("proxies is required")
	}
	if payload.Accounts == nil {
		return errors.New("accounts is required")
	}
	return nil
}

func validateDataProxy(item DataProxy) error {
	if strings.TrimSpace(item.Protocol) == "" {
		return errors.New("proxy protocol is required")
	}
	if strings.TrimSpace(item.Host) == "" {
		return errors.New("proxy host is required")
	}
	if item.Port <= 0 || item.Port > 65535 {
		return errors.New("proxy port is invalid")
	}
	switch item.Protocol {
	case "http", "https", "socks5", "socks5h":
	default:
		return fmt.Errorf("proxy protocol is invalid: %s", item.Protocol)
	}
	if item.Status != "" {
		normalizedStatus := normalizeProxyStatus(item.Status)
		if normalizedStatus != service.StatusActive && normalizedStatus != "inactive" {
			return fmt.Errorf("proxy status is invalid: %s", item.Status)
		}
	}
	return nil
}

func validateDataAccount(item DataAccount) error {
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("account name is required")
	}
	if strings.TrimSpace(item.Platform) == "" {
		return errors.New("account platform is required")
	}
	if strings.TrimSpace(item.Type) == "" {
		return errors.New("account type is required")
	}
	if len(item.Credentials) == 0 {
		return errors.New("account credentials is required")
	}
	switch item.Type {
	case service.AccountTypeOAuth, service.AccountTypeSetupToken, service.AccountTypeAPIKey, service.AccountTypeUpstream:
	default:
		return fmt.Errorf("account type is invalid: %s", item.Type)
	}
	if item.RateMultiplier != nil && *item.RateMultiplier < 0 {
		return errors.New("rate_multiplier must be >= 0")
	}
	if item.Concurrency < 0 {
		return errors.New("concurrency must be >= 0")
	}
	if item.Priority < 0 {
		return errors.New("priority must be >= 0")
	}
	return nil
}

func defaultProxyName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "imported-proxy"
	}
	return name
}

func normalizeProxyStatus(status string) string {
	normalized := strings.TrimSpace(strings.ToLower(status))
	switch normalized {
	case "":
		return ""
	case service.StatusActive:
		return service.StatusActive
	case "inactive", service.StatusDisabled:
		return "inactive"
	default:
		return normalized
	}
}
