package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/sub2api/dfswitch/internal/adapter"
	"github.com/sub2api/dfswitch/internal/client"
	"github.com/sub2api/dfswitch/internal/service"
	"github.com/sub2api/dfswitch/internal/store"
)

// App is the Wails binding target. Every exported method becomes
// window.go.main.App.<Method>() in the frontend. Return values are
// marshalled to JSON and the returned error is surfaced as a JS Promise
// rejection. Names mirror the old HTTP routes 1:1 to minimise frontend churn.
type App struct {
	ctx     context.Context
	cfg     *store.Config
	syncSvc *service.SyncService
}

func NewApp() *App {
	cfg, err := store.Load()
	if err != nil {
		cfg = store.NewDefault()
	}
	return &App{
		cfg:     cfg,
		syncSvc: service.NewSyncService(cfg),
	}
}

// Startup is invoked by Wails once the window/context is ready.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// --- auth -----------------------------------------------------------------

// LoginResult mirrors the HTTP /api/login response. Either RequiresTFA is
// true (and TempToken/UserEmailMasked are set) or Ok is true and User is
// populated. Keeping both branches in one struct so TS gets a single type.
type LoginResult struct {
	Ok              bool         `json:"ok"`
	RequiresTFA     bool         `json:"requires_2fa"`
	TempToken       string       `json:"temp_token,omitempty"`
	UserEmailMasked string       `json:"user_email_masked,omitempty"`
	User            *client.User `json:"user,omitempty"`
}

func (a *App) Login(email, password, serverURL string) (*LoginResult, error) {
	if email == "" || password == "" {
		return nil, errors.New("请填写邮箱和密码")
	}
	if serverURL != "" {
		a.cfg.SetServerURL(serverURL)
		_ = a.cfg.Save()
	}
	if a.cfg.GetServerURL() == "" {
		return nil, errors.New("请配置服务器地址")
	}

	cli := client.New(a.cfg)
	r, err := cli.Login(a.ctx, email, password)
	if err != nil {
		return nil, translateClientError(err)
	}
	if r.RequiresTFA {
		return &LoginResult{
			RequiresTFA:     true,
			TempToken:       r.TempToken,
			UserEmailMasked: r.UserEmailMasked,
		}, nil
	}
	return &LoginResult{Ok: true, User: r.User}, nil
}

func (a *App) Login2FA(tempToken, totpCode string) (*LoginResult, error) {
	if tempToken == "" || totpCode == "" {
		return nil, errors.New("请填写临时令牌和验证码")
	}
	if a.cfg.GetServerURL() == "" {
		return nil, errors.New("请配置服务器地址")
	}
	cli := client.New(a.cfg)
	r, err := cli.Login2FA(a.ctx, tempToken, totpCode)
	if err != nil {
		return nil, translateClientError(err)
	}
	return &LoginResult{Ok: true, User: r.User}, nil
}

func (a *App) Logout() error {
	cli := client.New(a.cfg)
	cli.Logout(a.ctx)
	return nil
}

type AuthStatus struct {
	LoggedIn  bool         `json:"logged_in"`
	ServerURL string       `json:"server_url"`
	User      *client.User `json:"user,omitempty"`
}

func (a *App) AuthStatus() (*AuthStatus, error) {
	token := a.cfg.GetAccessToken()
	out := &AuthStatus{
		LoggedIn:  token != "",
		ServerURL: a.cfg.GetServerURL(),
	}
	if token == "" {
		return out, nil
	}
	cli := client.New(a.cfg)
	u, err := cli.GetMe(a.ctx)
	if err != nil {
		if errors.Is(err, client.ErrUnauthorized) {
			out.LoggedIn = false
			return out, nil
		}
		return out, translateClientError(err)
	}
	out.User = u
	return out, nil
}

// --- keys -----------------------------------------------------------------

// KeyItem is the enriched key shape the frontend expects (adds
// suggested_base_url and flattens group fields). Matches the old
// /api/keys response item-for-item.
type KeyItem struct {
	ID               int64  `json:"id"`
	Key              string `json:"key"`
	Name             string `json:"name"`
	GroupID          int64  `json:"group_id"`
	GroupName        string `json:"group_name"`
	Platform         string `json:"platform"`
	SubscriptionType string `json:"subscription_type"`
	Status           string `json:"status"`
	Quota            any    `json:"quota"`
	QuotaUsed        any    `json:"quota_used"`
	RateLimit1d      any    `json:"rate_limit_1d"`
	Usage1d          any    `json:"usage_1d"`
	Concurrency      int    `json:"concurrency"`
	ExpiresAt        string `json:"expires_at"`
	LastUsedAt       string `json:"last_used_at"`
	SuggestedBaseURL string `json:"suggested_base_url"`
}

