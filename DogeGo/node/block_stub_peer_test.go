// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"testing"

	"dogego/chain"
	"dogego/store"
)

func TestShouldRotatePeerForStubBlock(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("timeout: no valid block"), false},
		{errors.New("raw block too short at height 10006: 213 bytes (need >= 280)"), true},
		{errors.New("batch incomplete: 1/1 block(s) missing; rejected 1 undersized stub(s)"), true},
	}
	for _, tc := range cases {
		if got := shouldRotatePeerForStubBlock(tc.err); got != tc.want {
			t.Fatalf("shouldRotatePeerForStubBlock(%q)=%v want %v", tc.err, got, tc.want)
		}
	}
}

func TestMinRawBlockBytesAllowsReal213ByteBlockAtHeight10006(t *testing.T) {
	// Mainnet block 10006 is a 213 B coinbase-only block (Blockchair / Core); not a pruned stub.
	if store.MinRawBlockBytes(chain.MainnetDogecoin, 10_006) != 140 {
		t.Fatalf("height 10006 min=%d want 140", store.MinRawBlockBytes(chain.MainnetDogecoin, 10_006))
	}
	if 213 < store.MinRawBlockBytes(chain.MainnetDogecoin, 10_006) {
		t.Fatal("213 B real block at 10006 must pass size floor")
	}
}

func TestSessionFailureHardFromStubBatchError(t *testing.T) {
	err := errors.New("batch incomplete: 1/1 block(s) missing (notfound or timeout); rejected 1 undersized stub(s)")
	if !sessionFailureHardFromFetchErr(err) {
		t.Fatal("stub batch error should be hard session failure")
	}
	if !shouldRotatePeerForStubBlock(err) {
		t.Fatal("stub batch error should rotate peer")
	}
}
