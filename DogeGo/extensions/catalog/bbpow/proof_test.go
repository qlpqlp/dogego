// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package bbpow

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"dogego/pow"
	"dogego/wire"
)

func TestBuildCommitmentRoundTrip(t *testing.T) {
	// Arbitrary display hash.
	disp := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	payload, err := BuildCommitmentPayload(disp)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) != 36 || !bytes.Equal(payload[:4], CommitmentMagic) {
		t.Fatalf("payload %x", payload)
	}
	hx, err := BuildCommitmentHex(disp)
	if err != nil || hx != hex.EncodeToString(payload) {
		t.Fatalf("hex %s", hx)
	}
}

func TestValidateProofSingleTxBlock(t *testing.T) {
	dogeDisp := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	dogeLE, err := displayHexToLE32(dogeDisp)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := BuildCommitmentPayload(dogeDisp)
	if err != nil {
		t.Fatal(err)
	}

	// Coinbase with OP_RETURN commitment.
	cb := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{},
			PrevIdx:  0xffffffff,
			Script:   []byte{0x01, 0x00},
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{
			Value:    50e8,
			PkScript: append([]byte{0x6a, byte(len(commit))}, commit...),
		}},
		LockTime: 0,
	}
	cbRaw, err := cb.Serialize()
	if err != nil {
		t.Fatal(err)
	}

	txHash := cb.TxHash()
	// Single-tx merkle root = txid.
	var h80 [80]byte
	binary.LittleEndian.PutUint32(h80[0:4], 1) // version
	copy(h80[36:68], txHash[:])
	binary.LittleEndian.PutUint32(h80[68:72], 1700000000)
	bits := uint32(0x220000ff) // max-ish compact: any hash meets target (research only)
	binary.LittleEndian.PutUint32(h80[72:76], bits)
	binary.LittleEndian.PutUint32(h80[76:80], 0)
	if err := CheckSHA256PoW(h80[:], bits); err != nil {
		t.Fatalf("easy pow: %v", err)
	}

	p := Proof{
		Version:          1,
		DogeBlockHash:    dogeDisp,
		BitcoinHeaderHex: hex.EncodeToString(h80[:]),
		CoinbaseTxHex:    hex.EncodeToString(cbRaw),
		MerkleBranchHex:  nil,
		MerkleIndex:      0,
	}
	res := ValidateProof(p)
	if !res.OK {
		t.Fatalf("validate: %+v", res)
	}
	if !res.MerkleOK || !res.SHA256PoWOK || res.CommitmentWhere != "op_return" {
		t.Fatalf("%+v", res)
	}
	_ = dogeLE
}

func TestCheckSHA256PoWMainnetRejectsEasyOverLimit(t *testing.T) {
	var h80 [80]byte
	bits := uint32(0x220000ff)
	binary.LittleEndian.PutUint32(h80[72:76], bits)
	if err := CheckSHA256PoW(h80[:], bits); err != nil {
		t.Fatalf("research pow should pass: %v", err)
	}
	if err := CheckSHA256PoWMainnet(h80[:], bits); err == nil {
		t.Fatal("mainnet limit should reject over-limit target")
	}
}

func TestCheckSHA256PoWRejectsHardTarget(t *testing.T) {
	var h80 [80]byte
	binary.LittleEndian.PutUint32(h80[72:76], 0x17000000) // very hard compact
	if err := CheckSHA256PoW(h80[:], 0x17000000); err == nil {
		t.Fatal("expected reject")
	}
}

func TestDualModelDominance(t *testing.T) {
	m := NewDualDifficultyModel()
	for i := 0; i < 20; i++ {
		m.RecordLaneBlock(LaneScrypt, 0)
	}
	m.RecordLaneBlock(LaneSHA256, 0)
	if w := m.DominanceWarning(); w == "" {
		t.Fatal("expected scrypt dominance warning")
	}
	snap := m.Snapshot()
	if snap["scrypt_blocks"].(int64) != 20 {
		t.Fatalf("%v", snap)
	}
}

func TestCompareToAuxPoW(t *testing.T) {
	c := CompareToAuxPoW()
	if c["dogecoin_change"] != "hard_fork" {
		t.Fatalf("%v", c)
	}
	sf, ok := c["soft_fork"].(map[string]interface{})
	if !ok || sf["or_bitcoin_instead_of_scrypt"] != false {
		t.Fatalf("soft_fork %#v", c["soft_fork"])
	}
	asics, ok := c["asics"].(map[string]interface{})
	if !ok || asics["one_asic_both_algos"] != false {
		t.Fatalf("asics %#v", c["asics"])
	}
}

func TestMerkleBranchTwoTx(t *testing.T) {
	a := [32]byte{1}
	b := [32]byte{2}
	root := pow.CheckMerkleBranch(a, [][32]byte{b}, 0)
	// Manual: Hash(a||b)
	var h80 [80]byte
	copy(h80[36:68], root[:])
	if root == [32]byte{} {
		t.Fatal("empty root")
	}
}
