// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"

	"dogego/pow"
)

func TestBitReaderTwentyZeros(t *testing.T) {
	br := newBitReader([]byte{0, 0, 0})
	for i := 0; i < 20; i++ {
		if _, err := br.readBit(); err != nil {
			t.Fatalf("bit %d: %v", i, err)
		}
	}
}

func TestGolombRiceBitRoundTrip(t *testing.T) {
	for _, v := range []uint64{0, 1, 100, 500000, 783000} {
		bw := &bitWriter{}
		golombRiceEncode(bw, BasicFilterP, v)
		data := bw.flush()
		br := newBitReader(data)
		got, err := br.readGolombRice(BasicFilterP)
		if err != nil || got != v {
			t.Fatalf("v=%d got=%d err=%v data=%x", v, got, err, data)
		}
	}
}

func TestDecodeBuiltBasicFilterHashedKeys(t *testing.T) {
	hashLE := [32]byte{9, 0x33, 0xea, 1}
	spk := []byte{0x51}
	enc := BuildBasicGCSFilter(hashLE, [][]byte{spk}, nil)
	keys, err := DecodeBasicFilterHashed(enc)
	if err != nil {
		t.Fatalf("decode: %v enc=%x", err, enc)
	}
	k0 := binary.LittleEndian.Uint64(hashLE[0:8])
	k1 := binary.LittleEndian.Uint64(hashLE[8:16])
	want := fastRange64(SipHash24(k0, k1, spk), uint64(len(keys))*BasicFilterM)
	if len(keys) != 1 || keys[0] != want {
		t.Fatalf("keys=%v want=%v enc=%x", keys, want, enc)
	}
}

func TestDecodeCoreGenesisFilter(t *testing.T) {
	vecs := loadCoreBlockFilterVectors(t)
	if len(vecs) == 0 {
		t.Fatal("no block filter vectors")
	}
	v := vecs[0]
	enc, err := hex.DecodeString(strings.TrimSpace(v.BasicFilter))
	if err != nil {
		t.Fatal(err)
	}
	blockRaw, _ := hex.DecodeString(v.Block)
	hashLE := pow.BlockHashLE(blockRaw[:80])
	built, err := buildBasicFilterFromCoreVector(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(built) != string(enc) {
		t.Fatalf("build %x enc %x", built, enc)
	}
	keys, err := DecodeBasicFilterHashed(enc)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("keys=%v want 2 (coinbase prevout + genesis output)", keys)
	}
	k0 := binary.LittleEndian.Uint64(hashLE[0:8])
	k1 := binary.LittleEndian.Uint64(hashLE[8:16])
	spk, _ := hex.DecodeString("4104678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5fac")
	wantPrev := fastRange64(SipHash24(k0, k1, spk), uint64(len(keys))*BasicFilterM)
	found := false
	for _, k := range keys {
		if k == wantPrev {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("keys=%v missing prevout script hash %v", keys, wantPrev)
	}
}
