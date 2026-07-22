// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
	"dogego/primitives"
	"dogego/wire"
)

func TestConnectBlockSpendAndCoinbase(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x66
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	redeem := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	redeem = append(redeem, 0x88, 0xac)

	coinbase := minimalCoinbaseWireTx(t)
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2_000_000_000, PkScript: redeem}},
	}
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1_500_000_000, PkScript: []byte{0x51}}},
	}
	digest, err := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)

	parentTxs := []*wire.Tx{coinbase, funding}
	parentMerkle := wire.BlockMerkleRoot(parentTxs)
	parentHdr := primitives.BlockHeader{
		Version: 1, PrevBlock: [32]byte{}, MerkleRoot: parentMerkle,
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
	idx := memTxIndex{txidDisplayFromLE(funding.TxHash()): {block: pid, idx: 1}}

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
		t.Fatal(err)
	}
}

func mustBlockHashLEConnect(t *testing.T, h80 []byte) [32]byte {
	t.Helper()
	var id [32]byte
	copy(id[:], h80[:32])
	return id
}
