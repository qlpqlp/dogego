//go:build linux

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func desktopEntryPath(scheme string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "dogego-url-"+scheme+".desktop"), nil
}

func registerURLSchemePlatform(scheme, exe string) error {
	path, err := desktopEntryPath(scheme)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`[Desktop Entry]
Name=DogeGo Node URL
Comment=Open DogeGo dashboard via %s://
Exec=%s open --url %%u
Terminal=false
Type=Application
NoDisplay=true
MimeType=x-scheme-handler/%s;
`, scheme, quoteDesktopExec(exe), scheme)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	_ = exec.Command("xdg-mime", "default", filepath.Base(path), "x-scheme-handler/"+scheme).Run()
	_ = exec.Command("update-desktop-database", filepath.Dir(path)).Run()
	return nil
}

func unregisterURLSchemePlatform(scheme string) error {
	path, err := desktopEntryPath(scheme)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func urlSchemeStatusPlatform(scheme string) (bool, string, error) {
	path, err := desktopEntryPath(scheme)
	if err != nil {
		return false, "", err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, strings.TrimSpace(string(b)), nil
}

func quoteDesktopExec(exe string) string {
	if strings.ContainsAny(exe, " \t") {
		return `"` + exe + `"`
	}
	return exe
}

func platformTraySupported() bool { return true }
