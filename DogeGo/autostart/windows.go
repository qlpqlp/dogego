//go:build windows

package autostart

import (
	"fmt"
	"os/exec"
	"strings"
)

const taskName = "DogeGo Node"

func platformName() string { return "windows" }

func windowsTaskCommand(opts Options) string {
	parts := []string{
		`set "DOGECOINCONF=` + opts.ConfPath + `"`,
		`"` + opts.ExePath + `"`,
	}
	parts = append(parts, nodeArgv(opts)...)
	return `cmd.exe /c ` + strings.Join(parts, " && ")
}

func applyPlatform(opts Options) error {
	tr := windowsTaskCommand(opts)
	cmd := exec.Command("schtasks", "/Create",
		"/TN", taskName,
		"/TR", tr,
		"/SC", "ONLOGON",
		"/F",
		"/RL", "LIMITED",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("autostart: schtasks create: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func removePlatform() error {
	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")
	out, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if strings.Contains(text, "cannot find") || strings.Contains(text, "does not exist") {
			return nil
		}
		return fmt.Errorf("autostart: schtasks delete: %w (%s)", err, text)
	}
	return nil
}

func statusPlatform() (installed bool, method, detail string) {
	method = "Task Scheduler (ONLOGON)"
	cmd := exec.Command("schtasks", "/Query", "/TN", taskName, "/FO", "LIST")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, method, ""
	}
	if strings.Contains(string(out), taskName) {
		return true, method, "Runs at Windows sign-in"
	}
	return false, method, ""
}
