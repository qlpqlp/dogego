// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/store"
	"dogego/wire"
)

func TestMaybeRepairTxIndexDetectsSparseIndex(t *testing.T) {
	dir := t.TempDir()
	blockRaw, hash := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), blockRaw[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, raw, ix, store.NewUtxoCache())
	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	bs.Utxo.SetTipHeightForTest(0)
	var fundHash [32]byte
	_ = wire.ForEachBlockTx(blockRaw, func(_ uint32, tx *wire.Tx) error {
		fundHash = tx.TxHash()
		return nil
	})
	_ = os.Remove(filepath.Join(ix.RootDir(), store.TxIDIndexFileNameLE(fundHash)))
	sparse, err := store.TxIndexSparseThrough(j, raw, ix, p.Net, 0, 1)
	if err != nil || !sparse {
		t.Fatalf("want sparse before repair, sparse=%v err=%v", sparse, err)
	}
	maybeRepairTxIndex(dir, bs, 1)
	sparse, err = store.TxIndexSparseThrough(j, raw, ix, p.Net, 0, 1)
	if err != nil || sparse {
		t.Fatalf("want index complete after repair, sparse=%v err=%v", sparse, err)
	}
}

// storeTxidFileName mirrors store.txidRPCFileName for test deletes (same package cannot access unexported name).