package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/dollarkillerx/my-claude/internal/i18n"
)

func TestLangFromArgs(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"list"}, ""},
		{[]string{"--lang", "ja", "list"}, "ja"},
		{[]string{"list", "--lang=zh"}, "zh"},
		{[]string{"use", "a", "--lang", "en"}, "en"},
		{[]string{"--lang"}, ""},                 // missing value
		{[]string{"add", "--", "--lang=ja"}, ""}, // after --, it is an argument
	}
	for _, c := range cases {
		if got := langFromArgs(c.args); got != c.want {
			t.Errorf("langFromArgs(%q) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestWindowLabel(t *testing.T) {
	for in, want := range map[string]string{
		"five_hour":        "5h",
		"seven_day":        "7d",
		"seven_day_opus":   "7d opus",
		"seven_day_sonnet": "7d sonnet",
		"monthly":          "monthly",
	} {
		if got := windowLabel(in); got != want {
			t.Errorf("windowLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBar(t *testing.T) {
	count := func(s, r string) int { return strings.Count(s, r) }
	for pct, filled := range map[float64]int{0: 0, 2: 0, 2.5: 1, 50: 10, 99: 20, 100: 20, 150: 20, -5: 0} {
		b := bar(pct)
		if got := count(b, "█"); got != filled {
			t.Errorf("bar(%v) filled = %d, want %d (%s)", pct, got, filled, b)
		}
		if count(b, "█")+count(b, "░") != 20 {
			t.Errorf("bar(%v) width wrong: %s", pct, b)
		}
	}
}

func TestResetsIn(t *testing.T) {
	defer i18n.SetLang(string(i18n.Current()))
	i18n.SetLang("en")

	if got := resetsIn(time.Time{}); got != "" {
		t.Errorf("zero time = %q", got)
	}
	if got := resetsIn(time.Now().Add(-time.Minute)); got != "resets now" {
		t.Errorf("past = %q", got)
	}
	if got := resetsIn(time.Now().Add(2*time.Hour + 5*time.Minute + 10*time.Second)); !strings.HasPrefix(got, "resets in 2h05m (") {
		t.Errorf("hours = %q", got)
	}
	if got := resetsIn(time.Now().Add(3*24*time.Hour + 6*time.Hour + time.Minute)); !strings.HasPrefix(got, "resets in 3d6h (") {
		t.Errorf("days = %q", got)
	}

	i18n.SetLang("zh")
	if got := resetsIn(time.Now().Add(90*time.Minute + 10*time.Second)); !strings.HasPrefix(got, "1 小时 30 分后重置") {
		t.Errorf("zh = %q", got)
	}
}

func TestOrDash(t *testing.T) {
	if orDash("") != "-" || orDash("x") != "x" {
		t.Error("orDash")
	}
}

// Help output is translated, including cobra's built-in headings and flags.
func TestHelpIsLocalized(t *testing.T) {
	defer i18n.SetLang(string(i18n.Current()))
	want := map[string][]string{
		"en": {"Usage:", "Available Commands:", "Flags:", "help for cswitch", "Switch Claude Code and Claude Desktop"},
		"zh": {"用法：", "可用命令：", "选项：", "cswitch 的帮助", "切换到指定 profile"},
		"ja": {"使い方:", "利用可能なコマンド:", "オプション:", "cswitch のヘルプ", "プロファイルに切り替え"},
	}
	for lang, needles := range want {
		i18n.SetLang(lang)
		root := NewRoot()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{"--help"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		for _, n := range needles {
			if !strings.Contains(out.String(), n) {
				t.Errorf("%s help lacks %q:\n%s", lang, n, out.String())
			}
		}
		if lang != "en" && strings.Contains(out.String(), "Available Commands:") {
			t.Errorf("%s help still has English headings", lang)
		}
	}
}

func TestEverySubcommandHasTranslatedHelp(t *testing.T) {
	defer i18n.SetLang(string(i18n.Current()))
	i18n.SetLang("ja")
	for _, c := range NewRoot().Commands() {
		if c.Short == "" {
			t.Errorf("%s has no short help", c.Name())
		}
		if f := c.Flags().Lookup("help"); f == nil || !strings.Contains(f.Usage, "ヘルプ") {
			t.Errorf("%s: help flag not translated", c.Name())
		}
	}
}

func TestMutuallyExclusiveParts(t *testing.T) {
	root := NewRoot()
	root.SetArgs([]string{"use", "x", "--code-only", "--desktop-only"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err == nil {
		t.Error("--code-only with --desktop-only should be rejected")
	}
}
