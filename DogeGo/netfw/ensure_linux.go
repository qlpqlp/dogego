//go:build linux

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

func platformName() string { return "linux" }

func presentPlatform(cfg Config) bool {
	switch detectBackend() {
	case backendUFW:
		return ufwHasPort(cfg.Port)
	case backendFirewalld:
		return firewalldHasPort(cfg.Port)
	default:
		return false
	}
}

type backend int

const (
	backendNone backend = iota
	backendUFW
	backendFirewalld
)

func detectBackend() backend {
	if out, err := exec.Command("ufw", "status").CombinedOutput(); err == nil {
		s := string(out)
		if strings.Contains(s, "Status: active") {
			return backendUFW
		}
	}
	if out, err := exec.Command("firewall-cmd", "--state").CombinedOutput(); err == nil {
		if strings.TrimSpace(string(out)) == "running" {
			return backendFirewalld
		}
	}
	return backendNone
}

func ensurePlatform(cfg Config) Result {
	b := detectBackend()
	switch b {
	case backendUFW:
		return ensureUFW(cfg)
	case backendFirewalld:
		return ensureFirewalld(cfg)
	default:
		return Result{
			Platform:    platformName(),
			NeedsAdmin:  true,
			UserMessage: "no active ufw or firewalld detected  -  open TCP " + itoa(cfg.Port) + " manually (commands below), or install/enable ufw or firewalld",
			Err:         fmt.Errorf("netfw: install and enable ufw or firewalld, or open TCP %d manually", cfg.Port),
		}
	}
}

func ensureUFW(cfg Config) Result {
	comment := "DogeGo P2P"
	args := []string{"allow", fmt.Sprintf("%d/tcp", cfg.Port), "comment", comment}
	if err := runMaybeSudo("ufw", args...); err != nil {
		return Result{
			NeedsAdmin:  true,
			Platform:    platformName(),
			UserMessage: "ufw rule needs root (try: sudo ufw allow " + itoa(cfg.Port) + "/tcp)",
			Err:         err,
		}
	}
	return Result{OK: true, Applied: []string{"ufw allow " + itoa(cfg.Port) + "/tcp"}, Platform: platformName(), UserMessage: "ufw allowed TCP " + itoa(cfg.Port)}
}

func ensureFirewalld(cfg Config) Result {
	portSpec := fmt.Sprintf("%d/tcp", cfg.Port)
	if err := runMaybeSudo("firewall-cmd", "--permanent", "--add-port="+portSpec); err != nil {
		return firewalldFail(cfg, err)
	}
	if err := runMaybeSudo("firewall-cmd", "--reload"); err != nil {
		return firewalldFail(cfg, err)
	}
	return Result{OK: true, Applied: []string{"firewall-cmd --add-port=" + portSpec}, Platform: platformName(), UserMessage: "firewalld opened TCP " + itoa(cfg.Port)}
}

func firewalldFail(cfg Config, err error) Result {
	return Result{
		NeedsAdmin:  true,
		Platform:    platformName(),
		UserMessage: "firewalld needs root",
		Err:         err,
	}
}

func ufwHasPort(port int) bool {
	out, err := exec.Command("ufw", "status").CombinedOutput()
	if err != nil {
		return false
	}
	needle := fmt.Sprintf("%d/tcp", port)
	return strings.Contains(string(out), needle) && strings.Contains(string(out), "ALLOW")
}

func firewalldHasPort(port int) bool {
	out, err := exec.Command("firewall-cmd", "--list-ports").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("%d/tcp", port))
}

func runMaybeSudo(name string, args ...string) error {
	err := exec.Command(name, args...).Run()
	if err == nil {
		return nil
	}
	if !needsRoot(err) {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	sudo := exec.Command("sudo", append([]string{"-n", name}, args...)...)
	out, err2 := sudo.CombinedOutput()
	if err2 == nil {
		return nil
	}
	return fmt.Errorf("%s: %w (%s); passwordless sudo failed: %v (%s)", name, err, bytesTrim(out), err2, bytesTrim(out))
}

func needsRoot(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "permission denied") || strings.Contains(s, "not permitted") || strings.Contains(s, "operation not permitted")
}

func manualPlatform(cfg Config) string {
	p := itoa(cfg.Port)
	return "Linux  -  run ONE of these in a terminal:\n" +
		"sudo ufw allow " + p + "/tcp comment 'DogeGo P2P'\n" +
		"sudo firewall-cmd --permanent --add-port=" + p + "/tcp && sudo firewall-cmd --reload\n" +
		"# If you use nftables/iptables only, allow TCP " + p + " inbound (and outbound if restricted).\n" +
		"# On DogeBox, the host/gateway may already allow P2P  -  use Dismiss if peers connect fine."
}

func bytesTrim(b []byte) string {
	return strings.TrimSpace(string(b))
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
