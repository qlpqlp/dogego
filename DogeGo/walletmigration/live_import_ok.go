// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package walletmigration

import (
	"fmt"
	"strings"
)

// WalletDatProbeOptional reports whether a failed wallet.dat probe should be ignored
// (auto-discovered path, not explicitly configured, and not required).
func WalletDatProbeOptional(explicit, require bool) bool {
	return !explicit && !require
}

func RPCClientForHostPort(host string, port int) RPCClient {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	c := DefaultRPCClient()
	if port > 0 {
		c.BaseURL = fmt.Sprintf("http://%s:%d", host, port)
	}
	return c
}

// LiveImportOK reports whether a live RPC import result satisfies optional/required policy.
func LiveImportOK(live *LiveImportResult, requireWalletDat bool) bool {
	if live == nil {
		return !requireWalletDat
	}
	switch live.Status {
	case "passed", "passed_encrypted":
		return true
	case "skipped_needs_passphrase", "skipped_encrypted_or_blocked":
		return !requireWalletDat
	default:
		return false
	}
}

// LiveProbeOK reports whether a live RPC probe result satisfies optional/required policy.
// When requireWalletDat is true and a passphrase is supplied, set extractOK from a file dry-run extract.
func LiveProbeOK(live *LiveImportResult, requireWalletDat, extractOK bool) bool {
	if live == nil {
		return !requireWalletDat
	}
	switch live.Status {
	case "probe_passed":
		return true
	case "probe_needs_passphrase":
		if !requireWalletDat {
			return true
		}
		return extractOK
	case "probe_blocked", "probe_failed", "not_bdb":
		return false
	default:
		return false
	}
}
