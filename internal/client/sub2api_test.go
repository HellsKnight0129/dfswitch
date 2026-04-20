package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sub2api/dfswitch/internal/store"
)

// isolateConfig redirects HOME to a tempdir so store.Save() doesn't touch the
// user's real ~/.dfswitch.
func isolateConfig(t *testing.T) *store.Config {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	return store.NewDefault()
}

func TestLogin_UnwrapsEnvelope(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"access_token":"A","refresh_token":"R","expires_in":900,"token_type":"Bearer","user":{"id":1,"email":"e"}}}`))
	}))
	defer ts.Close()

	cfg := isolateConfig(t)
	cfg.SetServerURL(ts.URL)
	res, err := New(cfg).Login(context.Background(), "e", "p")
	if err != nil {
		t.Fatal(err)
	}
	if res.RequiresTFA {
		t.Fatal("should not require TFA")
	}
	if cfg.GetAccessToken() != "A" || cfg.GetRefreshToken() != "R" {
		t.Fatalf("tokens not stored: %q / %q", cfg.GetAccessToken(), cfg.GetRefreshToken())
	}
	if res.User == nil || res.User.Email != "e" {
		t.Fatal("user not parsed")
	}
}

func TestLogin_DetectsTFA(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"requires_2fa":true,"temp_token":"T","user_email_masked":"e***@x"}}`))
	}))
	defer ts.Close()

	cfg := isolateConfig(t)
	cfg.SetServerURL(ts.URL)
	res, err := New(cfg).Login(context.Background(), "e", "p")
	if err != nil {
		t.Fatal(err)
	}
	if !res.RequiresTFA {
		t.Fatal("should require TFA")
	}
	if res.TempToken != "T" || res.UserEmailMasked != "e***@x" {
		t.Fatalf("bad TFA payload: %+v", res)
	}
	if cfg.GetAccessToken() != "" {
		t.Fatal("tokens should not be stored on TFA challenge")
	}
}

func TestListKeys_RefreshesOn401(t *testing.T) {
	var keysCalls, refreshCalls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/keys":
			atomic.AddInt32(&keysCalls, 1)
			tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if tok == "OLD" {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"code":"TOKEN_EXPIRED","message":"expired"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":1,"key":"k","name":"n","status":"active","group":{"platform":"anthropic"}}],"total":1,"page":1,"page_size":100,"pages":1}}`))
		case r.URL.Path == "/api/v1/auth/refresh":
			atomic.AddInt32(&refreshCalls, 1)
			body, _ := io.ReadAll(r.Body)
			var in map[string]string
			_ = json.Unmarshal(body, &in)
			if in["refresh_token"] != "RT" {
				t.Errorf("bad refresh body: %s", body)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"NEW","refresh_token":"RT2","expires_in":900,"token_type":"Bearer"}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()

	cfg := isolateConfig(t)
	cfg.SetServerURL(ts.URL)
	cfg.SetTokens("OLD", "RT")

	keys, err := New(cfg).ListKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].Key != "k" {
		t.Fatalf("bad keys: %+v", keys)
	}
	if atomic.LoadInt32(&refreshCalls) != 1 {
		t.Fatalf("refresh should be called exactly once, got %d", refreshCalls)
	}
	if atomic.LoadInt32(&keysCalls) != 2 {
		t.Fatalf("keys should be called twice (fail + retry), got %d", keysCalls)
	}
	if cfg.GetAccessToken() != "NEW" || cfg.GetRefreshToken() != "RT2" {
		t.Fatalf("tokens not rotated: %q / %q", cfg.GetAccessToken(), cfg.GetRefreshToken())
	}
}

func TestListKeys_UnauthorizedWhenRefreshFails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/keys":
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"code":"TOKEN_EXPIRED"}`))
		case "/api/v1/auth/refresh":
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"code":"TOKEN_REVOKED"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer ts.Close()

	cfg := isolateConfig(t)
	cfg.SetServerURL(ts.URL)
	cfg.SetTokens("BAD", "BAD-RT")

	_, err := New(cfg).ListKeys(context.Background())
	if err != ErrUnauthorized {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestListKeys_NoRetryOnNonRefreshable401(t *testing.T) {
	var refreshCalls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/keys":
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"code":"USER_INACTIVE","message":"disabled"}`))
		case "/api/v1/auth/refresh":
			atomic.AddInt32(&refreshCalls, 1)
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"N","refresh_token":"N"}}`))
		}
	}))
	defer ts.Close()

	cfg := isolateConfig(t)
	cfg.SetServerURL(ts.URL)
	cfg.SetTokens("T", "RT")

	_, err := New(cfg).ListKeys(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if atomic.LoadInt32(&refreshCalls) != 0 {
		t.Fatalf("USER_INACTIVE must NOT trigger refresh, got %d calls", refreshCalls)
	}
}
