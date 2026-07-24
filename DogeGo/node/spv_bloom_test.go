// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"dogego/chain"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestSPVBloomRequestFilteredBlocks(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	c := NewSPVBloomClient(w, p, nil)
	if !c.Active() {
		t.Fatal("expected active bloom from fresh wallet")
	}
	cSrv, cCli := net.Pipe()
	defer cSrv.Close()
	defer cCli.Close()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan string, 1)
	go func() {
		_ = cCli.SetReadDeadline(time.Now().Add(3 * time.Second))
		cmd, _, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			done <- err.Error()
			return
		}
		done <- cmd
	}()
	entries := []wire.InvEntry{
		{Type: wire.InvTypeBlock, Hash: [32]byte{1}},
		{Type: wire.InvTypeTx, Hash: [32]byte{2}},
	}
	if err := c.RequestFilteredBlocks(mw, entries); err != nil {
		t.Fatal(err)
	}
	cmd := <-done
	if cmd != "getdata" {
		t.Fatalf("got %q", cmd)
	}
}

func TestHandleMerkleBlockStandalone(t *testing.T) {
	blockRaw, _ := store.TestMinimalBlock()
	pb, err := wire.ParseBlock(blockRaw)
	if err != nil {
		t.Fatal(err)
	}
	hashes := make([][32]byte, len(pb.Txs))
	match := make([]bool, len(pb.Txs))
	for i, tx := range pb.Txs {
		hashes[i] = tx.TxHash()
		match[i] = true
	}
	pmt, err := wire.NewPartialMerkleTree(hashes, match)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := wire.SerializeMerkleBlock(blockRaw[:80], pmt)
	if err != nil {
		t.Fatal(err)
	}
	h2, m2, err := HandleMerkleBlockStandalone(pl)
	if err != nil {
		t.Fatal(err)
	}
	if len(h2) != 80 {
		t.Fatal("header len")
	}
	if len(m2) != len(pb.Txs) {
		t.Fatalf("matches %d want %d", len(m2), len(pb.Txs))
	}
}

func TestSPVBloomSendFilterLoadRequiresNODE_BLOOM(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "w.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	c := NewSPVBloomClient(w, p, nil)
	cSrv, cCli := net.Pipe()
	defer cSrv.Close()
	defer cCli.Close()
	mw := NewMsgWriter(cSrv, p.Magic)
	if err := c.SendFilterLoad(mw, 0); err != nil {
		t.Fatal(err)
	}
	_ = cCli.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	if _, _, err := wire.ReadMessage(cCli, p.Magic); err == nil {
		t.Fatal("should not send without NODE_BLOOM")
	}
	done := make(chan string, 1)
	go func() {
		_ = cCli.SetReadDeadline(time.Now().Add(3 * time.Second))
		cmd, _, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			done <- "err:" + err.Error()
			return
		}
		done <- cmd
	}()
	if err := c.SendFilterLoad(mw, chain.ServiceBloom); err != nil {
		t.Fatal(err)
	}
	if got := <-done; got != "filterload" {
		t.Fatalf("got %q", got)
	}
}
