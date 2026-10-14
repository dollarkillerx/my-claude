// Package keychain wraps /usr/bin/security for generic-password items.
package keychain

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"

	"github.com/dollarkillerx/my-claude/internal/i18n"
)

// ErrNotFound is returned when no matching item exists.
var ErrNotFound = errors.New("keychain item not found")

const securityBin = "/usr/bin/security"

// runQuiet is run without security's output in the error. When a write
// fails, security may echo the arguments, which contain the secret.
func runQuiet(stdin []byte, args ...string) ([]byte, error) {
	out, err := run(stdin, args...)
	var se *securityError
	if errors.As(err, &se) {
		return nil, fmt.Errorf("security %s: %v", se.cmd, se.err)
	}
	return out, err
}

type securityError struct {
	cmd    string
	err    error
	stderr string
}

func (e *securityError) Error() string {
	return fmt.Sprintf("security %s: %v: %s", e.cmd, e.err, e.stderr)
}

func run(stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command(securityBin, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		// Exit status 44 is errSecItemNotFound.
		if errors.As(err, &ee) && ee.ExitCode() == 44 {
			return nil, ErrNotFound
		}
		return nil, &securityError{cmd: args[0], err: err, stderr: strings.TrimSpace(stderr.String())}
	}
	return stdout.Bytes(), nil
}

// Get returns the secret of the item with the given service (and account, if non-empty).
//
// It uses -g rather than -w: -w prints non-printable or non-ASCII data as bare
// hex, indistinguishable from a secret that happens to be hex, while -g marks
// hex output with a 0x prefix.
func Get(service, account string) (string, error) {
	args := []string{"find-generic-password", "-s", service}
	if account != "" {
		args = append(args, "-a", account)
	}
	cmd := exec.Command(securityBin, append(args, "-g")...)
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = io.Discard, &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 44 {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("security find-generic-password: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return parsePassword(stderr.String())
}

// parsePassword extracts the secret from find-generic-password -g output:
//
//	password: "text"
//	password: 0x7B22...  "escaped preview"
//	password:            (empty)
func parsePassword(out string) (string, error) {
	const prefix = "password: "
	i := strings.Index(out, prefix)
	if i < 0 {
		if strings.Contains(out, "password:") {
			return "", nil
		}
		return "", errors.New("security: no password in output")
	}
	v := out[i+len(prefix):]
	if strings.HasPrefix(v, "0x") {
		h := v[2:]
		if j := strings.IndexAny(h, " \n"); j >= 0 {
			h = h[:j]
		}
		b, err := hex.DecodeString(h)
		if err != nil {
			return "", fmt.Errorf("security: bad hex password: %w", err)
		}
		return string(b), nil
	}
	// Printable data is printed raw between quotes and runs to the end of the
	// line (it cannot contain a newline, which would make it non-printable).
	if j := strings.IndexByte(v, '\n'); j >= 0 {
		v = v[:j]
	}
	if v == "" {
		return "", nil
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1], nil
	}
	return "", errors.New("security: unrecognized password format")
}

// acctRe matches the account attribute in find-generic-password output, either
// quoted ("acct"<blob>="name") or hex for non-ASCII names
// ("acct"<blob>=0x6E61...  "escaped preview").
var acctRe = regexp.MustCompile(`"acct"<blob>=(?:0x([0-9A-Fa-f]+)|"((?:[^"\\]|\\.)*)")`)

// Account returns the account attribute of the first item with the given service.
func Account(service string) (string, error) {
	out, err := run(nil, "find-generic-password", "-s", service)
	if err != nil {
		return "", err
	}
	return parseAccount(string(out), service)
}

func parseAccount(out, service string) (string, error) {
	m := acctRe.FindStringSubmatch(out)
	switch {
	case m == nil:
		return "", fmt.Errorf("keychain item %q has no account attribute", service)
	case m[1] != "":
		b, err := hex.DecodeString(m[1])
		if err != nil {
			return "", fmt.Errorf("keychain item %q: bad account attribute: %w", service, err)
		}
		return string(b), nil
	default:
		return m[2], nil
	}
}

// maxInteractiveLine is the longest command line security's interactive mode
// accepts; longer lines are silently truncated at about 4 KB.
const maxInteractiveLine = 4000

// Set creates or updates an item.
//
// Small secrets are passed hex-encoded over stdin (security's interactive
// mode), so they never appear in the process list. Interactive mode truncates
// lines at about 4 KB, so larger secrets have to go on the command line, the
// same way Claude Code writes its own item. They are then visible in the
// process list (to any local user) while security runs, so callers storing
// their own data keep secrets small: store keeps only a key in the keychain,
// and only Claude Code's own credentials item can take this path.
func Set(service, account, secret string) error {
	if strings.ContainsAny(service+account, "\"\n\\") {
		return fmt.Errorf("invalid characters in keychain service/account")
	}
	hexSecret := hex.EncodeToString([]byte(secret))
	line := fmt.Sprintf("add-generic-password -U -s \"%s\" -a \"%s\" -X %s\n", service, account, hexSecret)
	var err error
	if len(line) <= maxInteractiveLine {
		_, err = runQuiet([]byte(line), "-i")
	} else {
		_, err = runQuiet(nil, "add-generic-password", "-U", "-s", service, "-a", account, "-X", hexSecret)
	}
	if err != nil {
		return err
	}
	// Interactive mode does not reliably report failures through its exit code,
	// so read the item back to confirm the write.
	got, err := Get(service, account)
	if err != nil {
		return i18n.E("err.keychain_verify", err)
	}
	if got != secret {
		return i18n.E("err.keychain_verify", errors.New("stored value differs"))
	}
	return nil
}

// Delete removes an item; a missing item is not an error.
func Delete(service, account string) error {
	_, err := run(nil, "delete-generic-password", "-s", service, "-a", account)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
