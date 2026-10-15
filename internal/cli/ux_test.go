package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
	"github.com/dollarkillerx/my-claude/internal/usage"
)

func TestSanitizeName(t *testing.T) {
	for in, want := range map[string]string{
		"beta":        "beta",
		"John.Doe":    "john.doe",
		"me+work":     "me-work",
		"_x":          "x",
		"名前":          "",
		"a b":         "ab",
		"...":         "",
		"user_name-1": "user_name-1",
	} {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAutoName(t *testing.T) {
	t.Setenv("CSWITCH_HOME", t.TempDir())
	if got, _ := autoName("beta@example.com", "u1"); got != "beta" {
		t.Errorf("from email = %q", got)
	}
	if got, _ := autoName("", "1a2b3c4d-0000-4000-8000-000000000001"); got != "desktop-1a2b3c4d" {
		t.Errorf("desktop = %q", got)
	}
	if got, _ := autoName("名前@example.com", "abcdef12-0000"); got != "desktop-abcdef12" {
		t.Errorf("unusable email = %q", got)
	}
	store.Save(&store.Profile{Name: "beta"})
	store.Save(&store.Profile{Name: "beta-2"})
	if got, _ := autoName("beta@other.com", "u2"); got != "beta-3" {
		t.Errorf("taken name = %q", got)
	}
}

func TestSavedLanguageIsUsed(t *testing.T) {
	defer i18n.SetLang(string(i18n.Current()))
	t.Setenv("CSWITCH_HOME", t.TempDir())
	t.Setenv("CSWITCH_LANG", "")
	os.Unsetenv("CSWITCH_LANG")

	root := NewRoot()
	root.SetArgs([]string{"lang", "zh"})
	root.SetOut(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if store.LoadSettings().Lang != "zh" {
		t.Fatal("language not saved")
	}

	// A later run picks the saved language up...
	i18n.SetLang("en")
	applyLanguage([]string{"list"})
	if i18n.Current() != i18n.ZH {
		t.Errorf("saved language not applied: %s", i18n.Current())
	}
	// ...but --lang still wins for one run.
	applyLanguage([]string{"--lang", "ja", "list"})
	if i18n.Current() != i18n.JA {
		t.Errorf("--lang did not override: %s", i18n.Current())
	}

	root = NewRoot()
	root.SetArgs([]string{"lang", "auto"})
	root.SetOut(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if store.LoadSettings().Lang != "" {
		t.Error("auto did not clear the setting")
	}

	root = NewRoot()
	root.SetArgs([]string{"lang", "fr"})
	root.SetOut(&bytes.Buffer{})
	if err := root.Execute(); err == nil {
		t.Error("unsupported language accepted")
	}
}

func TestSwitchUnknownNameExplains(t *testing.T) {
	defer i18n.SetLang(string(i18n.Current()))
	i18n.SetLang("en")
	t.Setenv("CSWITCH_HOME", t.TempDir())
	store.Save(&store.Profile{Name: "work", Email: "w@example.com"})

	root := NewRoot()
	root.SetArgs([]string{"remote"})
	root.SetOut(&bytes.Buffer{})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{`"remote"`, "work", "cswitch remove"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message lacks %q:\n%s", want, msg)
		}
	}
}

func TestPickWithoutTerminalPrintsTable(t *testing.T) {
	defer i18n.SetLang(string(i18n.Current()))
	i18n.SetLang("en")
	dir := t.TempDir()
	t.Setenv("CSWITCH_HOME", dir)
	t.Setenv("CSWITCH_CLAUDE_JSON", filepath.Join(dir, "none.json"))
	t.Setenv("CSWITCH_CODE_CREDENTIALS_FILE", filepath.Join(dir, "none-creds.json"))
	t.Setenv("CSWITCH_KEY_FILE", filepath.Join(dir, "key"))
	t.Setenv("CSWITCH_DESKTOP_DIR", filepath.Join(dir, "no-desktop"))
	store.Save(&store.Profile{Name: "alpha", HasDesktop: true, AccountUUID: "u"})

	// Run the picker with stdin from a non-terminal and capture stdout.
	r, w, _ := os.Pipe()
	oldOut, oldIn := os.Stdout, os.Stdin
	os.Stdout, os.Stdin = w, r
	err := pick(t.Context(), switchFlags{})
	w.Close()
	os.Stdout, os.Stdin = oldOut, oldIn
	var out bytes.Buffer
	out.ReadFrom(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "alpha") || !strings.Contains(out.String(), "(Desktop only)") {
		t.Errorf("table = %q", out.String())
	}
}

func TestKnownWindow(t *testing.T) {
	for _, n := range []string{"five_hour", "seven_day", "seven_day_opus"} {
		if !knownWindow(n) {
			t.Errorf("%s should be shown", n)
		}
	}
	if knownWindow("iguana_necktie") {
		t.Error("internal codename should be hidden by default")
	}
}

func TestPrintReportLikeTheApp(t *testing.T) {
	defer i18n.SetLang(string(i18n.Current()))
	i18n.SetLang("zh")
	r := &usage.Report{Limits: []usage.Limit{
		{Kind: "session", Group: "session", Percent: 0},
		{Kind: "weekly_all", Group: "weekly", Percent: 100, Active: true, ResetsAt: time.Now().Add(30 * time.Hour)},
		{Kind: "weekly_scoped", Group: "weekly", Percent: 3, Model: "Fable", ResetsAt: time.Now().Add(30 * time.Hour)},
	}, Windows: []usage.Window{{Name: "iguana_necktie", Utilization: 0}}}

	out := captureStdout(t, func() { printReport(r, false) })
	for _, want := range []string{"当前会话", "未开始", "本周", "100.0%", "本周 Fable", "3.0%"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "iguana") {
		t.Error("internal codename shown without --all")
	}
	if all := captureStdout(t, func() { printReport(r, true) }); !strings.Contains(all, "iguana_necktie") {
		t.Error("--all should list raw windows")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var b bytes.Buffer
	b.ReadFrom(r)
	return b.String()
}
