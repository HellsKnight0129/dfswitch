package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sub2api/dfswitch/internal/store"
)

// Client talks to the sub2api backend on behalf of the logged-in user.
// It owns token refresh and unwraps the standard {code,message,data} envelope.
type Client struct {
	cfg  *store.Config
	http *http.Client

	refreshMu sync.Mutex
}

func New(cfg *store.Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// envelope matches the sub2api standard response wrapper. Code is `any`
// because handler responses use int codes while middleware 401s use string
// codes (e.g. "TOKEN_EXPIRED").
type envelope struct {
	Code    any             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// ErrUnauthorized is returned when the caller must re-authenticate.
var ErrUnauthorized = errors.New("unauthorized")

// TokenPair wraps the three time-bound fields returned by login / refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// User mirrors the sub2api user DTO, only with the fields dfswitch cares about.
type User struct {
	ID            int64  `json:"id"`
	Email         string `json:"email"`
	Username      string `json:"username"`
	Role          string `json:"role"`
	Balance       any    `json:"balance"`
	Concurrency   int    `json:"concurrency"`
	Status        string `json:"status"`
	AllowedGroups []int  `json:"allowed_groups"`
	RunMode       string `json:"run_mode,omitempty"`
}

// LoginResult captures both the normal and 2FA-challenge login branches.
type LoginResult struct {
	RequiresTFA     bool
	TempToken       string
	UserEmailMasked string

	Tokens *TokenPair
	User   *User
}

// Group mirrors the embedded group DTO on each API key.
type Group struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Platform         string `json:"platform"`
	SubscriptionType string `json:"subscription_type"`
	Status           string `json:"status"`
	DailyLimitUSD    any    `json:"daily_limit_usd"`
	WeeklyLimitUSD   any    `json:"weekly_limit_usd"`
	MonthlyLimitUSD  any    `json:"monthly_limit_usd"`
	ClaudeCodeOnly   bool   `json:"claude_code_only"`
}

// Key mirrors the per-item shape returned by GET /api/v1/keys.
type Key struct {
	ID          int64  `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	GroupID     int64  `json:"group_id"`
	Status      string `json:"status"`
	Quota       any    `json:"quota"`
	QuotaUsed   any    `json:"quota_used"`
	ExpiresAt   string `json:"expires_at"`
	LastUsedAt  string `json:"last_used_at"`
	RateLimit1d any    `json:"rate_limit_1d"`
	Usage1d     any    `json:"usage_1d"`
	Concurrency int    `json:"concurrency"`
	Group       *Group `json:"group"`
}

type listKeysData struct {
	Items    []Key `json:"items"`
	Total    int   `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Pages    int   `json:"pages"`
}

// middlewareErrorCode is the set of middleware-returned string codes that
// indicate the access token itself is bad or expired — refreshable.
var refreshableCodes = map[string]struct{}{
	"TOKEN_EXPIRED":  {},
	"INVALID_TOKEN":  {},
	"TOKEN_REVOKED":  {},
	"EMPTY_TOKEN":    {},
}

// --- Public API ---------------------------------------------------------

// Login starts the login flow. If the backend returns requires_2fa, the
// result carries the temp_token instead of a real token pair.
func (c *Client) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := c.post(ctx, "/api/v1/auth/login", body, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, wrapErr(resp.StatusCode, raw)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("login: parse envelope: %w", err)
	}

	// Try 2FA branch first.
	var tfa struct {
		RequiresTFA     bool   `json:"requires_2fa"`
		TempToken       string `json:"temp_token"`
		UserEmailMasked string `json:"user_email_masked"`
	}
	_ = json.Unmarshal(env.Data, &tfa)
	if tfa.RequiresTFA {
		return &LoginResult{
			RequiresTFA:     true,
			TempToken:       tfa.TempToken,
			UserEmailMasked: tfa.UserEmailMasked,
		}, nil
	}

	var normal struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		User         *User  `json:"user"`
	}
	if err := json.Unmarshal(env.Data, &normal); err != nil {
		return nil, fmt.Errorf("login: parse data: %w", err)
	}
	if normal.AccessToken == "" {
		return nil, fmt.Errorf("login: missing access_token in response")
	}
	c.cfg.SetTokens(normal.AccessToken, normal.RefreshToken)
	_ = c.cfg.Save()
	return &LoginResult{
		Tokens: &TokenPair{
			AccessToken:  normal.AccessToken,
			RefreshToken: normal.RefreshToken,
			ExpiresIn:    normal.ExpiresIn,
			TokenType:    normal.TokenType,
		},
		User: normal.User,
	}, nil
}

// Login2FA completes the 2FA challenge.
func (c *Client) Login2FA(ctx context.Context, tempToken, totpCode string) (*LoginResult, error) {
	body, _ := json.Marshal(map[string]string{"temp_token": tempToken, "totp_code": totpCode})
	resp, err := c.post(ctx, "/api/v1/auth/login/2fa", body, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, wrapErr(resp.StatusCode, raw)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("login2fa: parse envelope: %w", err)
	}
	var normal struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		User         *User  `json:"user"`
	}
	if err := json.Unmarshal(env.Data, &normal); err != nil {
		return nil, fmt.Errorf("login2fa: parse data: %w", err)
	}
	if normal.AccessToken == "" {
		return nil, fmt.Errorf("login2fa: missing access_token")
	}
	c.cfg.SetTokens(normal.AccessToken, normal.RefreshToken)
	_ = c.cfg.Save()
	return &LoginResult{
		Tokens: &TokenPair{
			AccessToken:  normal.AccessToken,
			RefreshToken: normal.RefreshToken,
			ExpiresIn:    normal.ExpiresIn,
			TokenType:    normal.TokenType,
		},
		User: normal.User,
	}, nil
}

