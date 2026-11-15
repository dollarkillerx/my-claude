package usage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/dollarkillerx/my-claude/internal/i18n"
)

// Limit resets ("上限のリセット" in the apps) are grants that refill an
// account's usage once, e.g. a weekly reset or a model's weekly allowance.
// Claude Code reads their status from the usage endpoint's "cedar_ember"
// block (?cedar_ember=1) and claims one with
// POST /api/organizations/{org}/reset_rate_limits, both with its OAuth login.
// The field names below follow Claude Code's own schema.

// Grant is one limit reset an account holds.
type Grant struct {
	ID                string    `json:"id"`
	Label             string    `json:"label"`
	ResetsTotal       int       `json:"resets_total"`
	ResetsLeft        int       `json:"resets_left"`
	StartsAt          time.Time `json:"-"`
	EndsAt            time.Time `json:"-"` // when the grant expires
	Paused            bool      `json:"paused"`
	UsableNow         bool      `json:"usable_now"`
	UseRequiresLimit  bool      `json:"use_requires_limit"` // only usable when at a limit
	Type              string    `json:"type"`               // "session", "weekly", "full"
	GivesBackWeekly   bool      `json:"gives_back_weekly"`
	UnavailableReason string    `json:"unavailable_reason"`
}

// Resets is an account's limit-reset status.
type Resets struct {
	Eligible         bool
	IneligibleReason string
	AtLimit          bool
	Grants           []Grant
	NextGrantID      string    // the grant Claude Code would use next
	WeeklyResetsAt   time.Time // when the weekly window resets anyway
	CooldownUntil    time.Time // no claim possible before this
}

// Available is the number of resets left across grants that are not paused.
func (r *Resets) Available() int {
	if r == nil || !r.Eligible {
		return 0
	}
	n := 0
	for _, g := range r.Grants {
		if !g.Paused {
			n += g.ResetsLeft
		}
	}
	return n
}

// Grant returns the grant with the given ID, or nil.
func (r *Resets) Grant(id string) *Grant {
	if r == nil {
		return nil
	}
	for i := range r.Grants {
		if r.Grants[i].ID == id {
			return &r.Grants[i]
		}
	}
	return nil
}

var (
	grantIDRe   = regexp.MustCompile(`^[a-z0-9_-]{1,40}$`)
	requestIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

func parseTime(s *string) time.Time {
	if s == nil {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, *s)
	return t
}

// parseResets reads the "cedar_ember" block leniently: malformed grants are
// skipped, like Claude Code does.
func parseResets(raw json.RawMessage) *Resets {
	var v struct {
		Eligible         bool              `json:"eligible"`
		IneligibleReason *string           `json:"ineligible_reason"`
		AtLimit          bool              `json:"at_limit"`
		Grants           []json.RawMessage `json:"grants"`
		NextGrantID      *string           `json:"next_grant_id"`
		WeeklyResetsAt   *string           `json:"weekly_resets_at"`
		CooldownUntil    *string           `json:"cooldown_until"`
	}
	if string(raw) == "null" || json.Unmarshal(raw, &v) != nil {
		return nil
	}
	r := &Resets{
		Eligible:       v.Eligible,
		AtLimit:        v.AtLimit,
		WeeklyResetsAt: parseTime(v.WeeklyResetsAt),
		CooldownUntil:  parseTime(v.CooldownUntil),
	}
	if v.IneligibleReason != nil {
		r.IneligibleReason = *v.IneligibleReason
	}
	for _, gr := range v.Grants {
		var g Grant
		var times struct {
			StartsAt *string `json:"starts_at"`
			EndsAt   *string `json:"ends_at"`
		}
		g.UseRequiresLimit = true // Claude Code's default when absent
		if json.Unmarshal(gr, &g) != nil || json.Unmarshal(gr, &times) != nil || !grantIDRe.MatchString(g.ID) {
			continue
		}
		g.StartsAt, g.EndsAt = parseTime(times.StartsAt), parseTime(times.EndsAt)
		r.Grants = append(r.Grants, g)
	}
	if v.NextGrantID != nil && r.Grant(*v.NextGrantID) != nil {
		r.NextGrantID = *v.NextGrantID
	}
	return r
}

// ClaimResult is the server's answer to a reset claim.
type ClaimResult struct {
	Result        string    // "reset", "already_used", "not_limited", "cooldown", "ineligible", "unavailable"
	Reason        string    // detail for a refusal
	ResetsLeft    int       // -1 when not reported
	CooldownUntil time.Time // when the next claim is possible, if reported
}

// OK reports whether the reset was applied.
func (c *ClaimResult) OK() bool { return c.Result == "reset" }

// ClaimReset uses one reset of grantID for the organization orgUUID, the
// way Claude Code does. A fresh request ID makes the claim idempotent if the
// response is lost and the same request is retried.
func ClaimReset(ctx context.Context, accessToken, orgUUID, grantID string) (*ClaimResult, error) {
	if orgUUID == "" || !grantIDRe.MatchString(grantID) {
		return nil, i18n.E("err.reset_bad_grant", grantID)
	}
	rid := make([]byte, 16)
	if _, err := rand.Read(rid); err != nil {
		return nil, err
	}
	requestID := hex.EncodeToString(rid)
	if !requestIDRe.MatchString(requestID) {
		return nil, errors.New("bad request id")
	}
	body, _ := json.Marshal(map[string]string{"program": "cedar_ember", "grant_id": grantID, "request_id": requestID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL()+"/api/organizations/"+orgUUID+"/reset_rate_limits", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Content-Type", "application/json")
	data, err := do(req)
	if err != nil {
		return nil, i18n.E("err.reset_claim", err)
	}
	var v struct {
		Result        string  `json:"result"`
		Reason        *string `json:"reason"`
		ResetsLeft    *int    `json:"resets_left"`
		CooldownUntil *string `json:"cooldown_until"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, i18n.E("err.reset_claim", err)
	}
	res := &ClaimResult{Result: v.Result, ResetsLeft: -1, CooldownUntil: parseTime(v.CooldownUntil)}
	switch res.Result {
	case "reset", "already_used", "not_limited", "cooldown", "ineligible", "unavailable":
	default:
		res.Result = "unavailable" // as Claude Code treats unknown answers
	}
	if v.Reason != nil {
		res.Reason = *v.Reason
	}
	if v.ResetsLeft != nil {
		res.ResetsLeft = *v.ResetsLeft
	}
	return res, nil
}
