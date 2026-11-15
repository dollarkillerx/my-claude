package usage

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Shape of a usage response with a reset status block (?cedar_ember=1).
const resetsResponse = `{"limits":[{"kind":"weekly_all","group":"weekly","percent":100,"resets_at":"2030-01-05T00:00:00Z","is_active":true}],
"cedar_ember":{"eligible":true,"at_limit":true,"grants":[
 {"id":"example-grant","label":"Fable reset","resets_total":1,"resets_left":1,"ends_at":"2030-03-01T00:00:00Z","paused":false,"usable_now":true,"use_requires_limit":true,"type":"full","gives_back_weekly":false},
 {"id":"weekly-1","label":"Weekly reset","resets_total":1,"resets_left":1,"ends_at":"2026-12-10T22:00:00Z","usable_now":false,"use_requires_limit":true,"type":"weekly","gives_back_weekly":true},
 {"id":"BAD ID","resets_left":3}
],"next_grant_id":"example-grant","weekly_resets_at":"2030-01-05T00:00:00Z","cooldown_until":null}}`

func TestParseResets(t *testing.T) {
	r, err := Parse([]byte(resetsResponse))
	if err != nil {
		t.Fatal(err)
	}
	rs := r.Resets
	if rs == nil || !rs.Eligible || !rs.AtLimit {
		t.Fatalf("resets = %+v", rs)
	}
	if len(rs.Grants) != 2 {
		t.Fatalf("grants = %+v (malformed ones must be skipped)", rs.Grants)
	}
	g := rs.Grant("example-grant")
	if g == nil || g.Label != "Fable reset" || g.ResetsLeft != 1 || !g.UsableNow || g.EndsAt.IsZero() {
		t.Errorf("grant = %+v", g)
	}
	if rs.NextGrantID != "example-grant" || rs.Available() != 2 {
		t.Errorf("next = %q available = %d", rs.NextGrantID, rs.Available())
	}
	if (&Resets{Eligible: false, Grants: rs.Grants}).Available() != 0 {
		t.Error("ineligible accounts have none available")
	}
	if r2, _ := Parse([]byte(`{"cedar_ember":null}`)); r2.Resets != nil {
		t.Error("null block should give no status")
	}
}

func TestClaimReset(t *testing.T) {
	var got struct {
		path, auth string
		body       map[string]string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.auth = r.Method+" "+r.URL.Path, r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &got.body)
		w.Write([]byte(`{"result":"reset","resets_left":0,"cleared":["weekly"]}`))
	}))
	defer srv.Close()
	t.Setenv("CSWITCH_API_URL", srv.URL)

	res, err := ClaimReset(t.Context(), "tok", "org-1", "example-grant")
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() || res.ResetsLeft != 0 {
		t.Errorf("result = %+v", res)
	}
	if got.path != "POST /api/organizations/org-1/reset_rate_limits" || got.auth != "Bearer tok" {
		t.Errorf("request = %q %q", got.path, got.auth)
	}
	if got.body["program"] != "cedar_ember" || got.body["grant_id"] != "example-grant" || len(got.body["request_id"]) != 32 {
		t.Errorf("body = %v", got.body)
	}
	if _, err := ClaimReset(t.Context(), "tok", "org-1", "Bad Grant!"); err == nil {
		t.Error("malformed grant ID must be refused before sending")
	}
}

func TestClaimResetRefusalAndUnknownResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":"something_new","reason":"x"}`))
	}))
	defer srv.Close()
	t.Setenv("CSWITCH_API_URL", srv.URL)
	res, err := ClaimReset(t.Context(), "tok", "org-1", "g1")
	if err != nil || res.OK() || res.Result != "unavailable" || res.ResetsLeft != -1 {
		t.Errorf("res = %+v, %v", res, err)
	}
}

func TestRateLimitedRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	t.Setenv("CSWITCH_USAGE_URL", srv.URL)
	_, err := Fetch(t.Context(), "tok")
	limited, after := IsRateLimited(err)
	if !limited || after != 2*time.Minute {
		t.Errorf("limited = %v after = %v (%v)", limited, after, err)
	}
}

func TestUsageURLAsksForResetStatus(t *testing.T) {
	var q string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.RawQuery
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	t.Setenv("CSWITCH_USAGE_URL", srv.URL)
	Fetch(t.Context(), "tok")
	if q != "cedar_ember=1&skip_spend=1" {
		t.Errorf("query = %q", q)
	}
}
