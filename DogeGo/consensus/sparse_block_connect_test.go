// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestConnectSparseCoinbaseBlockRawRejectsMultiTx(t *testing.T) {
	coinbase := minimalCoinbaseWireTx(t)
	spend := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	txs := []*wire.Tx{coinbase, spend}
	hdr := primitives.BlockHeader{
		Version: 1, MerkleRoot: wire.BlockMerkleRoot(txs),
		Timestamp: 1747000002, Bits: 0x1e0ffff0, Nonce: 1,
	}
	var buf bytes.Buffer
	h80 := hdr.EncodeWire80()
	_, _ = buf.Write(h80[:])
	_ = wire.WriteCompactSize(&buf, 2)
	for _, tx := range txs {
		raw, err := tx.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = buf.Write(raw)
	}
	if err := ConnectSparseCoinbaseBlockRaw(buf.Bytes(), 1, chain.RebootTestnet); err == nil {
		t.Fatal("expected error for multi-tx block")
	}
}

func TestCoreMainnetFieldSparseCoinbaseConnect(t *testing.T) {
	sparseHeights := []int64{1, 2, 3, 100, 200, 272, 10006}
	byHeight := map[int64][]byte{}
	for _, e := range loadMainnetFieldBlockEntries(t) {
		decoded, err := hex.DecodeString(strings.TrimSpace(e.Hex))
		if err != nil {
			t.Fatalf("height %d: %v", e.Height, err)
		}
		byHeight[e.Height] = decoded
	}
	for _, h := range sparseHeights {
		h := h
		raw, ok := byHeight[h]
		if !ok {
			t.Fatalf("missing field block height %d", h)
		}
		t.Run(fmt.Sprintf("height_%d", h), func(t *testing.T) {
			if err := CheckBlockCoinbaseSubsidyPayload(raw, h, chain.MainnetDogecoin, nil); err != nil {
				t.Fatal(err)
			}
			if err := ConnectSparseCoinbaseBlockRaw(raw, h, chain.MainnetDogecoin); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMainnetFieldBlock10006SubsidyCap(t *testing.T) {
	var spec *mainnetCanonicalBlockSpec
	for i := range mainnetCanonicalBlockSpecs {
		if mainnetCanonicalBlockSpecs[i].Height == 10006 {
			spec = &mainnetCanonicalBlockSpecs[i]
			break
		}
	}
	if spec == nil {
		t.Fatal("missing height 10006 spec")
	}
	raw, err := buildMainnetCanonicalBlockRaw(*spec)
	if err != nil {
		t.Fatal(err)
	}
	prev, err := chain.Hash256FromDisplayHex(spec.PrevHash)
	if err != nil {
		t.Fatal(err)
	}
	subsidy := BlockSubsidy(spec.Height, prev, chain.MainnetDogecoin)
	const wantOut = int64(8649600000000) // Core coinbase vout at height 10006
	if wantOut > subsidy {
		t.Fatalf("coinbase out %d exceeds subsidy %d", wantOut, subsidy)
	}
	if err := CheckBlockCoinbaseSubsidyPayload(raw, spec.Height, chain.MainnetDogecoin, nil); err != nil {
		t.Fatal(err)
	}
}

// TestCrashActiveRawPut_MainnetFieldBlock10006 verifies 213 B field blocks survive raw put crash recovery.
func TestCrashActiveRawPut_MainnetFieldBlock10006(t *testing.T) {
	var spec mainnetCanonicalBlockSpec
	for _, s := range mainnetCanonicalBlockSpecs {
		if s.Height == 10006 {
			spec = s
			break
		}
	}
	if spec.Height != 10006 {
		t.Fatal("missing height 10006 spec")
	}
	blockRaw, err := buildMainnetCanonicalBlockRaw(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(blockRaw) != 213 {
		t.Fatalf("block len=%d want 213", len(blockRaw))
	}
	id := pow.BlockHashLE(blockRaw[:80])

	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := filepath.Join(raw.Dir(), hex.EncodeToString(id[:])+".bin.tmp")
	if err := os.WriteFile(tmpPath, blockRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if raw.Has(id) {
		t.Fatal("complete .tmp must not count as stored block")
	}
	n, err := raw.PurgeStaleRawBlockTemps()
	if err != nil || n != 1 {
		t.Fatalf("purge tmp: n=%d err=%v", n, err)
	}
	if err := raw.Put(id, blockRaw); err != nil {
		t.Fatal(err)
	}
	got, err := raw.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 213 {
		t.Fatalf("stored len=%d want 213", len(got))
	}
	if pow.BlockHashHex(got[:80]) != spec.WantHash {
		t.Fatalf("hash %s want %s", pow.BlockHashHex(got[:80]), spec.WantHash)
	}
}
