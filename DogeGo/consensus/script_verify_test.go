// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
	"dogego/mempool"
	"dogego/pow"
	"dogego/primitives"
	"dogego/wire"
)

func TestVerifyScriptP2PKHRoundTrip(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x11
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
	fundRaw, err := funding.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	pool := mempool.New(100)
	if err := pool.Add(fundRaw); err != nil {
		t.Fatal(err)
	}

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: []byte{0x51}}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	spend.Vin[0].Script = buildP2PKHScriptSig(sigBytes, pubC)

	view := &MempoolPrevOutView{Pool: pool}
	if err := VerifyScript(spend, view); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyScriptP2PKHPushdata1ScriptSig(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x22
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{8}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 500_000_000, PkScript: pkScript}},
	}
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: funding.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 400_000_000, PkScript: []byte{0x51}}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var scriptSig []byte
	scriptSig = append(scriptSig, 0x4c, byte(len(sigBytes)))
	scriptSig = append(scriptSig, sigBytes...)
	scriptSig = append(scriptSig, byte(len(pubC)))
	scriptSig = append(scriptSig, pubC...)
	spend.Vin[0].Script = scriptSig

	view := stubPrevOutView{outpointKey(funding.TxHash(), 0): {Value: 500_000_000, PkScript: pkScript}}
	if err := VerifyScript(spend, view); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyScriptSpendFromIndexedBlock(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x33
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	redeem := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	redeem = append(redeem, 0x88, 0xac)

	coinbase := minimalCoinbaseWireTx(t)
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{7}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2_000_000_000, PkScript: redeem}},
	}
	txs := []*wire.Tx{coinbase, funding}
	merkle := wire.BlockMerkleRoot(txs)
	hdr := primitives.BlockHeader{
		Version: 1, PrevBlock: [32]byte{}, MerkleRoot: merkle,
		Timestamp: 1747000001, Bits: 0x1e0ffff0, Nonce: 42,
	}
	var block bytes.Buffer
	h80 := hdr.EncodeWire80()
	_, _ = block.Write(h80[:])
	_ = wire.WriteCompactSize(&block, 2)
	for _, tx := range txs {
		raw, _ := tx.Serialize()
		_, _ = block.Write(raw)
	}
	blockID := mustBlockHashLE(t, h80[:])
	txid := txidDisplayFromLE(funding.TxHash())
	idx := memTxIndex{txid: {block: blockID, idx: 1}}
	blocks := memBlocks{blockID: block.Bytes()}

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1_500_000_000, PkScript: append(append([]byte{0x76, 0xa9, 0x14}, h160[:]...), 0x88, 0xac)}},
	}
	digest, err := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	if err != nil {
		t.Fatal(err)
	}
	sig := ecdsa.Sign(priv, digest[:])
	spend.Vin[0].Script = buildP2PKHScriptSig(append(sig.Serialize(), byte(wire.SigHashAll)), pubC)

	adm := NewMempoolAdmission(nil, nil, idx, blocks, nil, chain.RebootTestnet)
	if err := AcceptMempoolTxAdmission(spend, adm); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyScriptP2SHP2PK(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x46
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	redeem := append([]byte{0x21}, pubC...)
	redeem = append(redeem, 0xac)
	rh := hash160(redeem)
	p2sh := append([]byte{0xa9, 0x14}, rh[:]...)
	p2sh = append(p2sh, 0x87)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{11}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 2_000_000_000, PkScript: p2sh}},
	}
	pool := mempool.New(10)
	raw, _ := funding.Serialize()
	_ = pool.Add(raw)

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 1_500_000_000, PkScript: []byte{0x51}}},
	}
	digest, _ := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()

	view := &MempoolPrevOutView{Pool: pool}
	if err := VerifyScript(spend, view); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyScriptP2SH(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x44
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	redeem := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	redeem = append(redeem, 0x88, 0xac)
	rh := hash160(redeem)
	p2sh := append([]byte{0xa9, 0x14}, rh[:]...)
	p2sh = append(p2sh, 0x87)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{8}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: p2sh}},
	}
	fundRaw, _ := funding.Serialize()
	pool := mempool.New(10)
	_ = pool.Add(fundRaw)

	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 900_000_000, PkScript: []byte{0x51}}},
	}
	digest, _ := wire.CalcSignatureHashLegacy(redeem, wire.SigHashAll, spend, 0)
	sig := ecdsa.Sign(priv, digest[:])
	sigBytes := append(sig.Serialize(), byte(wire.SigHashAll))
	var script bytes.Buffer
	script.WriteByte(byte(len(sigBytes)))
	_, _ = script.Write(sigBytes)
	script.WriteByte(byte(len(pubC)))
	_, _ = script.Write(pubC)
	script.WriteByte(byte(len(redeem)))
	_, _ = script.Write(redeem)
	spend.Vin[0].Script = script.Bytes()

	view := AdmissionPrevOutView(pool, nil, nil)
	if err := VerifyScript(spend, view); err != nil {
		t.Fatal(err)
	}
}

func minimalCoinbaseWireTx(t *testing.T) *wire.Tx {
	t.Helper()
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, int32(1))
	_ = wire.WriteCompactSize(&buf, 1)
	var zeros [32]byte
	_, _ = buf.Write(zeros[:])
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_, _ = buf.Write([]byte{0x00})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_ = binary.Write(&buf, binary.LittleEndian, int64(8800000000))
	_ = wire.WriteCompactSize(&buf, 2)
	_, _ = buf.Write([]byte{0x51, 0x51})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	tx, err := wire.ReadTx(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func mustBlockHashLE(t *testing.T, h80 []byte) [32]byte {
	t.Helper()
	return pow.BlockHashLE(h80)
}

func TestAcceptMempoolTxMissingPrevout(t *testing.T) {
	pkScript := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	pkScript = append(pkScript, 0x88, 0xac)
	tx := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{1},
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout:     []wire.TxOut{{Value: HardDustLimitKoinu, PkScript: pkScript}},
		LockTime: 0,
	}
	err := AcceptMempoolTx(tx, &MempoolPrevOutView{Pool: mempool.New(10)})
	if !errors.Is(err, ErrMissingPrevout) {
		t.Fatalf("got %v", err)
	}
}

