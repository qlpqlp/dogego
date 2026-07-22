// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"net"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// TestInboundServeBlackBox exercises getheaders + getdata on one synthetic peer (STANDALONE §3 inbound serving).
func TestInboundServeBlackBox(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderChain(dir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	h80 := make([]byte, 80)
	copy(h80, gen[:])
	binaryLETime(h80, 1_702_000_000)
	if err := j.AppendHeaders([][]byte{h80}); err != nil {
		t.Fatal(err)
	}
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantBlock, hash := store.TestMinimalBlock()
	if err := raw.Put(hash, wantBlock); err != nil {
		t.Fatal(err)
	}

	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "headers" {
			t.Errorf("getheaders reply cmd=%q err=%v", cmd, err)
			return
		}
		if len(pl) < 80 {
			t.Errorf("headers payload len %d", len(pl))
		}
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd2, pl2, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd2 != "block" || string(pl2) != string(wantBlock) {
			t.Errorf("getdata reply cmd=%q len=%d", cmd2, len(pl2))
		}
	}()

	genLE := pow.BlockHashLE(gen[:])
	ghPl, err := wire.EncodeGetHeaders(p.ProtocolVersion, [][32]byte{genLE}, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetHeaders(context.Background(), mw, GetHeadersServeEnv{Journal: j}, ghPl); err != nil {
		t.Fatal(err)
	}
	gdPl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeBlock, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Raw: raw}, gdPl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestInboundServeBlackBoxConfirmedTxAndNotFound(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndexWithOpts(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	payload, blockHash := store.TestMinimalBlock()
	if err := raw.Put(blockHash, payload); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(blockHash, payload); err != nil {
		t.Fatal(err)
	}
	var txHash [32]byte
	if err := wire.ForEachBlockTx(payload, func(_ uint32, tx *wire.Tx) error {
		h := tx.TxHash()
		copy(txHash[:], h[:])
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var missHash [32]byte
	missHash[0] = 0xde

	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "tx" || len(pl) == 0 {
			t.Errorf("tx reply cmd=%q len=%d err=%v", cmd, len(pl), err)
			return
		}
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd2, pl2, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd2 != "notfound" {
			t.Errorf("notfound cmd=%q err=%v", cmd2, err)
			return
		}
		missing, err := wire.DecodeInvPayload(pl2)
		if err != nil || len(missing) != 1 || missing[0].Hash != missHash {
			t.Errorf("notfound payload: %v err %v", missing, err)
		}
	}()

	gdPl, err := wire.EncodeGetData([]wire.InvEntry{
		{Type: wire.InvTypeTx, Hash: txHash},
		{Type: wire.InvTypeTx, Hash: missHash},
	})
	if err != nil {
		t.Fatal(err)
	}
	env := GetDataServeEnv{Raw: raw, TxIx: ix}
	if err := HandleInboundGetData(context.Background(), mw, env, gdPl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestInboundServeBlackBoxWitnessBlock(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantBlock, hash := store.TestMinimalBlock()
	if err := raw.Put(hash, wantBlock); err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "block" || string(pl) != string(wantBlock) {
			t.Errorf("witness getdata cmd=%q len=%d err=%v", cmd, len(pl), err)
		}
	}()
	gdPl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeWitnessBlock, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Raw: raw}, gdPl); err != nil {
		t.Fatal(err)
	}
	<-done
}
