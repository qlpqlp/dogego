//go:build !windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package netfw

// ConfirmElevation returns true when the user agrees to an OS admin prompt (non-Windows: always true).
func ConfirmElevation(cfg Config) bool {
	_ = cfg
	return true
}

// NotifyFirewallSetupNeeded shows a native dialog when rules could not be added (non-Windows: no UI).
func NotifyFirewallSetupNeeded(cfg Config, res Result) {
	_ = cfg
	_ = res
}

func thirdPartyFirewallNotePlatform() string {
	return "If you use a third-party firewall or antivirus, allow DogeGo and the chain P2P port there as well."
}
