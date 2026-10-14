// Package i18n translates cswitch's user-facing text into English, Chinese
// and Japanese.
//
// The language is chosen from, in order: SetLang (the --lang flag),
// CSWITCH_LANG, LC_ALL, LC_MESSAGES, LANG, and the macOS system language.
// Messages live in messages.go; every key must have all three translations
// with the same format verbs (enforced by the package tests).
package i18n

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Lang is a supported language.
type Lang string

const (
	EN Lang = "en"
	ZH Lang = "zh"
	JA Lang = "ja"
)

// Supported lists the languages in catalog order.
var Supported = []Lang{EN, ZH, JA}

var (
	mu      sync.RWMutex
	current Lang
	once    sync.Once
)

// Parse maps a locale string such as "zh_CN.UTF-8", "ja-JP" or "en" to a Lang.
func Parse(s string) (Lang, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.HasPrefix(s, "zh"):
		return ZH, true
	case strings.HasPrefix(s, "ja"):
		return JA, true
	case strings.HasPrefix(s, "en"):
		return EN, true
	}
	return "", false
}

func detect() Lang {
	for _, k := range []string{"CSWITCH_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if l, ok := Parse(os.Getenv(k)); ok {
			return l
		}
	}
	// Terminals often leave LANG unset or "C"; fall back to the macOS UI language.
	if out, err := exec.Command("/usr/bin/defaults", "read", "-g", "AppleLanguages").Output(); err == nil {
		for _, f := range strings.FieldsFunc(string(out), func(r rune) bool { return strings.ContainsRune("(), \n\"", r) }) {
			if l, ok := Parse(f); ok {
				return l
			}
		}
	}
	return EN
}

// Current returns the active language.
func Current() Lang {
	once.Do(func() {
		mu.Lock()
		if current == "" {
			current = detect()
		}
		mu.Unlock()
	})
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// SetLang overrides the language; it reports false for unsupported values.
func SetLang(s string) bool {
	l, ok := Parse(s)
	if !ok {
		return false
	}
	mu.Lock()
	current = l
	mu.Unlock()
	once.Do(func() {}) // skip detection
	return true
}

// T returns the translated message for key, formatted with args.
// Unknown keys are returned verbatim so a missing entry is visible, not fatal.
func T(key string, args ...any) string {
	m, ok := messages[key]
	if !ok {
		if len(args) > 0 {
			return key + fmt.Sprint(args...)
		}
		return key
	}
	format := m[EN]
	if s := m[Current()]; s != "" {
		format = s
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// E returns a translated error; %w in the message wraps an error argument.
func E(key string, args ...any) error {
	m, ok := messages[key]
	if !ok {
		return fmt.Errorf("%s", T(key, args...))
	}
	format := m[EN]
	if s := m[Current()]; s != "" {
		format = s
	}
	return fmt.Errorf(format, args...)
}
