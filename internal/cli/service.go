package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/dollarkillerx/my-claude/internal/fsutil"
	"github.com/dollarkillerx/my-claude/internal/i18n"
	"github.com/dollarkillerx/my-claude/internal/store"
)

// The auto-switcher runs in the background as a launchd LaunchAgent on macOS
// (started at login, restarted if it exits) or a systemd user service on
// Linux. Both run `cswitch auto` with the flags given to --install and log
// to ~/.config/cswitch/auto.log.

const serviceLabel = "com.github.dollarkillerx.cswitch.auto"

func autoLogPath() string { return filepath.Join(store.Home(), "auto.log") }

// launchAgentPath is the plist location (CSWITCH_SERVICE_DIR overrides the
// directory for testing).
func launchAgentPath() string {
	dir := os.Getenv("CSWITCH_SERVICE_DIR")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Library", "LaunchAgents")
	}
	return filepath.Join(dir, serviceLabel+".plist")
}

func systemdUnitPath() string {
	dir := os.Getenv("CSWITCH_SERVICE_DIR")
	if dir == "" {
		cfg, _ := os.UserConfigDir()
		dir = filepath.Join(cfg, "systemd", "user")
	}
	return filepath.Join(dir, "cswitch-auto.service")
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// launchAgentPlist renders the LaunchAgent for exe with args.
func launchAgentPlist(exe string, args []string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + serviceLabel + `</string>
	<key>ProgramArguments</key>
	<array>
`)
	for _, a := range append([]string{exe}, args...) {
		b.WriteString("\t\t<string>" + xmlEscape(a) + "</string>\n")
	}
	b.WriteString(`	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ThrottleInterval</key>
	<integer>30</integer>
	<key>StandardOutPath</key>
	<string>` + xmlEscape(autoLogPath()) + `</string>
	<key>StandardErrorPath</key>
	<string>` + xmlEscape(autoLogPath()) + `</string>
	<key>EnvironmentVariables</key>
	<dict>
		<key>PATH</key>
		<string>/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
	</dict>
</dict>
</plist>
`)
	return b.String()
}

// systemdUnit renders the systemd user unit for exe with args.
func systemdUnit(exe string, args []string) string {
	quoted := []string{strconv.Quote(exe)}
	for _, a := range args {
		quoted = append(quoted, strconv.Quote(a))
	}
	return `[Unit]
Description=cswitch auto-switcher for Claude accounts

[Service]
ExecStart=` + strings.Join(quoted, " ") + `
Restart=always
RestartSec=30
StandardOutput=append:` + autoLogPath() + `
StandardError=append:` + autoLogPath() + `

[Install]
WantedBy=default.target
`
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func installAutoService(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	if err := os.MkdirAll(store.Home(), 0o700); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		path := launchAgentPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := fsutil.AtomicWrite(path, []byte(launchAgentPlist(exe, args)), 0o644); err != nil {
			return err
		}
		domain := "gui/" + strconv.Itoa(os.Getuid())
		_ = run("launchctl", "bootout", domain+"/"+serviceLabel) // replace a running one
		if err := run("launchctl", "bootstrap", domain, path); err != nil {
			return err
		}
	case "linux":
		path := systemdUnitPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := fsutil.AtomicWrite(path, []byte(systemdUnit(exe, args)), 0o644); err != nil {
			return err
		}
		if err := run("systemctl", "--user", "daemon-reload"); err != nil {
			return err
		}
		if err := run("systemctl", "--user", "enable", "--now", "cswitch-auto.service"); err != nil {
			return err
		}
		_ = run("systemctl", "--user", "restart", "cswitch-auto.service")
	default:
		return i18n.E("err.auto_service_unsupported", runtime.GOOS)
	}
	info(i18n.T("info.auto_installed", strings.Join(args[1:], " "), autoLogPath()))
	return nil
}

func uninstallAutoService() error {
	switch runtime.GOOS {
	case "darwin":
		_ = run("launchctl", "bootout", "gui/"+strconv.Itoa(os.Getuid())+"/"+serviceLabel)
		if err := os.Remove(launchAgentPath()); err != nil && !os.IsNotExist(err) {
			return err
		}
	case "linux":
		_ = run("systemctl", "--user", "disable", "--now", "cswitch-auto.service")
		if err := os.Remove(systemdUnitPath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		_ = run("systemctl", "--user", "daemon-reload")
	default:
		return i18n.E("err.auto_service_unsupported", runtime.GOOS)
	}
	info(i18n.T("info.auto_uninstalled"))
	return nil
}

func autoServiceStatus() error {
	running := false
	installed := false
	switch runtime.GOOS {
	case "darwin":
		_, err := os.Stat(launchAgentPath())
		installed = err == nil
		running = exec.Command("launchctl", "print", "gui/"+strconv.Itoa(os.Getuid())+"/"+serviceLabel).Run() == nil
	case "linux":
		_, err := os.Stat(systemdUnitPath())
		installed = err == nil
		running = exec.Command("systemctl", "--user", "is-active", "--quiet", "cswitch-auto.service").Run() == nil
	default:
		return i18n.E("err.auto_service_unsupported", runtime.GOOS)
	}
	switch {
	case running:
		fmt.Println(i18n.T("auto.status_running", autoLogPath()))
	case installed:
		fmt.Println(i18n.T("auto.status_stopped", autoLogPath()))
	default:
		fmt.Println(i18n.T("auto.status_not_installed"))
	}
	if st := loadAutoState(); !st.LastSwitch.IsZero() {
		fmt.Println(i18n.T("auto.status_last_switch", st.LastSwitch.Local().Format("2006-01-02 15:04")))
	}
	return nil
}
