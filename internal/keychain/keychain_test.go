package keychain

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestParsePassword(t *testing.T) {
	cases := map[string]string{
		"keychain: \"x\"\npassword: \"pl\"ain\"text\"\n":                        `pl"ain"text`,
		"password: 0x7B2261223A22E4B8AD227D  \"{\"a\":\"\\344\\270\\255\"}\"\n": `{"a":"中"}`,
		"password: 0x61626364\n":                                                "abcd",
		"password: \"0x1234\"\n":                                                "0x1234",
		"password: \n":                                                          "",
	}
	for in, want := range cases {
		got, err := parsePassword(in)
		if err != nil || got != want {
			t.Errorf("parsePassword(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestParseAccount(t *testing.T) {
	cases := map[string]string{
		`    "acct"<blob>="github"` + "\n":                                   "github",
		`    "acct"<blob>="with \"quote\""` + "\n":                           `with \"quote\"`,
		`    "acct"<blob>=0x E5908DE5898D  "\345\220\215"` + "\n":            "", // malformed spacing: no match
		`    "acct"<blob>=0xE5908DE5898D  "\345\220\215\345\211\215"` + "\n": "名前",
	}
	for in, want := range cases {
		got, err := parseAccount(in, "svc")
		if want == "" {
			if err == nil {
				t.Errorf("parseAccount(%q) = %q, want error", in, got)
			}
			continue
		}
		if err != nil || got != want {
			t.Errorf("parseAccount(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := parseAccount(`"svce"<blob>="x"`, "svc"); err == nil {
		t.Error("missing acct should fail")
	}
}

func TestSecurityErrorsAreRedacted(t *testing.T) {
	// runQuiet must drop security's output, which can echo the secret.
	_, err := runQuiet(nil, "add-generic-password", "-s", "x", "-X", "not-hex-SECRET")
	if err == nil {
		t.Skip("security accepted the bogus call")
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Errorf("secret leaked into error: %v", err)
	}
}

func TestExitCodesMapToSentinels(t *testing.T) {
	for code, want := range map[int]error{44: ErrNotFound, 36: ErrLocked} {
		err := exec.Command("/bin/sh", "-c", fmt.Sprintf("exit %d", code)).Run()
		if got := exitErr(err); got != want {
			t.Errorf("exit %d -> %v, want %v", code, got, want)
		}
	}
	if exitErr(exec.Command("/bin/sh", "-c", "exit 1").Run()) != nil {
		t.Error("other exit codes must not map to a sentinel")
	}
	if !strings.Contains(ErrLocked.Error(), "unlock-keychain") {
		t.Errorf("locked message lacks the fix: %q", ErrLocked.Error())
	}
}
