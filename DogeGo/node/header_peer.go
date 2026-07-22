// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
)

// recoverableHeaderPeerErr reports transport-level failures where trying another
// outbound peer may succeed (peer closed early, reset, stall).
func recoverableHeaderPeerErr(err error) bool {
	if err == nil {
		return false
	}
	if IsBenignShutdownErr(err) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) {
		if op.Timeout() {
			return true
		}
		if errors.Is(op.Err, syscall.ECONNRESET) || errors.Is(op.Err, syscall.EPIPE) {
			return true
		}
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNABORTED) {
		return true
	}
	// Broken pipe, wsasend abort, "connection was aborted", etc. (see isPermanentFetchErr).
	if isPermanentFetchErr(err) {
		return true
	}
	s := err.Error()
	if strings.Contains(s, "timeout waiting for headers") ||
		strings.Contains(s, "header sync stall") ||
		strings.Contains(s, "background catch-up required") {
		return true
	}
	if strings.Contains(s, "reject during headers sync") {
		return true
	}
	if strings.Contains(s, "header sync incomplete") {
		return true
	}
	if IsHeaderRewindRetryErr(err) {
		return false
	}
	if strings.Contains(s, "retry getheaders") {
		return true
	}
	if strings.Contains(s, "reset by peer") || strings.Contains(s, "Broken pipe") || strings.Contains(s, "broken pipe") {
		return true
	}
	if strings.Contains(s, "connection reset") || strings.Contains(s, "forcibly closed") ||
		strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "connection aborted") {
		return true
	}
	// Wrong-network peer or desynced stream - try next header-sync candidate.
	if isBadMagicP2PErr(err) {
		return true
	}
	// Invalid header chain from peer (difficulty, prev, pow) - try another candidate.
	if strings.Contains(s, "bad nBits") ||
		strings.Contains(s, "bad prev") ||
		strings.Contains(s, "nTime regressed") ||
		strings.Contains(s, " pow:") ||
		strings.Contains(s, "nextwork:") ||
		strings.Contains(s, "time too old") ||
		strings.Contains(s, "time too new") ||
		strings.Contains(s, "missing auxpow") ||
		strings.Contains(s, "unexpected auxpow") ||
		strings.Contains(s, "legacy block after auxpow") ||
		strings.Contains(s, "legacy scrypt header after auxpow") ||
		strings.Contains(s, "auxpow before activation") ||
		strings.Contains(s, "auxpow header before activation") ||
		strings.Contains(s, "wrong auxpow chain id") ||
		strings.Contains(s, "checkpoint hash mismatch") ||
		strings.Contains(s, "aux hash block mismatch") ||
		strings.Contains(s, " aux:") ||
		strings.Contains(s, "aux parent") ||
		strings.Contains(s, "aux decode:") {
		return true
	}
	return false
}

func isBadMagicP2PErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "bad magic")
}

// recoverablePrimarySessionErr reports primary read-loop failures where redialing may help.
func recoverablePrimarySessionErr(err error) bool {
	if errors.Is(err, ErrBlockDownloadStall) || errors.Is(err, ErrBlockDownloadTimeout) {
		return true
	}
	if errors.Is(err, ErrGenesisPeerNotFound) {
		return true
	}
	if shouldRedialPrimaryForAncientFetch(err, 0) {
		return true
	}
	return recoverableHeaderPeerErr(err)
}
