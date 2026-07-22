// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"testing"
)

func TestShouldAutoRecoverHeaderSync(t *testing.T) {
	if !shouldAutoRecoverHeaderSync(errors.New("header batch index 0: bad nBits want 0x1d00ba8a")) {
		t.Fatal("bad nBits")
	}
	if !shouldAutoRecoverHeaderSync(errors.New("header batch index 1583 (chain height 371337 on mainnet): legacy scrypt header after auxpow activation")) {
		t.Fatal("auxpow boundary")
	}
	if !shouldAutoRecoverHeaderSync(errors.New("write tcp 127.0.0.1:22556->1.2.3.4:22556: wsasend: An established connection was aborted by the software in your host machine")) {
		t.Fatal("transport abort")
	}
	if shouldAutoRecoverHeaderSync(errors.New("headers: fork rejected (insufficient chain work)")) {
		t.Fatal("fork reject should not auto-recover")
	}
	if !shouldAutoRecoverHeaderSync(errors.New("headers: rewound journal to height 240 (retry getheaders)")) {
		t.Fatal("rewind retry")
	}
	if !shouldAutoRecoverHeaderSync(errors.New("no peer handshakes succeeded after 48 dial attempts")) {
		t.Fatal("probe failure")
	}
	if !shouldAutoRecoverHeaderSync(errors.New("no peer candidates for header recovery")) {
		t.Fatal("recovery candidates")
	}
	if !shouldAutoRecoverHeaderSync(errors.New("header 991 aux: aux parent chain id must be zero (litecoin merge-mining parent)")) {
		t.Fatal("obsolete aux gate should still trigger peer rotation recovery")
	}
	if isAuxpowValidationErr(errors.New("header 991 aux: aux parent chain id must be zero (litecoin merge-mining parent)")) {
		t.Fatal("obsolete gate is not journal auxpow corruption")
	}
	if !shouldAutoRecoverHeaderSync(errors.New("header batch index 0: header at height 371337: checkpoint hash mismatch (got abc… want def…)")) {
		t.Fatal("checkpoint mismatch")
	}
}

func TestParseCheckpointMismatchHeight(t *testing.T) {
	err := errors.New("header batch index 0 (chain height 371337 on mainnet): header at height 371337: checkpoint hash mismatch")
	h, ok := parseCheckpointMismatchHeight(err)
	if !ok || h != 371337 {
		t.Fatalf("height=%d ok=%v", h, ok)
	}
}

func TestShouldAttemptDeepHeaderRewind(t *testing.T) {
	if shouldAttemptDeepHeaderRewind(nil) {
		t.Fatal("nil should not deep-rewind")
	}
	if shouldAttemptDeepHeaderRewind(errors.New("timeout waiting for headers")) {
		t.Fatal("timeouts should rotate peers, not deep-rewind")
	}
	if shouldAttemptDeepHeaderRewind(errors.New("header sync stall: no headers for 30s")) {
		t.Fatal("stalls should not deep-rewind")
	}
	if !shouldAttemptDeepHeaderRewind(errors.New("header batch index 0: bad nBits want 0x1d00ba8a")) {
		t.Fatal("bad nBits should deep-rewind")
	}
	if !shouldAttemptDeepHeaderRewind(errors.New("header at height 371337: checkpoint hash mismatch")) {
		t.Fatal("checkpoint mismatch should deep-rewind")
	}
}
