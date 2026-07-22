// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "fmt"

// FormatServicesHex renders NODE_* service bits as a 16-digit lowercase hex string (getnetworkinfo / getpeerinfo).
func FormatServicesHex(services uint64) string {
	return fmt.Sprintf("%016x", services)
}

// LocalServiceNames maps NODE_* bits to Core-style service name strings (subset used by DogeGo).
func LocalServiceNames(services uint64) []string {
	var names []string
	if services&1 != 0 {
		names = append(names, "NETWORK")
	}
	if services&2 != 0 {
		names = append(names, "GETUTXO")
	}
	if services&4 != 0 {
		names = append(names, "BLOOM")
	}
	if services&8 != 0 {
		names = append(names, "WITNESS")
	}
	if services&64 != 0 { // NODE_COMPACT_FILTERS (1<<6)
		names = append(names, "COMPACT_FILTERS")
	}
	if services&(1<<10) != 0 {
		names = append(names, "NETWORK_LIMITED")
	}
	if len(names) == 0 {
		return []string{}
	}
	return names
}
