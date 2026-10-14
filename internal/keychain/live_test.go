//go:build livekeychain

package keychain

import (
	"strings"
	"testing"
)

// Writes test-only items to the login keychain:
//
//	go test -tags livekeychain ./internal/keychain
func TestLiveRoundTrip(t *testing.T) {
	defer Delete("cswitch-probe", "probe")
	for _, v := range []string{`{"n":"中文"}`, `plain "quoted" text`, "deadbeef"} {
		if err := Set("cswitch-probe", "probe", v); err != nil {
			t.Fatalf("Set(%q): %v", v, err)
		}
	}
}

// Desktop snapshots and Claude Code credentials with MCP tokens can be
// several KB. security -i truncates input lines at ~4 KB.
func TestLiveLargeSecret(t *testing.T) {
	defer Delete("cswitch-probe", "probe")
	for _, n := range []int{1000, 4000, 16000} {
		v := strings.Repeat("x", n)
		if err := Set("cswitch-probe", "probe", v); err != nil {
			t.Errorf("Set(%d bytes): %v", n, err)
		}
	}
}