func (a *App) ListKeys() ([]KeyItem, error) {
	cli := client.New(a.cfg)
	keys, err := cli.ListKeys(a.ctx)
	if err != nil {
		return nil, translateClientError(err)
	}
	serverURL := a.cfg.GetServerURL()
	out := make([]KeyItem, 0, len(keys))
	for _, k := range keys {
		platform, groupName, subType := "", "", ""
		if k.Group != nil {
			platform = k.Group.Platform
			groupName = k.Group.Name
			subType = k.Group.SubscriptionType
		}
		out = append(out, KeyItem{
			ID:               k.ID,
			Key:              k.Key,
			Name:             k.Name,
			GroupID:          k.GroupID,
			GroupName:        groupName,
			Platform:         platform,
			SubscriptionType: subType,
			Status:           k.Status,
			Quota:            k.Quota,
			QuotaUsed:        k.QuotaUsed,
			RateLimit1d:      k.RateLimit1d,
			Usage1d:          k.Usage1d,
			Concurrency:      k.Concurrency,
			ExpiresAt:        k.ExpiresAt,
			LastUsedAt:       k.LastUsedAt,
			SuggestedBaseURL: adapter.GatewayURL(serverURL, platform),
		})
	}
	return out, nil
}

// --- tools ----------------------------------------------------------------

func (a *App) ListTools() ([]adapter.Tool, error) {
	adapters := adapter.AllAdapters()
	out := make([]adapter.Tool, 0, len(adapters))
	for _, ad := range adapters {
		out = append(out, adapter.Tool{
			ID:                 ad.ID(),
			Name:               ad.Name(),
			Description:        ad.Description(),
			Installed:          ad.Detect(),
			ConfigPath:         ad.ConfigFilePath(),
			SupportedPlatforms: ad.SupportedPlatforms(),
		})
	}
	return out, nil
}

// --- apply ----------------------------------------------------------------

// ApplyRequest is the frontend-facing payload: the caller picks either
// KeyID (backend resolves key value + platform + base URL) or supplies
// APIKey/BaseURL/Platform manually.
type ApplyRequest struct {
	ToolIDs  []string `json:"tool_ids"`
	KeyID    int64    `json:"key_id"`
	APIKey   string   `json:"api_key"`
	BaseURL  string   `json:"base_url"`
	Platform string   `json:"platform"`
}

