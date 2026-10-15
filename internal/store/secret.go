package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/dollarkillerx/my-claude/internal/fsutil"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/keychain"
)

// Profile secrets (Claude Code credentials, Desktop token snapshots) can be
// several KB, more than the keychain tool reliably accepts over stdin. They are
// encrypted with AES-256-GCM into files in the profile directory, and only the
// 32-byte key lives in the keychain (service "cswitch", account "master-key").
// The profile name and secret kind are bound in as associated data, so an
// encrypted file cannot be swapped into another profile or slot.

const masterKeyAccount = "master-key"

var (
	keyMu     sync.Mutex
	keyCache  []byte
	keyCacheS string // service the cached key belongs to
)

// masterKey returns the encryption key, creating it on first use.
func masterKey() ([]byte, error) {
	keyMu.Lock()
	defer keyMu.Unlock()
	svc := keychainService()
	if keyCache != nil && keyCacheS == svc {
		return keyCache, nil
	}
	if !useKeychain() {
		k, err := fileKey()
		if err != nil {
			return nil, err
		}
		keyCache, keyCacheS = k, svc
		return k, nil
	}
	h, err := keychain.Get(svc, masterKeyAccount)
	if errors.Is(err, keychain.ErrNotFound) {
		k := make([]byte, 32)
		if _, err := rand.Read(k); err != nil {
			return nil, err
		}
		if err := keychain.Set(svc, masterKeyAccount, hex.EncodeToString(k)); err != nil {
			return nil, err
		}
		keyCache, keyCacheS = k, svc
		return k, nil
	}
	if err != nil {
		return nil, err
	}
	k, err := hex.DecodeString(h)
	if err != nil || len(k) != 32 {
		return nil, fmt.Errorf("keychain item %s/%s is not a valid key", svc, masterKeyAccount)
	}
	keyCache, keyCacheS = k, svc
	return k, nil
}

// useKeychain reports whether the master key lives in the macOS keychain.
// Elsewhere it is a 0600 file, which protects the secrets exactly as well as
// Claude Code protects its own ~/.claude/.credentials.json on those systems.
func useKeychain() bool {
	return runtime.GOOS == "darwin" && os.Getenv("CSWITCH_KEY_FILE") == ""
}

func keyFilePath() string {
	if p := os.Getenv("CSWITCH_KEY_FILE"); p != "" {
		return p
	}
	return filepath.Join(Home(), "master.key")
}

// fileKey reads the master key file, creating it on first use.
func fileKey() ([]byte, error) {
	path := keyFilePath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		k := make([]byte, 32)
		if _, err := rand.Read(k); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
		// O_EXCL: if another cswitch created the key meanwhile, use theirs.
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if os.IsExist(err) {
			return fileKey()
		}
		if err != nil {
			return nil, err
		}
		if _, err := f.WriteString(hex.EncodeToString(k)); err != nil {
			f.Close()
			return nil, err
		}
		return k, f.Close()
	}
	if err != nil {
		return nil, err
	}
	k, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(k) != 32 {
		return nil, fmt.Errorf("%s is not a valid key", path)
	}
	return k, nil
}

func secretPath(name, kind string) string { return filepath.Join(Dir(name), kind+".enc") }

func aead() (cipher.AEAD, error) {
	k, err := masterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func associatedData(name, kind string) []byte { return []byte("cswitch/v1/" + name + "/" + kind) }

// Secret returns a stored secret; kind is "code" or "desktop".
func Secret(name, kind string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	data, err := os.ReadFile(secretPath(name, kind))
	if os.IsNotExist(err) {
		if !useKeychain() {
			return "", i18n.E("err.no_secret_"+kind, name)
		}
		// Profiles saved by early versions kept the secret in a keychain item.
		s, kerr := keychain.Get(keychainService(), name+"/"+kind)
		if errors.Is(kerr, keychain.ErrNotFound) {
			return "", i18n.E("err.no_secret_"+kind, name)
		}
		return s, kerr
	}
	if err != nil {
		return "", err
	}
	g, err := aead()
	if err != nil {
		return "", err
	}
	if len(data) < g.NonceSize() {
		return "", i18n.E("err.secret_corrupt", name, kind)
	}
	plain, err := g.Open(nil, data[:g.NonceSize()], data[g.NonceSize():], associatedData(name, kind))
	if err != nil {
		return "", i18n.E("err.secret_corrupt", name, kind)
	}
	return string(plain), nil
}

// failSetSecret lets tests inject a write failure.
var failSetSecret func(name, kind string) error

// SetSecret stores a secret; kind is "code" or "desktop".
func SetSecret(name, kind, value string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if failSetSecret != nil {
		if err := failSetSecret(name, kind); err != nil {
			return err
		}
	}
	g, err := aead()
	if err != nil {
		return err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	if err := os.MkdirAll(Dir(name), 0o700); err != nil {
		return err
	}
	sealed := g.Seal(nonce, nonce, []byte(value), associatedData(name, kind))
	if err := fsutil.AtomicWrite(secretPath(name, kind), sealed, 0o600); err != nil {
		return err
	}
	if !useKeychain() {
		return nil
	}
	// Drop a copy left in the keychain by an early version.
	return keychain.Delete(keychainService(), name+"/"+kind)
}

// DeleteSecret removes a stored secret; a missing one is not an error.
func DeleteSecret(name, kind string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if err := os.Remove(secretPath(name, kind)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if useKeychain() {
		return keychain.Delete(keychainService(), name+"/"+kind)
	}
	return nil
}
