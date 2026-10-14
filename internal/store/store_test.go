package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setHome(t *testing.T) string {
	t.Helper()
	h := t.TempDir()
	t.Setenv("CSWITCH_HOME", h)
	// Keychain calls in Remove only ever delete, and only this test-only service.
	t.Setenv("CSWITCH_KEYCHAIN_SERVICE", fmt.Sprintf("cswitch-unit-%d", time.Now().UnixNano()))
	return h
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"a", "work", "my.profile_2-x", "A1"} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("ValidateName(%q) = %v", ok, err)
		}
	}
	long := ""
	for len(long) < 65 {
		long += "a"
	}
	for _, bad := range []string{"", ".hidden", "-x", "../x", "a/b", "a b", "名前", long, "a\nb"} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("ValidateName(%q) should fail", bad)
		}
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	h := setHome(t)
	p := &Profile{
		Name:             "work",
		AccountUUID:      "uuid-1",
		Email:            "me@example.com",
		OrganizationName: "Org",
		SubscriptionType: "max",
		OAuthAccount:     json.RawMessage(`{"accountUuid":"uuid-1"}`),
		HasCode:          true,
	}
	if err := Save(p); err != nil {
		t.Fatal(err)
	}
	if p.UpdatedAt.IsZero() {
		t.Error("Save did not set UpdatedAt")
	}
	fi, err := os.Stat(filepath.Join(h, "profiles", "work", "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("meta.json mode = %v", fi.Mode().Perm())
	}
	if di, _ := os.Stat(Dir("work")); di.Mode().Perm() != 0o700 {
		t.Errorf("profile dir mode = %v", di.Mode().Perm())
	}

	got, err := Load("work")
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != p.Email || got.AccountUUID != p.AccountUUID || !got.HasCode || got.HasDesktop {
		t.Errorf("loaded %+v", got)
	}
	// The raw oauthAccount may be re-indented, but must be the same JSON.
	var a, b any
	json.Unmarshal(got.OAuthAccount, &a)
	json.Unmarshal(p.OAuthAccount, &b)
	if fmt.Sprint(a) != fmt.Sprint(b) {
		t.Errorf("oauthAccount = %s", got.OAuthAccount)
	}
}

func TestLoadErrors(t *testing.T) {
	setHome(t)
	if _, err := Load("nobody"); !errors.Is(err, ErrNoProfile) {
		t.Errorf("missing profile: %v", err)
	}
	if _, err := Load("../etc"); err == nil || errors.Is(err, ErrNoProfile) {
		t.Errorf("invalid name: %v", err)
	}
	os.MkdirAll(Dir("broken"), 0o700)
	os.WriteFile(filepath.Join(Dir("broken"), "meta.json"), []byte("{"), 0o600)
	if _, err := Load("broken"); err == nil || errors.Is(err, ErrNoProfile) {
		t.Errorf("corrupt profile: %v", err)
	}
}

func TestLoadUsesDirectoryName(t *testing.T) {
	setHome(t)
	Save(&Profile{Name: "real"})
	// A hand-edited name inside meta.json must not redirect later writes.
	data, _ := os.ReadFile(filepath.Join(Dir("real"), "meta.json"))
	var m map[string]any
	json.Unmarshal(data, &m)
	m["name"] = "../../evil"
	data, _ = json.Marshal(m)
	os.WriteFile(filepath.Join(Dir("real"), "meta.json"), data, 0o600)

	p, err := Load("real")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "real" {
		t.Errorf("Name = %q", p.Name)
	}
}

func TestListSortedAndSkipsJunk(t *testing.T) {
	h := setHome(t)
	if ps, err := List(); err != nil || len(ps) != 0 {
		t.Fatalf("empty home: %v, %v", ps, err)
	}
	for _, n := range []string{"zeta", "alpha", "mid"} {
		if err := Save(&Profile{Name: n}); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Join(h, "profiles", "no-meta"), 0o700)
	os.MkdirAll(filepath.Join(h, "profiles", ".hidden"), 0o700)
	os.WriteFile(filepath.Join(h, "profiles", "stray-file"), nil, 0o600)
	listCache = nil

	ps, err := List()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range ps {
		names = append(names, p.Name)
	}
	if fmt.Sprint(names) != "[alpha mid zeta]" {
		t.Errorf("names = %v", names)
	}
}

func TestListCacheInvalidation(t *testing.T) {
	setHome(t)
	Save(&Profile{Name: "a", AccountUUID: "u-a"})
	if ps, _ := List(); len(ps) != 1 {
		t.Fatalf("len = %d", len(ps))
	}
	Save(&Profile{Name: "b", AccountUUID: "u-b"})
	if ps, _ := List(); len(ps) != 2 {
		t.Errorf("Save did not invalidate the cache: %d", len(ps))
	}

	// A different CSWITCH_HOME must never see another home's cached profiles.
	setHome(t)
	if ps, _ := List(); len(ps) != 0 {
		t.Errorf("cache leaked across homes: %d profiles", len(ps))
	}
}

func TestFindByAccount(t *testing.T) {
	setHome(t)
	Save(&Profile{Name: "a", AccountUUID: "u-a"})
	Save(&Profile{Name: "b", AccountUUID: "u-b"})

	if p, err := FindByAccount("u-b"); err != nil || p == nil || p.Name != "b" {
		t.Errorf("FindByAccount(u-b) = %v, %v", p, err)
	}
	if p, err := FindByAccount("u-none"); err != nil || p != nil {
		t.Errorf("unknown account = %v, %v", p, err)
	}
	if p, err := FindByAccount(""); err != nil || p != nil {
		t.Errorf("empty uuid must never match: %v, %v", p, err)
	}
}

func TestRemove(t *testing.T) {
	setHome(t)
	Save(&Profile{Name: "gone", AccountUUID: "u"})
	os.WriteFile(filepath.Join(Dir("gone"), "extra"), []byte("x"), 0o600)
	List() // populate the cache

	if err := Remove("gone"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(Dir("gone")); !os.IsNotExist(err) {
		t.Error("profile directory still exists")
	}
	if p, _ := FindByAccount("u"); p != nil {
		t.Error("removed profile still returned from cache")
	}
	if err := Remove("../x"); err == nil {
		t.Error("Remove must validate the name")
	}
}

func TestSaveRejectsInvalidName(t *testing.T) {
	h := setHome(t)
	if err := Save(&Profile{Name: "../escape"}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(filepath.Join(h, "escape")); !os.IsNotExist(err) {
		t.Error("wrote outside the profiles directory")
	}
}
