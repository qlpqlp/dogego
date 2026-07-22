// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/wallet/corewallet"
)

func TestReplayCorePoolIntoHDKeypoolMatchedPubkey(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	d, err := w.deriveReceive(0)
	if err != nil {
		t.Fatal(err)
	}
	pubHex := hex.EncodeToString(d.Priv.PubKey().SerializeCompressed())
	before := w.KeypoolSize()

	res, err := w.ReplayCorePoolIntoHDKeypool([]corewallet.PoolEntry{
		{Index: 99, PubKeyHex: pubHex},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Index 0 is the default spend key: store Core index, do not re-queue into keypool.
	if res.IndicesReplayed || res.Reserved != 0 || res.Matched != 1 || res.CoreIndicesStored != 1 {
		t.Fatalf("res=%+v", res)
	}
	if w.hdKeypoolCoreIdx[0] != 99 {
		t.Fatalf("core index=%d", w.hdKeypoolCoreIdx[0])
	}
	if w.KeypoolSize() != before {
		t.Fatalf("keypool=%d want %d", w.KeypoolSize(), before)
	}
	if w.IsReceiveInKeypool(d.Addr) {
		t.Fatal("default receive must not be iskeypool")
	}
}

func TestReplayCorePoolPersistsCoreIndexInWalletJSON(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "wallet.json")
	w, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	d, err := w.deriveReceive(0)
	if err != nil {
		t.Fatal(err)
	}
	pubHex := hex.EncodeToString(d.Priv.PubKey().SerializeCompressed())
	if _, err := w.ReplayCorePoolIntoHDKeypool([]corewallet.PoolEntry{{Index: 42, PubKeyHex: pubHex}}); err != nil {
		t.Fatal(err)
	}
	w2, err := LoadOrCreate(path, p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if w2.hdKeypoolCoreIdx[0] != 42 {
		t.Fatalf("core index map=%v", w2.hdKeypoolCoreIdx)
	}
	entries := w2.HDKeypoolCoreIndexEntries()
	if len(entries) != 1 || entries[0].ReceiveIndex != 0 || entries[0].CoreIndex != 42 {
		t.Fatalf("entries=%+v", entries)
	}
	if w2.IsReceiveInKeypool(d.Addr) {
		t.Fatal("default receive must not be re-queued into keypool")
	}
	if core, ok := w2.CorePoolIndexForAddress(d.Addr); !ok || core != 42 {
		t.Fatalf("core index=%d ok=%v", core, ok)
	}
	listRows := w2.ListAddressEntries(p.PubkeyHashAddrID, p.ScriptHashAddrID)
	var found bool
	for _, e := range listRows {
		if e.Address != d.Addr {
			continue
		}
		found = true
		if e.IsKeypool {
			t.Fatal("default receive must not report iskeypool")
		}
		if e.HDKeypoolCoreIndex == nil || *e.HDKeypoolCoreIndex != 42 {
			t.Fatalf("list core index %#v", e.HDKeypoolCoreIndex)
		}
	}
	if !found {
		t.Fatal("receive address missing from list")
	}
}

func TestReplayCorePoolIntoHDKeypoolSkipsNonHD(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	w.hdSeed = nil
	w.mu.Unlock()
	res, err := w.ReplayCorePoolIntoHDKeypool([]corewallet.PoolEntry{{Index: 1, PubKeyHex: "02aa"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IndicesReplayed || res.Reserved != 0 {
		t.Fatalf("res=%+v", res)
	}
}

func TestReplayCorePoolDeepReceiveIndex(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	const deepIndex uint32 = 150
	d, err := w.deriveReceive(deepIndex)
	if err != nil {
		t.Fatal(err)
	}
	pubHex := hex.EncodeToString(d.Priv.PubKey().SerializeCompressed())
	w.mu.Lock()
	w.hdKeypool = nil
	w.mu.Unlock()

	res, err := w.ReplayCorePoolIntoHDKeypool([]corewallet.PoolEntry{
		{Index: 9001, PubKeyHex: pubHex},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IndicesReplayed || res.Matched != 1 || res.Reserved != 1 {
		t.Fatalf("res=%+v", res)
	}
	if w.hdKeypoolCoreIdx[deepIndex] != 9001 {
		t.Fatalf("core index mismatch")
	}
	coreIdx, ok := w.CorePoolIndexForAddress(d.Addr)
	if !ok || coreIdx != 9001 {
		t.Fatalf("CorePoolIndexForAddress=%d ok=%v", coreIdx, ok)
	}
}

func TestReplayCorePoolIntoHDKeypoolAlreadyReserved(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	d, err := w.deriveReceive(5)
	if err != nil {
		t.Fatal(err)
	}
	pubHex := hex.EncodeToString(d.Priv.PubKey().SerializeCompressed())
	res, err := w.ReplayCorePoolIntoHDKeypool([]corewallet.PoolEntry{{Index: 12, PubKeyHex: pubHex}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched != 1 || res.Reserved != 0 || res.IndicesReplayed {
		t.Fatalf("res=%+v", res)
	}
}

func TestReplayCorePoolSkipsIssuedReceiveIndex(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	idx, ok := func() (uint32, bool) {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.receiveIndexForAddressLocked(issued)
	}()
	if !ok {
		t.Fatal("issued address not found")
	}
	d, err := w.deriveReceive(idx)
	if err != nil {
		t.Fatal(err)
	}
	pubHex := hex.EncodeToString(d.Priv.PubKey().SerializeCompressed())
	before := w.KeypoolSize()
	res, err := w.ReplayCorePoolIntoHDKeypool([]corewallet.PoolEntry{{Index: 55, PubKeyHex: pubHex}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched != 1 || res.Reserved != 0 || res.IndicesReplayed || res.CoreIndicesStored != 1 {
		t.Fatalf("res=%+v", res)
	}
	if w.KeypoolSize() != before {
		t.Fatalf("keypool grew to %d", w.KeypoolSize())
	}
	if w.IsReceiveInKeypool(issued) {
		t.Fatal("issued address must not return to keypool")
	}
	if core, ok := w.CorePoolIndexForAddress(issued); !ok || core != 55 {
		t.Fatalf("core index=%d ok=%v", core, ok)
	}
}
