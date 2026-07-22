// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import "strings"

// IsRebootTestnetNetwork reports testnet / reboot testnet slug (real scrypt PoW; RelaxedPoW=false).
func IsRebootTestnetNetwork(network string) bool {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "testnet", "reboottestnet":
		return true
	default:
		return false
	}
}

// ApplyTestnetAutoMine enables background mining on new reboot testnet installs (wallet payout).
func ApplyTestnetAutoMine(f *File) {
	if f == nil {
		return
	}
	if !IsRebootTestnetNetwork(f.Network) {
		return
	}
	if strings.ToLower(strings.TrimSpace(f.NodeMode)) == "spv" || f.NoWallet {
		return
	}
	f.Mine = true
}

// EffectiveMine resolves whether background mining runs this process.
// Reboot testnet defaults to on (wallet coinbase) unless -mine was set on the CLI.
func EffectiveMine(m Merged, cliMineVisited bool, cliMine bool) bool {
	if !IsRebootTestnetNetwork(m.Network) {
		return false
	}
	if strings.ToLower(strings.TrimSpace(m.NodeMode)) == "spv" || m.NoWallet {
		return false
	}
	if cliMineVisited {
		return cliMine
	}
	return true
}
