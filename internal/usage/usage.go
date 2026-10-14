// Package usage queries plan usage with a Claude Code OAuth token, the same
// endpoint Claude Code's /usage screen uses, and refreshes expired tokens.
package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dollarkillerx/my-claude/internal/i18n"
)

// Claude Code's public OAuth client ID.
const clientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var client = &http.Client{Timeout: 20 * time.Second}

// Endpoints; CSWITCH_TOKEN_URL and CSWITCH_USAGE_URL override them for testing.
func tokenURL() string {
	return env("CSWITCH_TOKEN_URL", "https://console.anthropic.com/v1/oauth/token")
}
func usageURL() string { return env("CSWITCH_USAGE_URL", "https://api.anthropic.com/api/oauth/usage") }

// Tokens is the result of a refresh.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// Refresh exchanges a refresh token for new tokens. The refresh token rotates,
// so the caller must persist the returned one everywhere the old one was stored.
func Refresh(ctx context.Context, refreshToken string) (*Tokens, error) {
	body, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     clientID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	data, err := do(req)
	if err != nil {
		return nil, i18n.E("err.refresh", err)
	}
	var v struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(data, &v); err != nil || v.AccessToken == "" {
		return nil, i18n.E("err.refresh", errors.New("unexpected response"))
	}
	return &Tokens{
		AccessToken:  v.AccessToken,
		RefreshToken: v.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(v.ExpiresIn) * time.Second),
	}, nil
}

// Window is one rate-limit window, e.g. five_hour or seven_day.
type Window struct {
	Name        string
	Utilization float64 // percent
	ResetsAt    time.Time
}

// Report is a parsed usage response.
type Report struct {
	Windows []Window
}

// Fetch returns the usage report for an access token.
func Fetch(ctx context.Context, accessToken string) (*Report, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Accept", "application/json")
	data, err := do(req)
	if err != nil {
		return nil, i18n.E("err.fetch_usage", err)
	}
	return Parse(data)
}

// Parse reads every top-level object that has a "utilization" field as a
// window, so new windows show up without code changes.
func Parse(data []byte) (*Report, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, i18n.E("err.fetch_usage", err)
	}
	r := &Report{}
	for name, raw := range m {
		var w struct {
			Utilization *float64 `json:"utilization"`
			ResetsAt    string   `json:"resets_at"`
		}
		if json.Unmarshal(raw, &w) != nil || w.Utilization == nil {
			continue
		}
		win := Window{Name: name, Utilization: *w.Utilization}
		if t, err := time.Parse(time.RFC3339, w.ResetsAt); err == nil {
			win.ResetsAt = t
		}
		r.Windows = append(r.Windows, win)
	}
	sort.Slice(r.Windows, func(i, j int) bool { return windowOrder(r.Windows[i].Name) < windowOrder(r.Windows[j].Name) })
	return r, nil
}

func windowOrder(name string) string {
	switch name {
	case "five_hour":
		return "0"
	case "seven_day":
		return "1"
	}
	return "2" + name
}

func do(req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", "cswitch/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Message: errorMessage(data)}
	}
	return data, nil
}

// HTTPError is a non-2xx response.
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message) }

// IsUnauthorized reports whether err is a 401 response.
func IsUnauthorized(err error) bool {
	var he *HTTPError
	return errors.As(err, &he) && he.StatusCode == http.StatusUnauthorized
}

// errorMessage extracts a short message without echoing anything token-like.
func errorMessage(data []byte) string {
	var v struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(data, &v) == nil {
		for _, s := range []string{v.Error.Message, v.ErrorDescription, v.Error.Type} {
			if s != "" {
				return s
			}
		}
	}
	s := strings.TrimSpace(string(data))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