// Refresh rotates both access and refresh tokens. Returns ErrUnauthorized if
// the refresh token itself is invalid or missing.
func (c *Client) Refresh(ctx context.Context) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	rt := c.cfg.GetRefreshToken()
	if rt == "" {
		return ErrUnauthorized
	}

	body, _ := json.Marshal(map[string]string{"refresh_token": rt})
	resp, err := c.post(ctx, "/api/v1/auth/refresh", body, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 401 {
		return ErrUnauthorized
	}
	if resp.StatusCode != 200 {
		return wrapErr(resp.StatusCode, raw)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("refresh: parse envelope: %w", err)
	}
	var pair TokenPair
	if err := json.Unmarshal(env.Data, &pair); err != nil {
		return fmt.Errorf("refresh: parse data: %w", err)
	}
	if pair.AccessToken == "" {
		return ErrUnauthorized
	}
	c.cfg.SetTokens(pair.AccessToken, pair.RefreshToken)
	_ = c.cfg.Save()
	return nil
}

// Logout clears local tokens and calls the backend logout endpoint best-effort.
func (c *Client) Logout(ctx context.Context) {
	rt := c.cfg.GetRefreshToken()
	if rt != "" {
		body, _ := json.Marshal(map[string]string{"refresh_token": rt})
		if resp, err := c.post(ctx, "/api/v1/auth/logout", body, ""); err == nil {
			_ = resp.Body.Close()
		}
	}
	c.cfg.SetTokens("", "")
	_ = c.cfg.Save()
}

// ListKeys returns the current user's API keys, auto-refreshing on 401 once.
func (c *Client) ListKeys(ctx context.Context) ([]Key, error) {
	raw, err := c.authedGET(ctx, "/api/v1/keys?page=1&page_size=100")
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("list keys: parse envelope: %w", err)
	}
	var data listKeysData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, fmt.Errorf("list keys: parse data: %w", err)
	}
	return data.Items, nil
}

// GetMe returns the current user's profile.
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	raw, err := c.authedGET(ctx, "/api/v1/auth/me")
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("me: parse envelope: %w", err)
	}
	var u User
	if err := json.Unmarshal(env.Data, &u); err != nil {
		return nil, fmt.Errorf("me: parse data: %w", err)
	}
	return &u, nil
}

// --- Internals ----------------------------------------------------------

// authedGET runs a GET with the access token and transparently refreshes on
// middleware 401 (string code in the refreshable set) before retrying once.
func (c *Client) authedGET(ctx context.Context, path string) ([]byte, error) {
	raw, status, code, err := c.doGET(ctx, path)
	if err != nil {
		return nil, err
	}
	if !isRefreshable(status, code) {
		if status != 200 {
			return nil, wrapErr(status, raw)
		}
		return raw, nil
	}

	// Try refresh + retry once.
	if err := c.Refresh(ctx); err != nil {
		return nil, ErrUnauthorized
	}
	raw, status, _, err = c.doGET(ctx, path)
	if err != nil {
		return nil, err
	}
	if status == 401 {
		return nil, ErrUnauthorized
	}
	if status != 200 {
		return nil, wrapErr(status, raw)
	}
	return raw, nil
}

func (c *Client) doGET(ctx context.Context, path string) ([]byte, int, string, error) {
	token := c.cfg.GetAccessToken()
	if token == "" {
		return nil, 0, "", ErrUnauthorized
	}
	server := c.cfg.GetServerURL()
	if server == "" {
		return nil, 0, "", fmt.Errorf("server URL not configured")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(server, "/")+path, nil)
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	code := ""
	if resp.StatusCode == 401 {
		code = extractErrorCode(raw)
	}
	return raw, resp.StatusCode, code, nil
}

func (c *Client) post(ctx context.Context, path string, body []byte, bearer string) (*http.Response, error) {
	server := c.cfg.GetServerURL()
	if server == "" {
		return nil, fmt.Errorf("server URL not configured")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(server, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return c.http.Do(req)
}

// isRefreshable decides whether a 401 should trigger a refresh attempt.
// Handler 401s have int code (often 401) and mean "not logged in" — refresh
// still worth trying since access token may just be missing.
// Middleware 401s use string codes; only TOKEN_EXPIRED/INVALID_TOKEN/
// TOKEN_REVOKED/EMPTY_TOKEN are refreshable, the rest (USER_INACTIVE etc.)
// cannot be fixed by rotating tokens.
func isRefreshable(status int, code string) bool {
	if status != 401 {
		return false
	}
	if code == "" {
		return true // handler-level 401 (int code), safe to try
	}
	_, ok := refreshableCodes[code]
	return ok
}

func extractErrorCode(raw []byte) string {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if s, ok := m["code"].(string); ok {
		return s
	}
	return ""
}

func wrapErr(status int, raw []byte) error {
	// Try to extract message from envelope or middleware shape.
	var m struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(raw, &m)
	msg := m.Message
	if msg == "" {
		msg = m.Error
	}
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", status)
	}
	return &HTTPError{Status: status, Message: msg, Body: raw}
}

// HTTPError carries the upstream status + message so handlers can echo
// meaningful errors back to the frontend.
type HTTPError struct {
	Status  int
	Message string
	Body    []byte
}

func (e *HTTPError) Error() string { return fmt.Sprintf("sub2api: %s (HTTP %d)", e.Message, e.Status) }

// QueryEscape is re-exported so callers in the same module need not import
// net/url directly.
func QueryEscape(s string) string { return url.QueryEscape(s) }
