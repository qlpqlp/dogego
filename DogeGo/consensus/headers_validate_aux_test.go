// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/wire"
)

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func auxChildHeader80() []byte {
	h := make([]byte, 80)
	// auxpow version bit + Dogecoin mainnet chain id in bits 16..29 (0x0062 = 98).
	binary.LittleEndian.PutUint32(h[0:4], 0x00620102)
	binary.LittleEndian.PutUint32(h[72:76], 0x1d00ffff)
	return h
}

// auxParentHeaderBaseline sets fields required by checkAuxPow beyond version (non-auxpow parent).
func auxParentHeaderBaseline(a *wire.AuxPow) {
	a.ParentHeader80[4] = 1
	binary.LittleEndian.PutUint32(a.ParentHeader80[68:72], 1_500_000_000)
	binary.LittleEndian.PutUint32(a.ParentHeader80[72:76], 0x1d00ffff)
	a.ParentHeader80[36] = 1
}

// wireAuxPowCoinbaseScript sets parent merkle root and merged-mining script (chain index 0, empty branches).
func wireAuxPowCoinbaseScript(a *wire.AuxPow, child80 []byte) {
	childHash := pow.BlockHashLE(child80)
	nRoot := pow.CheckMerkleBranch(childHash, a.ChainBranch, a.ChainIndex)
	rootScript := make([]byte, 32)
	copy(rootScript, nRoot[:])
	for i, j := 0, 31; i < j; i, j = i+1, j-1 {
		rootScript[i], rootScript[j] = rootScript[j], rootScript[i]
	}
	merkleHeight := uint(len(a.ChainBranch))
	cid := chainIDFromVersion(nVersionLE(child80))
	var nNonce uint32
	for n := uint32(0); n < 1<<20; n++ {
		if pow.AuxExpectedIndex(n, cid, merkleHeight) == int(a.ChainIndex) {
			nNonce = n
			break
		}
	}
	sig := append([]byte(nil), wire.MergedMiningHeader...)
	sig = append(sig, rootScript...)
	var sizeNonce [8]byte
	binary.LittleEndian.PutUint32(sizeNonce[0:4], uint32(1)<<merkleHeight)
	binary.LittleEndian.PutUint32(sizeNonce[4:8], nNonce)
	sig = append(sig, sizeNonce[:]...)
	a.Coinbase.Vin[0].Script = sig
	txh := a.Coinbase.TxHash()
	copy(a.ParentHeader80[36:68], txh[:])
	a.HashBlock = pow.BlockHashLE(a.ParentHeader80[:])
}

func minimalAuxPow(parentVer uint32) *wire.AuxPow {
	cb := &wire.Tx{
		Vin:  []wire.TxIn{{PrevIdx: 0xffffffff, Script: []byte{0x01, 0x00}}},
		Vout: []wire.TxOut{{Value: 0, PkScript: []byte{0x6a}}},
	}
	a := &wire.AuxPow{
		Coinbase:    cb,
		MerkleIndex: 0,
	}
	binary.LittleEndian.PutUint32(a.ParentHeader80[0:4], parentVer)
	return a
}

func TestCheckAuxPowAllowsParentTimestampAheadOfChild(t *testing.T) {
	// Core CAuxPow::check does not compare parent nTime to the child; mainnet skew can be hours.
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	child := auxChildHeader80()
	binary.LittleEndian.PutUint32(child[68:72], 1_600_000_000)
	binary.LittleEndian.PutUint32(a.ParentHeader80[68:72], 1_600_000_000+13*3600)
	wireAuxPowCoinbaseScript(a, child)
	err := checkAuxPow(child, a, dc)
	if err != nil && contains(err.Error(), "timestamp too far ahead") {
		t.Fatalf("must not reject parent-ahead-of-child nTime (Core parity): %v", err)
	}
}

func TestCheckAuxPowRejectsZeroParentPrevHash(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	clear(a.ParentHeader80[4:36]) // prev block hash must be non-zero
	err := checkAuxPow(auxChildHeader80(), a, dc)
	if err == nil {
		t.Fatal("expected error for zero parent prev hash")
	}
	if !contains(err.Error(), "prev block hash") {
		t.Fatalf("msg %q", err)
	}
}

func TestCheckAuxPowAllowsParentAuxpowVersionBit(t *testing.T) {
	// Core does not reject parent headers with VERSION_AUXPOW set.
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(0x20000100)
	auxParentHeaderBaseline(a)
	binary.LittleEndian.PutUint32(a.ParentHeader80[0:4], 0x20000100)
	child := auxChildHeader80()
	wireAuxPowCoinbaseScript(a, child)
	err := checkAuxPow(child, a, dc)
	if err != nil && contains(err.Error(), "must not be auxpow") {
		t.Fatalf("parent auxpow version bit must match Core (accept): %v", err)
	}
}

