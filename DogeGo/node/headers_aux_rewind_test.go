// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"testing"
)

func TestParseValidateHeaderIndex(t *testing.T) {
	idx, ok := parseValidateHeaderIndex(fmt.Errorf("header 1418 aux: aux hash block mismatch"))
	if !ok || idx != 1418 {
		t.Fatalf("idx=%d ok=%v", idx, ok)
	}
	idx, ok = parseValidateHeaderIndex(fmt.Errorf("header batch index 3 (chain height 371340 on mainnet): bad nBits"))
	if !ok || idx != 3 {
		t.Fatalf("batch idx=%d ok=%v", idx, ok)
	}
}

func TestRecoverableHeaderPeerErr_auxHashMismatch(t *testing.T) {
	if !recoverableHeaderPeerErr(fmt.Errorf("header 0 aux: aux hash block mismatch")) {
		t.Fatal("first-header aux failure should allow peer rotation")
	}
}
