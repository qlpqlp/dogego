// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"strings"
)

// sessionFailureHardFromFetchErr reports whether a block-fetch error should use a long peer cooldown (Core: misbehavior / useless peer).
func sessionFailureHardFromFetchErr(err error) bool {
	if err == nil {
		return false
	}
	if IsBenignShutdownErr(err) {
		return false
	}
	if errors.Is(err, ErrBlockDownloadStall) || errors.Is(err, ErrBlockDownloadTimeout) {
		return true
	}
	s := err.Error()
	// Wire desync / wrong-network framing: hard-cooldown so assist workers stop hammering the same peer.
	if strings.Contains(s, "bad magic") {
		return true
	}
	if strings.Contains(s, "use of closed network connection") {
		return false
	}
	return strings.Contains(s, "reject") ||
		strings.Contains(s, "notfound") ||
		strings.Contains(s, "batch incomplete") ||
		strings.Contains(s, "missing (notfound") ||
		strings.Contains(s, "too short") ||
		strings.Contains(s, "undersized stub")
}

// shouldRotatePeerForForwardIBDFetch reports whether to disconnect after a block batch failure
// during forward IBD (Core: rotate peers that stall or timeout on ancient getdata).
func shouldRotatePeerForForwardIBDFetch(err error, wantHeight int64) bool {
	if err == nil {
		return false
	}
	if shouldRedialPrimaryForAncientFetch(err, wantHeight) {
		return true
	}
	if errors.Is(err, ErrBlockDownloadStall) || errors.Is(err, ErrBlockDownloadTimeout) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "bad magic") ||
		strings.Contains(s, "i/o timeout") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "forcibly closed") ||
		strings.Contains(s, "connection aborted") ||
		strings.Contains(s, "batch incomplete")
}
