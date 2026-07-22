// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"testing"

	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestCountWindowTxsFromRawBlocks(t *testing.T) {
	txb := testMinimalCoinbase(t)
	rt := bytes.NewReader(txb)
	tx, err := wire.ReadTx(rt)
	if err != nil {
		t.Fatal(err)
	}
	th := tx.TxHash()
	hdr := primitives.BlockHeader{
		Version: 1, PrevBlock: [32]byte{}, MerkleRoot: th,
		Timestamp: 1747000000, Bits: 0x1e0ffff0, Nonce: 2139303,
	}
	var block bytes.Buffer
	h80 := hdr.EncodeWire80()
	_, _ = block.Write(h80[:])
	_ = wire.WriteCompactSize(&block, 1)
	_, _ = block.Write(txb)
	rawBytes := block.Bytes()

	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	blockID := pow.BlockHashLE(h80[:])
	if err := rs.Put(blockID, rawBytes); err != nil {
		t.Fatal(err)
	}
	best := pow.BlockHashHex(h80[:])
	j := &memJournal{tip: 0, best: best, gen: best, count: 1, hdrs: [][]byte{append([]byte(nil), h80[:]...)}}
	n, ok := countWindowTxsFromRawBlocks(j, rs, 0, 0)
	if !ok || n != 1 {
		t.Fatalf("ok=%v n=%d", ok, n)
	}
}
