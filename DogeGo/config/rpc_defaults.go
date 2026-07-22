// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import "strings"

// DefaultRPCListenAddr returns the loopback JSON-RPC bind address for a full node when rpc is unset
// (Core always exposes RPC; DogeGo uses :22557 on mainnet so Core can keep :22555 for side-by-side).
func DefaultRPCListenAddr(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "testnet":
		return "127.0.0.1:44555"
	case "reboottestnet":
		return "127.0.0.1:44556"
	default:
		return "127.0.0.1:22557"
	}
}
