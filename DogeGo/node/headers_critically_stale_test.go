// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/binary"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func mainnetGenesisHeader80(t *testing.T) []byte {
	t.Helper()
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	h80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	return h80[:]
}

func appendFakeHeaderChainAfterGenesis(t *testing.T, j *store.HeaderJournal, prev []byte, count int, nTime uint32) {
	t.Helper()
	h := append([]byte(nil), prev...)
	headers := make([][]byte, 0, count)
	for i := 0; i < count; i++ {
		h = append([]byte(nil), h...)
		binary.LittleEndian.PutUint64(h[36:44], uint64(i+1))
		binary.LittleEndian.PutUint32(h[68:72], nTime)
		cp := make([]byte, 80)
		copy(cp, h)
		headers = append(headers, cp)
	}
	if err := j.AppendHeaders(headers); err != nil {
		t.Fatal(err)
	}
}

func TestJournalFailsKnownCheckpoint_mainnetGenesisOnly(t *testing.T) {
	dir := t.TempDir()
	g := mainnetGenesisHeader80(t)
	j, err := store.OpenHeaderJournal(dir+"/h.bin", g)
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChainAfterGenesis(t, j, g, 9840, 1386967532)
	bad, msg := JournalFailsKnownCheckpoint(j, chain.MainnetDogecoin)
	if bad {
		t.Fatalf("mainnet genesis + synthetic extension should pass height-0 checkpoint: %q", msg)
	}
}

func TestJournalFailsKnownCheckpoint_wrongGenesisHash(t *testing.T) {
	dir := t.TempDir()
	g := mainnetGenesisHeader80(t)
	gBad := append([]byte(nil), g...)
	gBad[0] ^= 0xff
	j, err := store.OpenHeaderJournal(dir+"/h.bin", gBad)
	if err != nil {
		t.Fatal(err)
	}
	bad, msg := JournalFailsKnownCheckpoint(j, chain.MainnetDogecoin)
	if !bad || msg == "" {
		t.Fatalf("want checkpoint failure at genesis, got bad=%v msg=%q", bad, msg)
	}
}

func TestJournalFailsKnownCheckpoint_testnetIgnored(t *testing.T) {
	dir := t.TempDir()
	g := mainnetGenesisHeader80(t)
	j, err := store.OpenHeaderJournal(dir+"/h.bin", g)
	if err != nil {
		t.Fatal(err)
	}
	bad, _ := JournalFailsKnownCheckpoint(j, chain.RebootTestnet)
	if bad {
		t.Fatal("testnet uses separate checkpoint table")
	}
}
