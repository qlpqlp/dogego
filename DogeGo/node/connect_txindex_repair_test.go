// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"testing"

	"dogego/store"
)

func TestConnectErrNeedsTxIndexRepair(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("bad merkle root"), false},
		{errors.New("input 2: missing funding height"), true},
		{errors.New("funding height: block not in journal"), true},
	}
	for _, tc := range cases {
		if got := connectErrNeedsTxIndexRepair(tc.err); got != tc.want {
			t.Fatalf("connectErrNeedsTxIndexRepair(%v)=%v want %v", tc.err, got, tc.want)
		}
	}
}

func TestIsConnectStallErr(t *testing.T) {
	if !isConnectStallErr(errors.New("utxo sync: connect stalled at height 6856 (contiguous bodies through 10005)")) {
		t.Fatal("expected stall detection")
	}
	if isConnectStallErr(errors.New("utxo apply: disk full")) {
		t.Fatal("unexpected stall detection")
	}
}

func TestBlockStoreChainDirFromRaw(t *testing.T) {
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := &BlockStoreCtx{Raw: raw}
	if got := blockStoreChainDir(bs); got != dir {
		t.Fatalf("chainDir=%q want %q", got, dir)
	}
}
