//go:build darwin

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package netfw

import (
	"fmt"
	"os/exec"
	"strings"
)

const socketFilterFW = "/usr/libexec/ApplicationFirewall/socketfilterfw"

func platformName() string { return "darwin" }

func presentPlatform(cfg Config) bool {
	if cfg.ExePath == "" {
		return false
	}
	out, err := exec.Command(socketFilterFW, "--listapps").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), cfg.ExePath)
}

func ensurePlatform(cfg Config) Result {
	if cfg.ExePath == "" {
		return Result{Err: fmt.Errorf("netfw: executable path required on macOS")}
	}
	try := func(useSudo bool) error {
		name := socketFilterFW
		args := [][]string{
			{"--add", cfg.ExePath},
			{"--unblockapp", cfg.ExePath},
		}
		for _, a := range args {
			var cmd *exec.Cmd
			if useSudo {
				cmd = exec.Command("sudo", append([]string{"-n", name}, a...)...)
			} else {
				cmd = exec.Command(name, a...)
			}
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("%s %v: %w (%s)", name, a, err, strings.TrimSpace(string(out)))
			}
		}
		return nil
	}
	if err := try(false); err != nil {
		if err2 := try(true); err2 != nil {
			return Result{
				NeedsAdmin:  true,
				Platform:    platformName(),
				UserMessage: "macOS Application Firewall needs admin for socketfilterfw",
				Err:         err2,
			}
		}
	}
	if cfg.Inbound {
		_ = allowInboundPort(cfg.Port)
	}
	return Result{
		OK:          true,
		Applied:     []string{"socketfilterfw --add/--unblockapp"},
		Platform:    platformName(),
		UserMessage: "macOS allowed " + cfg.ExePath + " through Application Firewall",
	}
}

// allowInboundPort uses pf anchor when available; best-effort (many nodes rely on outbound dials).
func allowInboundPort(port int) error {
	// Documented approach for port is pf; skip auto pf edits to avoid breaking user rules.
	return nil
}

func manualPlatform(cfg Config) string {
	exe := cfg.ExePath
	if exe == "" {
		exe = "/path/to/dogego"
	}
	return "macOS — run in Terminal:\n" +
		"sudo " + socketFilterFW + " --add " + exe + "\n" +
		"sudo " + socketFilterFW + " --unblockapp " + exe + "\n" +
		"# Or: System Settings → Network → Firewall → Options → allow incoming for DogeGo\n" +
		"# For inbound listen on TCP " + itoa(cfg.Port) + ", allow that port if the Application Firewall is on."
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
