//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const launchAgentLabel = "com.dogego.node"

func platformName() string { return "darwin" }

func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}

func launchAgentBody(opts Options) string {
	argv := append([]string{opts.ExePath}, nodeArgv(opts)...)
	var argsXML strings.Builder
	for _, a := range argv {
		argsXML.WriteString("    <string>")
		argsXML.WriteString(xmlEscape(a))
		argsXML.WriteString("</string>\n")
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>` + launchAgentLabel + `</string>
  <key>ProgramArguments</key>
  <array>
` + argsXML.String() + `  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>DOGECOINCONF</key>
    <string>` + xmlEscape(opts.ConfPath) + `</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <false/>
</dict>
</plist>
`
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func guiDomain() string {
	out, err := exec.Command("id", "-u").Output()
	if err != nil {
		return "gui/501"
	}
	return "gui/" + strings.TrimSpace(string(out))
}

func applyPlatform(opts Options) error {
	path, err := launchAgentPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("autostart: mkdir LaunchAgents: %w", err)
	}
	if err := os.WriteFile(path, []byte(launchAgentBody(opts)), 0o644); err != nil {
		return fmt.Errorf("autostart: write plist: %w", err)
	}
	_ = exec.Command("launchctl", "bootout", guiDomain(), path).Run()
	out, err := exec.Command("launchctl", "bootstrap", guiDomain(), path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("autostart: launchctl bootstrap: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func removePlatform() error {
	path, err := launchAgentPath()
	if err != nil {
		return err
	}
	_ = exec.Command("launchctl", "bootout", guiDomain(), path).Run()
	_ = os.Remove(path)
	return nil
}

func statusPlatform() (installed bool, method, detail string) {
	method = "LaunchAgent"
	path, err := launchAgentPath()
	if err != nil {
		return false, method, ""
	}
	if _, err := os.Stat(path); err != nil {
		return false, method, ""
	}
	out, _ := exec.Command("launchctl", "print", guiDomain()+"/"+launchAgentLabel).CombinedOutput()
	if strings.Contains(string(out), launchAgentLabel) {
		return true, method, "Runs when you sign in to macOS"
	}
	return true, method, "LaunchAgent plist installed"
}
