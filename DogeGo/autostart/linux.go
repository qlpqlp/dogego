//go:build linux

package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const unitName = "dogego.service"

func platformName() string { return "linux" }

func systemdUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", unitName), nil
}

func xdgDesktopPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "autostart", "dogego.desktop"), nil
}

func hasSystemctl() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func systemdUnitBody(opts Options) string {
	argv := nodeArgv(opts)
	var b strings.Builder
	b.WriteString("[Unit]\n")
	b.WriteString("Description=DogeGo Dogecoin Node\n")
	b.WriteString("After=network-online.target\n")
	b.WriteString("Wants=network-online.target\n\n")
	b.WriteString("[Service]\n")
	b.WriteString("Type=simple\n")
	b.WriteString("Environment=DOGECOINCONF=" + opts.ConfPath + "\n")
	b.WriteString("ExecStart=" + opts.ExePath)
	for _, a := range argv {
		b.WriteString(" ")
		if strings.ContainsAny(a, " \t") {
			b.WriteString(`"` + a + `"`)
		} else {
			b.WriteString(a)
		}
	}
	b.WriteString("\n")
	b.WriteString("Restart=on-failure\n")
	b.WriteString("RestartSec=30\n\n")
	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=default.target\n")
	return b.String()
}

func xdgDesktopBody(opts Options) string {
	argv := nodeArgv(opts)
	var execLine strings.Builder
	execLine.WriteString("env DOGECOINCONF=")
	execLine.WriteString(shellQuote(opts.ConfPath))
	execLine.WriteString(" ")
	execLine.WriteString(shellQuote(opts.ExePath))
	for _, a := range argv {
		execLine.WriteString(" ")
		execLine.WriteString(shellQuote(a))
	}
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=DogeGo Node\n" +
		"Comment=DogeGo Dogecoin node at login\n" +
		"Exec=" + execLine.String() + "\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n" +
		"NoDisplay=false\n"
}

func shellQuote(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t'\"\\$") {
		return s
	}
	return `'` + strings.ReplaceAll(s, `'`, `'\''`) + `'`
}

func applyPlatform(opts Options) error {
	if hasSystemctl() {
		path, err := systemdUnitPath()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("autostart: mkdir systemd user: %w", err)
		}
		if err := os.WriteFile(path, []byte(systemdUnitBody(opts)), 0o644); err != nil {
			return fmt.Errorf("autostart: write unit: %w", err)
		}
		if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
			return fmt.Errorf("autostart: systemctl daemon-reload: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		if out, err := exec.Command("systemctl", "--user", "enable", unitName).CombinedOutput(); err != nil {
			return fmt.Errorf("autostart: systemctl enable: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	path, err := xdgDesktopPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("autostart: mkdir autostart: %w", err)
	}
	if err := os.WriteFile(path, []byte(xdgDesktopBody(opts)), 0o644); err != nil {
		return fmt.Errorf("autostart: write desktop entry: %w", err)
	}
	return nil
}

func removePlatform() error {
	if hasSystemctl() {
		_ = exec.Command("systemctl", "--user", "disable", "--now", unitName).Run()
		path, err := systemdUnitPath()
		if err == nil {
			_ = os.Remove(path)
		}
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	}
	desktop, err := xdgDesktopPath()
	if err == nil {
		_ = os.Remove(desktop)
	}
	return nil
}

func statusPlatform() (installed bool, method, detail string) {
	if hasSystemctl() {
		method = "systemd user unit"
		path, err := systemdUnitPath()
		if err == nil {
			if _, err := os.Stat(path); err == nil {
				out, _ := exec.Command("systemctl", "--user", "is-enabled", unitName).CombinedOutput()
				state := strings.TrimSpace(string(out))
				if state == "enabled" || state == "enabled-runtime" {
					return true, method, "Enabled via systemctl --user; headless hosts may need loginctl enable-linger"
				}
				return true, method, "Unit file present (run: systemctl --user enable " + unitName + ")"
			}
		}
	}
	method = "XDG autostart"
	desktop, err := xdgDesktopPath()
	if err == nil {
		if _, err := os.Stat(desktop); err == nil {
			return true, method, "Runs when your desktop session starts"
		}
	}
	return false, method, ""
}
