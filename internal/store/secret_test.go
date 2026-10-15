package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dollarkillerx/my-claude/internal/keychain"
)

// These tests create one test-only keychain item (the master key) under a
// unique service and delete it afterwards.
func withKeychain(t *testing.T) {
	setHome(t)
	svc := keychainService()
	keyMu.Lock()
	keyCache = nil
	keyMu.Unlock()
	t.Cleanup(func() { keychain.Delete(svc, masterKeyAccount) })
	if _, err := masterKey(); err != nil {
		t.Skipf("keychain not writable here (CI without a login keychain?): %v", err)
	}
}

func TestSecretRoundTripLarge(t *testing.T) {
	withKeychain(t)
	Save(&Profile{Name: "a"})
	big := strings.Repeat("token-blob-", 2000) // ~22 KB, far past security -i's line limit
	if err := SetSecret("a", "desktop", big); err != nil {
		t.Fatal(err)
	}
	got, err := Secret("a", "desktop")
	if err != nil {
		t.Fatal(err)
	}
	if got != big {
		t.Errorf("round trip lost data: %d vs %d bytes", len(got), len(big))
	}
	raw, _ := os.ReadFile(secretPath("a", "desktop"))
	if strings.Contains(string(raw), "token-blob") {
		t.Error("secret file is not encrypted")
	}
	if fi, _ := os.Stat(secretPath("a", "desktop")); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", fi.Mode().Perm())
	}
}

func TestSecretBoundToProfileAndKind(t *testing.T) {
	withKeychain(t)
	Save(&Profile{Name: "a"})
	Save(&Profile{Name: "b"})
	SetSecret("a", "code", "alice-token")

	// Swapping an encrypted file into another profile or slot must not decrypt.
	data, _ := os.ReadFile(secretPath("a", "code"))
	os.WriteFile(secretPath("b", "code"), data, 0o600)
	os.WriteFile(secretPath("a", "desktop"), data, 0o600)
	if _, err := Secret("b", "code"); err == nil {
		t.Error("file from profile a decrypted as profile b")
	}
	if _, err := Secret("a", "desktop"); err == nil {
		t.Error("code secret decrypted as desktop secret")
	}

	// Tampering is detected.
	data[len(data)-1] ^= 1
	os.WriteFile(secretPath("a", "code"), data, 0o600)
	if _, err := Secret("a", "code"); err == nil {
		t.Error("tampered file decrypted")
	}
}

func TestSecretMissingAndLegacy(t *testing.T) {
	withKeychain(t)
	if !useKeychain() {
		t.Skip("legacy keychain items exist only on macOS")
	}
	Save(&Profile{Name: "old"})
	if _, err := Secret("old", "code"); err == nil {
		t.Error("expected error for missing secret")
	}

	// Profiles from early versions kept the secret in a keychain item.
	svc := keychainService()
	if err := keychain.Set(svc, "old/code", "legacy-token"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { keychain.Delete(svc, "old/code") })
	if got, err := Secret("old", "code"); err != nil || got != "legacy-token" {
		t.Errorf("legacy secret = %q, %v", got, err)
	}
	// Saving again moves it into the encrypted file and drops the old item.
	if err := SetSecret("old", "code", "new-token"); err != nil {
		t.Fatal(err)
	}
	if _, err := keychain.Get(svc, "old/code"); err != keychain.ErrNotFound {
		t.Errorf("legacy keychain item not removed: %v", err)
	}
	if got, _ := Secret("old", "code"); got != "new-token" {
		t.Errorf("after migration = %q", got)
	}
}

