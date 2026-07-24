//go:build windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package netfw

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func platformName() string { return "windows" }

const (
	ruleInPort  = "DogeGo P2P (TCP In)"
	ruleOutProg = "DogeGo (program out)"
)

func presentPlatform(cfg Config) bool {
	if cfg.Inbound && !ruleExists(ruleInPort) {
		return false
	}
	if cfg.Outbound && cfg.ExePath != "" && !ruleExists(ruleOutProg) {
		return false
	}
	return true
}

func ruleExists(name string) bool {
	out, err := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name="+name).CombinedOutput()
	if err != nil {
		return false
	}
	s := string(out)
	return strings.Contains(s, "Rule Name:") && !strings.Contains(s, "No rules match")
}

func ensurePlatform(cfg Config) Result {
	var applied []string
	try := func() (needsAdmin bool, err error) {
		if cfg.Inbound {
			if err := netshAddInbound(cfg.Port); err != nil {
				if isWindowsAdminErr(err) {
					return true, err
				}
				return false, err
			}
			applied = append(applied, ruleInPort)
		}
		if cfg.Outbound && cfg.ExePath != "" {
			if err := netshAddOutboundProg(cfg.ExePath); err != nil {
				if isWindowsAdminErr(err) {
					return true, err
				}
				return false, err
			}
			applied = append(applied, ruleOutProg)
		}
		return false, nil
	}
	needsAdmin, err := try()
	if err == nil {
		return Result{OK: true, Applied: applied, Platform: platformName(), UserMessage: "Windows Firewall rules added"}
	}
	if needsAdmin && cfg.Elevate {
		if ConfirmElevation(cfg) {
			if err2 := ensureElevated(cfg); err2 == nil && Present(cfg) {
				return Result{OK: true, Applied: applied, Platform: platformName(), UserMessage: "Windows Firewall rules added (administrator prompt)"}
			}
		}
	}
	res := Result{
		NeedsAdmin:  true,
		Platform:    platformName(),
		UserMessage: "Windows Firewall needs administrator approval (or run the commands from the dashboard)",
		Err:         err,
	}
	NotifyFirewallSetupNeeded(cfg, res)
	return res
}

func netshAddInbound(port int) error {
	return runNetsh(
		"advfirewall", "firewall", "add", "rule",
		"name="+ruleInPort,
		"dir=in", "action=allow", "protocol=TCP",
		"localport="+itoa(port),
		"profile=any", "enable=yes",
	)
}

func netshAddOutboundProg(exe string) error {
	return runNetsh(
		"advfirewall", "firewall", "add", "rule",
		"name="+ruleOutProg,
		"dir=out", "action=allow",
		"program="+exe,
		"profile=any", "enable=yes",
	)
}

func runNetsh(args ...string) error {
	out, err := exec.Command("netsh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh %s: %w (%s)", strings.Join(args, " "), err, bytesTrim(out))
	}
	if strings.Contains(string(out), "already exists") {
		return nil
	}
	return nil
}

func isWindowsAdminErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "access is denied") ||
		strings.Contains(s, "requires elevation") ||
		strings.Contains(s, "0x5")
}

func ensureElevated(cfg Config) error {
	script, err := writeFirewallScript(cfg)
	if err != nil {
		return err
	}
	defer os.Remove(script)
	// Launch elevated PowerShell to run the script (same pattern as installers).
	ps := fmt.Sprintf(
		`Start-Process -FilePath 'powershell.exe' -Verb RunAs -Wait -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File','%s'`,
		strings.ReplaceAll(script, "'", "''"))
	return exec.Command("powershell.exe", "-NoProfile", "-Command", ps).Run()
}

func writeFirewallScript(cfg Config) (string, error) {
	var lines []string
	lines = append(lines, "$ErrorActionPreference = 'Stop'")
	if cfg.Inbound {
		lines = append(lines, fmt.Sprintf(
			`netsh advfirewall firewall add rule name="%s" dir=in action=allow protocol=TCP localport=%d profile=any enable=yes`,
			ruleInPort, cfg.Port))
	}
	if cfg.Outbound && cfg.ExePath != "" {
		lines = append(lines, fmt.Sprintf(
			`netsh advfirewall firewall add rule name="%s" dir=out action=allow program="%s" profile=any enable=yes`,
			ruleOutProg, cfg.ExePath))
	}
	dir := os.TempDir()
	path := filepath.Join(dir, "dogego-firewall-setup.ps1")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func manualPlatform(cfg Config) string {
	var lines []string
	lines = append(lines, "Windows  -  elevated Command Prompt or PowerShell (Run as administrator):")
	if cfg.Inbound {
		lines = append(lines, fmt.Sprintf(
			`netsh advfirewall firewall add rule name="%s" dir=in action=allow protocol=TCP localport=%d profile=any enable=yes`,
			ruleInPort, cfg.Port))
	}
	if cfg.Outbound && cfg.ExePath != "" {
		lines = append(lines, fmt.Sprintf(
			`netsh advfirewall firewall add rule name="%s" dir=out action=allow program="%s" profile=any enable=yes`,
			ruleOutProg, cfg.ExePath))
	}
	lines = append(lines, "# Also allow DogeGo in any third-party antivirus / firewall suite.")
	return strings.Join(lines, "\n")
}

func bytesTrim(b []byte) string {
	return strings.TrimSpace(string(b))
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
