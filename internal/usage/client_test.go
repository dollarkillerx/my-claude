package usage

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchSendsHeadersAndParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("anthropic-beta") != "oauth-2025-04-20" {
			t.Errorf("anthropic-beta = %q", r.Header.Get("anthropic-beta"))
		}
		w.Write([]byte(`{"five_hour":{"utilization":42,"resets_at":"2030-01-01T00:00:00Z"}}`))
	}))
	defer srv.Close()
	t.Setenv("CSWITCH_USAGE_URL", srv.URL)

	r, err := Fetch(t.Context(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Windows) != 1 || r.Windows[0].Utilization != 42 {
		t.Errorf("report = %+v", r)
	}
}

func TestFetchUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"type":"authentication_error","message":"invalid token"}}`))
	}))
	defer srv.Close()
	t.Setenv("CSWITCH_USAGE_URL", srv.URL)

	_, err := Fetch(t.Context(), "bad")
	if !IsUnauthorized(err) {
		t.Fatalf("IsUnauthorized(%v) = false", err)
	}
	var he *HTTPError
	if !errors.As(err, &he) || he.Message != "invalid token" {
		t.Errorf("HTTPError = %+v", he)
	}
	if IsUnauthorized(errors.New("HTTP 401: looks similar")) {
		t.Error("a plain error must not count as 401")
	}
}

func TestRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if r.Method != http.MethodPost || body["grant_type"] != "refresh_token" || body["refresh_token"] != "r1" || body["client_id"] != clientID {
			t.Errorf("request = %s %v", r.Method, body)
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "a2", "refresh_token": "r2", "expires_in": 3600})
	}))
	defer srv.Close()
	t.Setenv("CSWITCH_TOKEN_URL", srv.URL)

	tok, err := Refresh(t.Context(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "a2" || tok.RefreshToken != "r2" {
		t.Errorf("tokens = %+v", tok)
	}
	if d := time.Until(tok.ExpiresAt); d < 59*time.Minute || d > 61*time.Minute {
		t.Errorf("expires in %v", d)
	}
}

func TestRefreshErrors(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"invalid_grant": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant","error_description":"Refresh token revoked"}`))
		},
		"no access token": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) },
		"not json":        func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`<html>`)) },
	} {
		srv := httptest.NewServer(h)
		t.Setenv("CSWITCH_TOKEN_URL", srv.URL)
		if _, err := Refresh(t.Context(), "r"); err == nil {
			t.Errorf("%s: expected error", name)
		} else if name == "invalid_grant" && !strings.Contains(err.Error(), "Refresh token revoked") {
			t.Errorf("%s: %v", name, err)
		}
		srv.Close()
	}
}

func TestErrorMessageTruncatesUnknownBodies(t *testing.T) {
	long := strings.Repeat("x", 500)
	if got := errorMessage([]byte(long)); len(got) > 210 || !strings.HasSuffix(got, "…") {
		t.Errorf("not truncated: %d chars", len(got))
	}
	if got := errorMessage([]byte(`{"error":{"type":"rate_limit_error"}}`)); got != "rate_limit_error" {
		t.Errorf("type fallback = %q", got)
	}
}