type ApplyResult struct {
	ToolID   string   `json:"tool_id"`
	Success  bool     `json:"success"`
	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type UnsupportedTool struct {
	ToolID    string   `json:"tool_id"`
	Supported []string `json:"supported"`
	Reason    string   `json:"reason"`
}

// ApplyResponse folds both the success and platform-mismatch branches into
// one return shape — TypeScript can just check `unsupported.length > 0`.
type ApplyResponse struct {
	Results     []ApplyResult     `json:"results"`
	Successes   int               `json:"successes"`
	Failures    int               `json:"failures"`
	Unsupported []UnsupportedTool `json:"unsupported,omitempty"`
}

func (a *App) Apply(req ApplyRequest) (*ApplyResponse, error) {
	if len(req.ToolIDs) == 0 {
		return nil, errors.New("参数错误")
	}

	apiKey := req.APIKey
	baseURL := req.BaseURL
	platform := req.Platform

	if req.KeyID != 0 {
		cli := client.New(a.cfg)
		keys, err := cli.ListKeys(a.ctx)
		if err != nil {
			return nil, translateClientError(err)
		}
		var match *client.Key
		for i := range keys {
			if keys[i].ID == req.KeyID {
				match = &keys[i]
				break
			}
		}
		if match == nil {
			return nil, errors.New("未找到该 Key")
		}
		apiKey = match.Key
		if match.Group != nil {
			platform = match.Group.Platform
		}
		if baseURL == "" {
			baseURL = adapter.GatewayURL(a.cfg.GetServerURL(), platform)
		}
	}

	if apiKey == "" {
		return nil, errors.New("缺少 api_key")
	}

	adapters := adapter.AllAdapters()
	adapterMap := make(map[string]adapter.Adapter, len(adapters))
	for _, ad := range adapters {
		adapterMap[ad.ID()] = ad
	}

	// Pre-validate platform compatibility; reject the whole batch so we
	// never half-write.
	if platform != "" {
		var unsupported []UnsupportedTool
		for _, toolID := range req.ToolIDs {
			ad, ok := adapterMap[toolID]
			if !ok {
				continue
			}
			if !adapter.IsCompatible(ad.SupportedPlatforms(), platform) {
				unsupported = append(unsupported, UnsupportedTool{
					ToolID:    toolID,
					Supported: ad.SupportedPlatforms(),
					Reason:    fmt.Sprintf("工具 %s 不支持平台 %s", toolID, platform),
				})
			}
		}
		if len(unsupported) > 0 {
			// Don't wrap in error — Wails flattens errors to strings and we'd
			// lose the structured Unsupported list. Frontend checks
			// resp.unsupported.length instead.
			return &ApplyResponse{Unsupported: unsupported}, nil
		}
	}

	resp := &ApplyResponse{Results: make([]ApplyResult, 0, len(req.ToolIDs))}
	for _, toolID := range req.ToolIDs {
		ad, ok := adapterMap[toolID]
		if !ok {
			resp.Results = append(resp.Results, ApplyResult{ToolID: toolID, Success: false, Error: "未知工具"})
			resp.Failures++
			continue
		}
		if err := ad.Apply(adapter.ApplyRequest{APIKey: apiKey, BaseURL: baseURL, Platform: platform}); err != nil {
			resp.Results = append(resp.Results, ApplyResult{ToolID: toolID, Success: false, Error: fmt.Sprintf("写入失败: %v", err)})
			resp.Failures++
			continue
		}
		a.cfg.SetAppliedTool(toolID, apiKey, baseURL)

		entry := ApplyResult{ToolID: toolID, Success: true}
		if w, ok := ad.(interface{ Warnings() []string }); ok {
			if warnings := w.Warnings(); len(warnings) > 0 {
				entry.Warnings = warnings
			}
		}
		resp.Results = append(resp.Results, entry)
		resp.Successes++
	}
	_ = a.cfg.Save()
	return resp, nil
}

// --- sync -----------------------------------------------------------------

type SyncStatus struct {
	Running   bool   `json:"running"`
	LastSync  string `json:"last_sync"`
	LastError string `json:"last_error"`
	Interval  int    `json:"interval"`
}

func (a *App) SyncStart() error {
	a.syncSvc.Start()
	a.cfg.SyncEnabled = true
	_ = a.cfg.Save()
	return nil
}

func (a *App) SyncStop() error {
	a.syncSvc.Stop()
	a.cfg.SyncEnabled = false
	_ = a.cfg.Save()
	return nil
}

func (a *App) SyncStatus() (*SyncStatus, error) {
	return &SyncStatus{
		Running:   a.syncSvc.Running(),
		LastSync:  a.syncSvc.LastSync(),
		LastError: a.syncSvc.LastError(),
		Interval:  a.cfg.SyncInterval,
	}, nil
}

// --- settings -------------------------------------------------------------

type Settings struct {
	ServerURL    string `json:"server_url"`
	SyncEnabled  bool   `json:"sync_enabled"`
	SyncInterval int    `json:"sync_interval"`
}

type UpdateSettingsRequest struct {
	ServerURL            *string `json:"server_url,omitempty"`
	SyncIntervalMinutes  *int    `json:"sync_interval_minutes,omitempty"`
}

func (a *App) GetSettings() (*Settings, error) {
	return &Settings{
		ServerURL:    a.cfg.GetServerURL(),
		SyncEnabled:  a.cfg.SyncEnabled,
		SyncInterval: a.cfg.SyncInterval,
	}, nil
}

func (a *App) UpdateSettings(req UpdateSettingsRequest) error {
	if req.ServerURL != nil {
		a.cfg.SetServerURL(*req.ServerURL)
	}
	if req.SyncIntervalMinutes != nil && *req.SyncIntervalMinutes > 0 {
		a.cfg.SyncInterval = *req.SyncIntervalMinutes
	}
	return a.cfg.Save()
}

// --- updates --------------------------------------------------------------

// CheckUpdate queries GitHub Releases for a newer dfswitch binary.
type UpdateInfo struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	ReleaseNotes   string `json:"release_notes,omitempty"`
	DownloadURL    string `json:"download_url,omitempty"`
}

func (a *App) CheckUpdate() (*UpdateInfo, error) {
	return checkGitHubRelease(a.ctx)
}

// ApplyUpdate downloads the newer binary and replaces the running executable.
// Frontend should warn the user that the window will close after this call.
func (a *App) ApplyUpdate() error {
	return applyGitHubUpdate(a.ctx)
}

// --- helpers --------------------------------------------------------------

// translateClientError maps internal/client errors into messages the frontend
// can surface verbatim. ErrUnauthorized becomes a login-expired message so the
// UI can route back to the login page.
func translateClientError(err error) error {
	if errors.Is(err, client.ErrUnauthorized) {
		return errors.New("登录已过期，请重新登录")
	}
	var he *client.HTTPError
	if errors.As(err, &he) {
		return errors.New(he.Message)
	}
	return err
}