func TestCheckAuxPowRejectsZeroParentMerkleRoot(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(0x00000102)
	binary.LittleEndian.PutUint32(a.ParentHeader80[72:76], 0x1d00ffff)
	err := checkAuxPow(auxChildHeader80(), a, dc)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckAuxPowRejectsDuplicateChainRoot(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(0x00000102)
	binary.LittleEndian.PutUint32(a.ParentHeader80[72:76], 0x1d00ffff)
	childHash := pow.BlockHashLE(auxChildHeader80())
	nRoot := pow.CheckMerkleBranch(childHash, a.ChainBranch, a.ChainIndex)
	rootScript := make([]byte, 32)
	copy(rootScript, nRoot[:])
	for i, j := 0, 31; i < j; i, j = i+1, j-1 {
		rootScript[i], rootScript[j] = rootScript[j], rootScript[i]
	}
	sig := append([]byte(nil), wire.MergedMiningHeader...)
	sig = append(sig, rootScript...)
	sig = append(sig, rootScript...) // duplicate
	a.Coinbase.Vin[0].Script = sig
	err := checkAuxPow(auxChildHeader80(), a, dc)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckAuxPowRejectsZeroParentVersion(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(0x00000102)
	binary.LittleEndian.PutUint32(a.ParentHeader80[72:76], 0x1d00ffff)
	binary.LittleEndian.PutUint32(a.ParentHeader80[68:72], 1_500_000_000)
	a.ParentHeader80[0] = 0
	a.ParentHeader80[1] = 0
	a.ParentHeader80[2] = 0
	a.ParentHeader80[3] = 0
	err := checkAuxPow(auxChildHeader80(), a, dc)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckAuxPowRejectsZeroParentBits(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(0x00000102)
	binary.LittleEndian.PutUint32(a.ParentHeader80[68:72], 1_500_000_000)
	err := checkAuxPow(auxChildHeader80(), a, dc)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckAuxPowRejectsChainIndexOutOfRange(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(0x00000102)
	a.ChainBranch = [][32]byte{{}}
	a.ChainIndex = 2
	err := checkAuxPow(auxChildHeader80(), a, dc)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckAuxPowAllowsParentCoinbaseScriptOver100(t *testing.T) {
	// Core does not apply Dogecoin bad-cb-length to the aux parent coinbase.
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	child := auxChildHeader80()
	wireAuxPowCoinbaseScript(a, child)
	long := make([]byte, 101)
	copy(long, a.Coinbase.Vin[0].Script)
	for i := len(a.Coinbase.Vin[0].Script); i < len(long); i++ {
		long[i] = 0x00
	}
	a.Coinbase.Vin[0].Script = long
	txh := a.Coinbase.TxHash()
	copy(a.ParentHeader80[36:68], txh[:])
	a.HashBlock = pow.BlockHashLE(a.ParentHeader80[:])
	err := checkAuxPow(child, a, dc)
	if err != nil && contains(err.Error(), "bad-cb-length") {
		t.Fatalf("parent coinbase scriptSig >100 must match Core (accept): %v", err)
	}
}

func TestCheckAuxPowRejectsParentDogecoinChainID(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	child := auxChildHeader80()
	wireAuxPowCoinbaseScript(a, child)
	// Parent encodes Dogecoin chain id 0x62 in nVersion (Core CAuxPow::check).
	binary.LittleEndian.PutUint32(a.ParentHeader80[0:4], 0x00620000)
	err := checkAuxPow(child, a, dc)
	if err == nil || !contains(err.Error(), "same chain id") {
		t.Fatalf("err=%v", err)
	}
}

func TestCheckAuxPowAcceptsNonDogecoinParentChainID(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	child := auxChildHeader80()
	wireAuxPowCoinbaseScript(a, child)
	// Non-zero parent chain id that is not Dogecoin (0x62) - Core does not reject on chain id alone.
	binary.LittleEndian.PutUint32(a.ParentHeader80[0:4], 0x00010000)
	err := checkAuxPow(child, a, dc)
	if err != nil && (contains(err.Error(), "same chain id") || contains(err.Error(), "chain id must be zero")) {
		t.Fatalf("parent chain id 1 must not fail chain-id gate: %v", err)
	}
}

func TestCheckAuxPowRejectsParentPrevEqualsChildBlockHash(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	child := auxChildHeader80()
	wireAuxPowCoinbaseScript(a, child)
	childHash := pow.BlockHashLE(child)
	copy(a.ParentHeader80[4:36], childHash[:])
	a.HashBlock = pow.BlockHashLE(a.ParentHeader80[:])
	err := checkAuxPow(child, a, dc)
	if err == nil || !contains(err.Error(), "prev hash equals child block hash") {
		t.Fatalf("err=%v", err)
	}
}

func TestCheckAuxPowRejectsChainMerkleBranchOver30(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	child := auxChildHeader80()
	wireAuxPowCoinbaseScript(a, child)
	a.ChainBranch = make([][32]byte, 31)
	err := checkAuxPow(child, a, dc)
	if err == nil || !contains(err.Error(), "chain merkle branch too long") {
		t.Fatalf("err=%v", err)
	}
}

func TestCheckAuxPowRejectsCoinbaseMerkleBranchOver30(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	child := auxChildHeader80()
	wireAuxPowCoinbaseScript(a, child)
	a.MerkleBranch = make([][32]byte, 31)
	a.HashBlock = pow.BlockHashLE(a.ParentHeader80[:])
	err := checkAuxPow(child, a, dc)
	if err == nil || !contains(err.Error(), "merkle branch too long") {
		t.Fatalf("err=%v", err)
	}
}

func TestCheckAuxPowRejectsLegacyRootAfter20Bytes(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	child := auxChildHeader80()
	childHash := pow.BlockHashLE(child)
	nRoot := pow.CheckMerkleBranch(childHash, a.ChainBranch, a.ChainIndex)
	rootScript := make([]byte, 32)
	copy(rootScript, nRoot[:])
	for i, j := 0, 31; i < j; i, j = i+1, j-1 {
		rootScript[i], rootScript[j] = rootScript[j], rootScript[i]
	}
	merkleHeight := uint(len(a.ChainBranch))
	cid := chainIDFromVersion(nVersionLE(child))
	var nNonce uint32
	for n := uint32(0); n < 1<<20; n++ {
		if pow.AuxExpectedIndex(n, cid, merkleHeight) == int(a.ChainIndex) {
			nNonce = n
			break
		}
	}
	var sizeNonce [8]byte
	binary.LittleEndian.PutUint32(sizeNonce[0:4], uint32(1)<<merkleHeight)
	binary.LittleEndian.PutUint32(sizeNonce[4:8], nNonce)
	// No MergedMiningHeader; root starts after 21 bytes (Core backward-compat cap).
	sig := append(bytes.Repeat([]byte{0x00}, 21), rootScript...)
	sig = append(sig, sizeNonce[:]...)
	a.Coinbase.Vin[0].Script = sig
	txh := a.Coinbase.TxHash()
	copy(a.ParentHeader80[36:68], txh[:])
	a.HashBlock = pow.BlockHashLE(a.ParentHeader80[:])
	err := checkAuxPow(child, a, dc)
	if err == nil || !contains(err.Error(), "first 20 bytes") {
		t.Fatalf("err=%v", err)
	}
}

func TestCheckAuxPowRejectsMultipleMergedMiningHeaders(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	child := auxChildHeader80()
	wireAuxPowCoinbaseScript(a, child)
	sig := a.Coinbase.Vin[0].Script
	// Second merged-mining header before the first (Core CAuxPow::check).
	sig = append(append(append([]byte(nil), wire.MergedMiningHeader...), wire.MergedMiningHeader...), sig[len(wire.MergedMiningHeader):]...)
	a.Coinbase.Vin[0].Script = sig
	txh := a.Coinbase.TxHash()
	copy(a.ParentHeader80[36:68], txh[:])
	a.HashBlock = pow.BlockHashLE(a.ParentHeader80[:])
	err := checkAuxPow(child, a, dc)
	if err == nil || !contains(err.Error(), "multiple merged mining headers") {
		t.Fatalf("err=%v", err)
	}
}

func TestCheckAuxPowRejectsMergedMiningHeaderNotBeforeRoot(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(2)
	auxParentHeaderBaseline(a)
	child := auxChildHeader80()
	wireAuxPowCoinbaseScript(a, child)
	sig := a.Coinbase.Vin[0].Script
	// Bytes between merged-mining header and chain root (Core CAuxPow::check).
	sig = append(append(append([]byte(nil), wire.MergedMiningHeader...), 0x01), sig[len(wire.MergedMiningHeader):]...)
	a.Coinbase.Vin[0].Script = sig
	txh := a.Coinbase.TxHash()
	copy(a.ParentHeader80[36:68], txh[:])
	a.HashBlock = pow.BlockHashLE(a.ParentHeader80[:])
	err := checkAuxPow(child, a, dc)
	if err == nil || !contains(err.Error(), "not just before chain merkle root") {
		t.Fatalf("err=%v", err)
	}
}

func TestCheckAuxPowRejectsNonCoinbaseParentTx(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 2_000_000)
	a := minimalAuxPow(0x00000102)
	a.Coinbase.Vin[0].PrevHash[0] = 1
	a.Coinbase.Vin[0].PrevIdx = 0
	err := checkAuxPow(auxChildHeader80(), a, dc)
	if err == nil {
		t.Fatal("expected error")
	}
}
