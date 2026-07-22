//go:build !windows && !linux && !darwin

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package netfw

func platformName() string { return "unsupported" }

func presentPlatform(cfg Config) bool { return true }

func ensurePlatform(cfg Config) Result {
	return Result{
		Platform:    platformName(),
		NeedsAdmin:  true,
		UserMessage: "automatic firewall setup is not implemented on this OS; allow TCP port " + itoa(cfg.Port) + " manually if P2P is blocked",
	}
}

func manualPlatform(cfg Config) string {
	return "allow TCP " + itoa(cfg.Port) + " for " + cfg.ExePath
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
