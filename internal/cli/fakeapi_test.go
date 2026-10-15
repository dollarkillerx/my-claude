package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAPI stands in for Anthropic's OAuth endpoints. Access tokens look like
// "dummy-<who>"; owners maps <who> to the account the token belongs to.
type fakeAPI struct {
	mu        sync.Mutex
	owners    map[string][2]string // who -> {account UUID, email}
	usage     map[string]string    // who -> usage JSON (default: 10% session)
	refreshTo string               // who a refreshed token belongs to
	refreshes int
	srv       *httptest.Server
}

func newFakeAPI(t *testing.T, owners map[string][2]string) *fakeAPI {
	f := &fakeAPI{owners: owners, usage: map[string]string{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	t.Setenv("CSWITCH_PROFILE_URL", f.srv.URL+"/profile")
	t.Setenv("CSWITCH_USAGE_URL", f.srv.URL+"/usage")
	t.Setenv("CSWITCH_TOKEN_URL", f.srv.URL+"/token")
	resetLiveCache()
	return f
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/token" {
		f.refreshes++
		who := f.refreshTo
		if who == "" {
			who = "new"
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": "dummy-" + who, "refresh_token": "dummy-r-" + who, "expires_in": 3600})
		return
	}
	who := strings.TrimPrefix(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), "dummy-")
	owner, ok := f.owners[who]
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"type":"authentication_error","message":"invalid token"}}`)
		return
	}
	switch r.URL.Path {
	case "/profile":
		json.NewEncoder(w).Encode(map[string]any{
			"account":      map[string]any{"uuid": owner[0], "email": owner[1], "display_name": who},
			"organization": map[string]any{"uuid": "org-" + owner[0][:8], "name": who + "'s Organization", "organization_type": "claude_max"},
		})
	case "/usage":
		body := f.usage[who]
		if body == "" {
			body = `{"five_hour":{"utilization":10,"resets_at":"2030-01-01T00:00:00Z"},"limits":[{"kind":"session","group":"session","percent":10,"resets_at":"2030-01-01T00:00:00Z","is_active":true},{"kind":"weekly_all","group":"weekly","percent":20,"resets_at":"2030-01-05T00:00:00Z","is_active":true}]}`
		}
		fmt.Fprint(w, body)
	}
}

// http_slow delays every response of h.
func http_slow(h http.Handler, d time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(d)
		h.ServeHTTP(w, r)
	})
}
