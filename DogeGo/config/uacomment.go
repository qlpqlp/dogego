// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"fmt"
	"strings"

	"dogego/chain"
)

// UACommentUseNodeTipEnabled reports whether the dedicated HD node-tip key should be used.
func (f File) UACommentUseNodeTipEnabled() bool {
	return f.UACommentUseNodeTip != nil && *f.UACommentUseNodeTip
}

// UACommentPublishTip reports whether a tip address is configured for the P2P user-agent.
func (f File) UACommentPublishTip() bool {
	return strings.TrimSpace(f.UACommentTipAddress) != "" || f.UACommentUseNodeTipEnabled()
}

// EffectiveUAComment returns the wire user-agent comment (base text plus optional tip address).
func (f File) EffectiveUAComment() string {
	base := strings.TrimSpace(f.UAComment)
	tip := strings.TrimSpace(f.UACommentTipAddress)
	if tip == "" {
		return base
	}
	if base == "" {
		return tip
	}
	return base + "; " + tip
}

// ValidateUACommentTip checks tip address format against the configured network.
func ValidateUACommentTip(addr, network string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}
	payload, err := chain.DecodeBase58CheckBytes(addr)
	if err != nil || len(payload) != 21 {
		return fmt.Errorf("invalid dogecoin address %q", addr)
	}
	net := strings.ToLower(strings.TrimSpace(network))
	if net == "" {
		net = "testnet"
	}
	ver := payload[0]
	switch net {
	case "mainnet":
		if ver != 30 && ver != 22 {
			return fmt.Errorf("address %q is not a mainnet address", addr)
		}
	default:
		if ver != 0x41 && ver != 0x42 && ver != 0x71 && ver != 0xc4 {
			return fmt.Errorf("address %q is not a testnet address", addr)
		}
	}
	return nil
}