func TestSecretWithKeyFile(t *testing.T) {
	h := setHome(t)
	t.Setenv("CSWITCH_KEY_FILE", filepath.Join(h, "master.key"))
	keyMu.Lock()
	keyCache = nil
	keyMu.Unlock()

	Save(&Profile{Name: "a"})
	if err := SetSecret("a", "code", "file-backed"); err != nil {
		t.Fatal(err)
	}
	if got, err := Secret("a", "code"); err != nil || got != "file-backed" {
		t.Errorf("Secret = %q, %v", got, err)
	}
	fi, err := os.Stat(filepath.Join(h, "master.key"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("key file: %v, %v", fi, err)
	}

	// A fresh process (empty cache) reads the same key back.
	keyMu.Lock()
	keyCache = nil
	keyMu.Unlock()
	if got, err := Secret("a", "code"); err != nil || got != "file-backed" {
		t.Errorf("after reload = %q, %v", got, err)
	}

	os.WriteFile(filepath.Join(h, "master.key"), []byte("short"), 0o600)
	keyMu.Lock()
	keyCache = nil
	keyMu.Unlock()
	if _, err := Secret("a", "code"); err == nil {
		t.Error("invalid key file accepted")
	}
}

func TestRenameReencryptsSecrets(t *testing.T) {
	h := setHome(t)
	t.Setenv("CSWITCH_KEY_FILE", filepath.Join(h, "master.key"))
	keyMu.Lock()
	keyCache = nil
	keyMu.Unlock()

	Save(&Profile{Name: "desktop-abc", AccountUUID: "u", HasCode: true, HasDesktop: true})
	SetSecret("desktop-abc", "code", "code-secret")
	SetSecret("desktop-abc", "desktop", "desk-secret")
	Save(&Profile{Name: "taken"})

	if err := Rename("desktop-abc", "taken"); err == nil {
		t.Error("renaming onto an existing profile should fail")
	}
	if err := Rename("desktop-abc", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("desktop-abc"); err == nil {
		t.Error("old name still exists")
	}
	p, err := Load("alpha")
	if err != nil || p.AccountUUID != "u" {
		t.Fatalf("renamed = %+v, %v", p, err)
	}
	for kind, want := range map[string]string{"code": "code-secret", "desktop": "desk-secret"} {
		if got, err := Secret("alpha", kind); err != nil || got != want {
			t.Errorf("%s after rename = %q, %v", kind, got, err)
		}
	}
	if found, _ := FindByAccount("u"); found == nil || found.Name != "alpha" {
		t.Errorf("FindByAccount after rename = %+v", found)
	}
}

// A rename that fails halfway must leave every secret readable under the old
// name: secrets are bound to the name they were encrypted for.
func TestRenameFailureKeepsSecretsReadable(t *testing.T) {
	h := setHome(t)
	t.Setenv("CSWITCH_KEY_FILE", filepath.Join(h, "master.key"))
	keyMu.Lock()
	keyCache = nil
	keyMu.Unlock()

	Save(&Profile{Name: "old", AccountUUID: "u", HasCode: true, HasDesktop: true})
	SetSecret("old", "code", "code-secret")
	SetSecret("old", "desktop", "desk-secret")

	writes := 0
	failSetSecret = func(name, kind string) error {
		if name == "new" {
			writes++
			if writes == 2 { // the first secret is re-encrypted, the second fails
				return errors.New("disk full")
			}
		}
		return nil
	}
	defer func() { failSetSecret = nil }()

	if err := Rename("old", "new"); err == nil {
		t.Fatal("expected the injected failure")
	}
	if _, err := Load("new"); err == nil {
		t.Error("half-renamed profile left under the new name")
	}
	for kind, want := range map[string]string{"code": "code-secret", "desktop": "desk-secret"} {
		if got, err := Secret("old", kind); err != nil || got != want {
			t.Errorf("%s after failed rename = %q, %v", kind, got, err)
		}
	}
}

func TestReservedNames(t *testing.T) {
	Reserve("sync", "ls")
	if err := ValidateNewName("sync"); err == nil {
		t.Error("reserved name accepted")
	}
	if err := ValidateNewName("LS"); err == nil {
		t.Error("reserved names must match case-insensitively")
	}
	if err := ValidateNewName("syncer"); err != nil {
		t.Errorf("unreserved name rejected: %v", err)
	}
	if err := ValidateName("sync"); err != nil {
		t.Error("existing profiles with reserved names must stay loadable")
	}
}

func TestDeleteSecret(t *testing.T) {
	h := setHome(t)
	t.Setenv("CSWITCH_KEY_FILE", filepath.Join(h, "master.key"))
	keyMu.Lock()
	keyCache = nil
	keyMu.Unlock()
	Save(&Profile{Name: "a"})
	SetSecret("a", "code", "x")
	if err := DeleteSecret("a", "code"); err != nil {
		t.Fatal(err)
	}
	if _, err := Secret("a", "code"); err == nil {
		t.Error("secret still readable")
	}
	if err := DeleteSecret("a", "code"); err != nil {
		t.Errorf("deleting a missing secret: %v", err)
	}
}
