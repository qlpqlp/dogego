// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/store"
	"dogego/wire"
)

type memTxIndex map[string]struct {
	block [32]byte
	idx   uint32
}

func (m memTxIndex) Lookup(txidHex string) ([32]byte, uint32, error) {
	e, ok := m[txidHex]
	if !ok {
		return [32]byte{}, 0, errNotFound("tx")
	}
	return e.block, e.idx, nil
}

type memBlocks map[[32]byte][]byte

func (m memBlocks) Get(h [32]byte) ([]byte, error) {
	b, ok := m[h]
	if !ok {
		return nil, errNotFound("block")
	}
	return b, nil
}

type errNotFound string

func (e errNotFound) Error() string { return string(e) + " not found" }

func TestChainPrevOutViewGenesisCoinbase(t *testing.T) {
	raw, blockID := store.TestMinimalBlock()
	pb, err := wire.ParseBlock(raw)
	if err != nil {
		t.Fatal(err)
	}
	funding := pb.Txs[0]
	txid := txidDisplayFromLE(funding.TxHash())

	idx := memTxIndex{txid: {block: blockID, idx: 0}}
	blocks := memBlocks{blockID: raw}

	v := &ChainPrevOutView{Index: idx, Raw: blocks}
	out, ok := v.Lookup(funding.TxHash(), 0)
	if !ok {
		t.Fatal("expected coinbase vout 0")
	}
	if out.Value != funding.Vout[0].Value {
		t.Fatalf("value %d want %d", out.Value, funding.Vout[0].Value)
	}
}
