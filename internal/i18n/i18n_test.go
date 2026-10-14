package i18n

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var verbRe = regexp.MustCompile(`%(?:\[(\d+)\])?[-+# 0]*\d*(?:\.\d+)?([a-zA-Z%])`)

// verbs returns "argIndex:verb" for every format verb, resolving explicit
// indexes like %[2]s so reordered translations compare equal.
func verbs(s string) []string {
	var out []string
	next := 1
	for _, m := range verbRe.FindAllStringSubmatch(s, -1) {
		if m[2] == "%" {
			continue
		}
		idx := next
		if m[1] != "" {
			idx, _ = strconv.Atoi(m[1])
		}
		out = append(out, strconv.Itoa(idx)+":"+m[2])
		next = idx + 1
	}
	sort.Strings(out)
	return out
}

func TestCatalogComplete(t *testing.T) {
	for key, m := range messages {
		for _, l := range Supported {
			if strings.TrimSpace(m[l]) == "" {
				t.Errorf("%s: missing %s translation", key, l)
			}
		}
		want := strings.Join(verbs(m[EN]), ",")
		for _, l := range Supported[1:] {
			if got := strings.Join(verbs(m[l]), ","); got != want {
				t.Errorf("%s: %s verbs %q differ from en %q", key, l, got, want)
			}
		}
	}
}

// TestKeysUsedExist scans the module for i18n.T/E calls and checks every
// literal key is in the catalog.
func TestKeysUsedExist(t *testing.T) {
	callRe := regexp.MustCompile(`i18n\.[TE]\("([a-z_.]+)"`)
	listRe := regexp.MustCompile(`"(help\.[a-z_]+)"`)
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var keys [][]string
		keys = append(keys, callRe.FindAllStringSubmatch(string(src), -1)...)
		if !strings.Contains(path, "i18n") {
			keys = append(keys, listRe.FindAllStringSubmatch(string(src), -1)...)
		}
		for _, k := range keys {
			key := k[1]
			if strings.HasSuffix(key, "_") { // dynamic: "err.no_secret_"+kind
				continue
			}
			if _, ok := messages[key]; !ok {
				t.Errorf("%s: unknown message key %q", path, key)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestParse(t *testing.T) {
	for in, want := range map[string]Lang{"zh_CN.UTF-8": ZH, "zh-Hant": ZH, "ja_JP": JA, "en_US.UTF-8": EN, "EN": EN} {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q) = %q, %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "C", "POSIX", "fr_FR"} {
		if _, ok := Parse(in); ok {
			t.Errorf("Parse(%q) should fail", in)
		}
	}
}

func TestTranslateAndWrap(t *testing.T) {
	defer SetLang(string(Current()))
	base := errors.New("boom")
	for _, l := range Supported {
		SetLang(string(l))
		err := E("err.refresh", base)
		if !errors.Is(err, base) {
			t.Errorf("%s: E does not wrap", l)
		}
		if !strings.Contains(err.Error(), "boom") {
			t.Errorf("%s: %q", l, err)
		}
	}
	SetLang("ja")
	if got := T("err.desktop_other_account", "UUID", "work"); !strings.Contains(got, "work") || !strings.Contains(got, "UUID") || strings.Index(got, "work") > strings.Index(got, "UUID") {
		t.Errorf("indexed verbs: %q", got)
	}
	if got := T("no.such.key"); got != "no.such.key" {
		t.Errorf("unknown key: %q", got)
	}
}
