package usage

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParse(t *testing.T) {
	r, err := Parse([]byte(`{
		"seven_day_opus": {"utilization": 3, "resets_at": null},
		"seven_day": {"utilization": 41.5, "resets_at": "2026-10-20T10:00:00+00:00"},
		"five_hour": {"utilization": 12, "resets_at": "2026-10-14T15:00:00Z"},
		"extra_usage": {"is_enabled": false},
		"seven_day_sonnet": null
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, w := range r.Windows {
		names = append(names, w.Name)
	}
	if len(names) != 3 || names[0] != "five_hour" || names[1] != "seven_day" || names[2] != "seven_day_opus" {
		t.Fatalf("windows = %v", names)
	}
	if r.Windows[1].Utilization != 41.5 || r.Windows[1].ResetsAt.IsZero() {
		t.Errorf("seven_day = %+v", r.Windows[1])
	}
	if !r.Windows[2].ResetsAt.IsZero() {
		t.Errorf("null resets_at should be zero")
	}
}

// Shape of the real response observed from the endpoint (trimmed).
const realResponse = `{"five_hour":{"utilization":0.0,"resets_at":null},"seven_day":{"utilization":100.0,"resets_at":"2026-10-15T19:00:00.253393+00:00"},"seven_day_opus":null,"iguana_necktie":{"utilization":0.0,"resets_at":"2026-11-05T07:59:00+00:00","limit_dollars":250},"limits":[{"kind":"session","group":"session","percent":0,"severity":"normal","resets_at":null,"scope":null,"is_active":false},{"kind":"weekly_all","group":"weekly","percent":100,"severity":"critical","resets_at":"2026-10-15T19:00:00.253393+00:00","scope":null,"is_active":true},{"kind":"weekly_scoped","group":"weekly","percent":0,"severity":"normal","resets_at":"2026-10-15T19:00:00+00:00","scope":{"model":{"id":null,"display_name":"Fable"},"surface":null},"is_active":false}]}`

func TestParseLimits(t *testing.T) {
	r, err := Parse([]byte(realResponse))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Limits) != 3 {
		t.Fatalf("limits = %+v", r.Limits)
	}
	s, w := r.Session(), r.Weekly()
	if s == nil || s.Percent != 0 || s.Active || !s.ResetsAt.IsZero() {
		t.Errorf("session = %+v", s)
	}
	if w == nil || w.Percent != 100 || !w.Active || w.ResetsAt.IsZero() {
		t.Errorf("weekly = %+v", w)
	}
	if r.Limits[2].Model != "Fable" || r.Limits[2].Kind != "weekly_scoped" {
		t.Errorf("scoped = %+v", r.Limits[2])
	}
	if len(r.Windows) == 0 {
		t.Error("legacy windows should still be parsed")
	}
}

func TestFetchProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"account":{"uuid":"acc-1","email":"a@example.com","display_name":"A"},"organization":{"uuid":"org-1","name":"A's Org","organization_type":"claude_max","rate_limit_tier":"default_claude_max_20x","billing_type":"stripe_subscription"},"application":{"slug":"claude-code"}}`))
	}))
	defer srv.Close()
	t.Setenv("CSWITCH_PROFILE_URL", srv.URL)

	p, err := FetchProfile(t.Context(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if p.AccountUUID != "acc-1" || p.Email != "a@example.com" || p.OrganizationUUID != "org-1" || p.RateLimitTier != "default_claude_max_20x" {
		t.Errorf("profile = %+v", p)
	}
	if _, err := FetchProfile(t.Context(), "bad"); !IsUnauthorized(err) {
		t.Errorf("bad token: %v", err)
	}
}
