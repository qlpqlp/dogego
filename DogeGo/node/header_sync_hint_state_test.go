// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"strings"
	"testing"
)

func TestNoteHeaderSyncFailureStoresLastErr(t *testing.T) {
	clearHeaderSyncRecoveryHint()
	err := errors.New("legacy scrypt header after auxpow activation")
	noteHeaderSyncFailure(err)
	if headerSyncRecoveryHintStr() == "" {
		t.Fatal("expected recovery hint")
	}
	got := headerSyncLastFailure()
	if got == nil || got.Error() != err.Error() {
		t.Fatalf("last failure = %v", got)
	}
	clearHeaderSyncRecoveryHint()
	if headerSyncLastFailure() != nil {
		t.Fatal("clear should drop last failure")
	}
}

func TestHeaderSyncRecoveryHintObsoleteAuxParentChainGate(t *testing.T) {
	clearHeaderSyncRecoveryHint()
	err := errors.New("header 991 aux: aux parent chain id must be zero (litecoin merge-mining parent)")
	h := headerSyncRecoveryHintForErr(err)
	if h == "" || !strings.Contains(h, "outdated auxpow") {
		t.Fatalf("hint=%q", h)
	}
	if isAuxpowValidationErr(err) {
		t.Fatal("obsolete gate should not classify as journal auxpow damage")
	}
}
