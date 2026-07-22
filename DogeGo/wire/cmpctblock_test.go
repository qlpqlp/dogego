// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire_test

import (
	"bytes"
	"crypto/rand"
	"testing"

	"dogego/pow"
	"dogego/primitives"
	"dogego/wire"
)

func TestCmpctBlockRoundtrip(t *testing.T) {
	txb := buildMinimalCoinbase(t)
	rt := bytes.NewReader(txb)
	tx, err := wire.ReadTx(rt)
	if err != nil {
		t.Fatal(err)
	}
	tx2b := buildMinimalCoinbase(t) // second tx with different value
	rt2 := bytes.NewReader(tx2b)
	tx2, err := wire.ReadTx(rt2)
	if err != nil {
		t.Fatal(err)
	}
	tx2.Vout[0].Value = 1
	tx2raw, err := tx2.Serialize()
	if err != nil {
		t.Fatal(err)
	}

	th := tx.TxHash()
	th2 := tx2.TxHash()
	merkle := wire.HashPair(th, th2)
	hdr := primitives.BlockHeader{
		Version:    1,
		PrevBlock:  [32]byte{},
		MerkleRoot: merkle,
		Timestamp:  1747000000,
		Bits:       0x1e0ffff0,
		Nonce:      2139303,
	}
	var block bytes.Buffer
	h80 := hdr.EncodeWire80()
	_, _ = block.Write(h80[:])
	_ = wire.WriteCompactSize(&block, 2)
	_, _ = block.Write(txb)
	_, _ = block.Write(tx2raw)

	coinRaw := txb
	var nonce uint64 = 0x123456789abcdef0
	sid1 := wire.CmpctShortTxID(h80[:], nonce, th2)
	hs := &wire.HeaderAndShortIDs{
		Header80: h80,
		Nonce:    nonce,
		ShortIDs: []uint64{sid1},
		Prefilled: []wire.PrefilledTransaction{
			{Index: 0, Tx: coinRaw},
		},
	}
	enc, err := wire.EncodeHeaderAndShortIDs(hs)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := wire.DecodeHeaderAndShortIDs(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Nonce != nonce || len(dec.ShortIDs) != 1 || dec.ShortIDs[0] != sid1 {
		t.Fatalf("decode mismatch: %+v", dec)
	}
	if len(dec.Prefilled) != 1 || dec.Prefilled[0].Index != 0 {
		t.Fatalf("prefilled %+v", dec.Prefilled)
	}

	gotBlock, err := wire.ReconstructBlockFromCmpct(dec, map[uint64][]byte{sid1: tx2raw}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBlock, block.Bytes()) {
		t.Fatalf("reconstructed block mismatch len %d vs %d", len(gotBlock), block.Len())
	}
	id := pow.BlockHashLE(h80[:])
	if err := wire.ValidateParsedBlock(mustParse(t, gotBlock), id); err != nil {
		t.Fatal(err)
	}
}

func TestBlockTransactionsRequestRoundtrip(t *testing.T) {
	var h [32]byte
	_, _ = rand.Read(h[:])
	req := &wire.BlockTransactionsRequest{
		BlockHash: h,
		Indexes:   []uint64{0, 2, 5},
	}
	pl, err := wire.EncodeBlockTransactionsRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	got, err := wire.DecodeBlockTransactionsRequest(pl)
	if err != nil {
		t.Fatal(err)
	}
	if got.BlockHash != h || len(got.Indexes) != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestSerializeBlockRoundtrip(t *testing.T) {
	txb := buildMinimalCoinbase(t)
	rt := bytes.NewReader(txb)
	tx, err := wire.ReadTx(rt)
	if err != nil {
		t.Fatal(err)
	}
	hdr := primitives.BlockHeader{
		Version:    1,
		PrevBlock:  [32]byte{},
		MerkleRoot: tx.TxHash(),
		Timestamp:  1747000000,
		Bits:       0x1e0ffff0,
		Nonce:      2139303,
	}
	pb := &wire.ParsedBlock{Header: hdr, Txs: []*wire.Tx{tx}}
	ser, err := wire.SerializeBlock(pb)
	if err != nil {
		t.Fatal(err)
	}
	pb2, err := wire.ParseBlock(ser)
	if err != nil {
		t.Fatal(err)
	}
	if len(pb2.Txs) != 1 {
		t.Fatal(pb2)
	}
}

func mustParse(t *testing.T, raw []byte) *wire.ParsedBlock {
	t.Helper()
	pb, err := wire.ParseBlock(raw)
	if err != nil {
		t.Fatal(err)
	}
	return pb
}