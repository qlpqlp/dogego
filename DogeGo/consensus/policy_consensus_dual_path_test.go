// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"errors"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
	"dogego/mempool"
	"dogego/primitives"
	"dogego/wire"
)

// TestPolicyVsConsensusMinRelaySeparation ensures consensus block connect accepts txs that
// mempool policy rejects for min relay fee (Core policy-vs-consensus bar).
func TestPolicyVsConsensusMinRelaySeparation(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x55
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	redeem := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	redeem = append(redeem, 0x88, 0xac)

	coinbase := minimalCoinbaseWireTx(t)
	var prev [32]byte
	prev[0] = 1
	view := mempoolStubPrevOutView{}
	view[outpointKey(prev, 0)] = PrevOut{Value: 200_000, PkScript: redeem}

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 200_000, PkScript: redeem}},
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 184_000, PkScript: []byte{0x51}}},
	}
	digest, err := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)

	fundHash := funding.TxHash()
	view[outpointKey(fundHash, 0)] = PrevOut{Value: 200_000, PkScript: redeem}
	if err := CheckMinRelayFee(spend, view, DefaultMinRelayTxFeePerKB); err == nil {
		t.Fatalf("expected mempool min relay rejection (fee %d, size %d)", 200_000-175_000, len(spend.SerializeForHash()))
	}
	if err := AcceptMempoolTx(spend, view); err == nil {
		t.Fatal("expected mempool admission reject")
	}

	parentTxs := []*wire.Tx{coinbase, funding} // funding indexed at height 1 for connect
	parentMerkle := wire.BlockMerkleRoot(parentTxs)
	parentHdr := primitives.BlockHeader{
		Version: 1, MerkleRoot: parentMerkle,
		Timestamp: 1747000001, Bits: 0x1e0ffff0, Nonce: 41,
	}
	var parentBlk bytes.Buffer
	ph80 := parentHdr.EncodeWire80()
	_, _ = parentBlk.Write(ph80[:])
	_ = wire.WriteCompactSize(&parentBlk, 2)
	for _, tx := range parentTxs {
		raw, _ := tx.Serialize()
		_, _ = parentBlk.Write(raw)
	}
	pid := mustBlockHashLEConnect(t, ph80[:])
	blocks := memBlocks{pid: parentBlk.Bytes()}
	idx := memTxIndex{txidDisplayFromLE(fundHash): {block: pid, idx: 1}}

	childTxs := []*wire.Tx{minimalCoinbaseWireTx(t), spend}
	childMerkle := wire.BlockMerkleRoot(childTxs)
	childHdr := primitives.BlockHeader{
		Version: 1, PrevBlock: pid, MerkleRoot: childMerkle,
		Timestamp: 1747000002, Bits: 0x1e0ffff0, Nonce: 43,
	}
	var childBlk bytes.Buffer
	ch80 := childHdr.EncodeWire80()
	_, _ = childBlk.Write(ch80[:])
	_ = wire.WriteCompactSize(&childBlk, 2)
	for _, tx := range childTxs {
		raw, _ := tx.Serialize()
		_, _ = childBlk.Write(raw)
	}
	chainView := &ChainPrevOutView{Index: idx, Raw: blocks}
	childRaw := childBlk.Bytes()
	hdr, err := wire.BlockHeaderFromPayload(childRaw)
	if err != nil {
		t.Fatal(err)
	}
	if err := ConnectBlockRaw(childRaw, hdr, 1, chain.RebootTestnet, chainView, idx, nil); err != nil {
		t.Fatalf("consensus connect with low-fee tx: %v", err)
	}
}

// TestPolicyVsConsensusCoinbaseSeparation: coinbase is valid in blocks but never in mempool.
func TestPolicyVsConsensusCoinbaseSeparation(t *testing.T) {
	cb := minimalCoinbaseWireTx(t)
	if err := AcceptMempoolTx(cb, &MempoolPrevOutView{Pool: mempool.New(10)}); !errors.Is(err, ErrMempoolCoinbase) {
		t.Fatalf("mempool: %v", err)
	}
	mr := wire.BlockMerkleRoot([]*wire.Tx{cb})
	hdr := primitives.BlockHeader{
		Version: 1, MerkleRoot: mr, Timestamp: 1747000000, Bits: 0x1e0ffff0, Nonce: 40,
	}
	var blk bytes.Buffer
	h80 := hdr.EncodeWire80()
	_, _ = blk.Write(h80[:])
	_ = wire.WriteCompactSize(&blk, 1)
	raw, _ := cb.Serialize()
	_, _ = blk.Write(raw)
	h, err := wire.BlockHeaderFromPayload(blk.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := ConnectBlockRaw(blk.Bytes(), h, 0, chain.RebootTestnet, nil, nil, nil); err != nil {
		t.Fatalf("connect genesis+coinbase: %v", err)
	}
}
