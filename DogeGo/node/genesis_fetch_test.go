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

func TestShouldRedialPrimaryForAncientFetch(t *testing.T) {
	if !shouldRedialPrimaryForAncientFetch(ErrGenesisPeerNotFound, 0) {
		t.Fatal("genesis notfound should redial")
	}
	if shouldRedialPrimaryForAncientFetch(errors.New("timeout: no valid block"), 0) {
		t.Fatal("soft timeout should not redial")
	}
	if !shouldRedialPrimaryForAncientFetch(errors.New("batch incomplete: 1/16 block(s) missing (notfound or timeout)"), 100) {
		t.Fatal("ancient batch notfound should redial")
	}
	if shouldRedialPrimaryForAncientFetch(errors.New("batch incomplete: 1/16 block(s) missing (notfound or timeout)"), 10_000) {
		t.Fatal("high height batch notfound should not redial")
	}
	if !shouldRedialPrimaryForAncientFetch(errors.New("batch incomplete: 1/1 missing; rejected 1 undersized stub(s)"), 10_006) {
		t.Fatal("stub batch should redial at height 10006")
	}
}

func TestRecoverablePrimarySessionErrGenesis(t *testing.T) {
	if !recoverablePrimarySessionErr(ErrGenesisPeerNotFound) {
		t.Fatal("genesis notfound should be recoverable")
	}
}
