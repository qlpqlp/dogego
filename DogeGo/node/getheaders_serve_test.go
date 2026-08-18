// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func TestHandleInboundGetHeaders(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h2 := append([]byte(nil), g80[:]...)
	h2[76] ^= 2
	if err := j.AppendHeaders([][]byte{h2}); err != nil {
		t.Fatal(err)
	}
	genLE := pow.BlockHashLE(g80[:])
	loc := [][32]byte{genLE}
	pl, err := wire.EncodeGetHeaders(p.ProtocolVersion, loc, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}

	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, body, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			t.Error(err)
			return
		}
		if cmd != "headers" {
			t.Errorf("cmd %q", cmd)
			return
		}
		got, err := wire.DecodeHeadersPayload(body)
		if err != nil || len(got) != 1 {
			t.Fatalf("decode: %d err %v", len(got), err)
		}
	}()

	if err := HandleInboundGetHeaders(context.Background(), mw, GetHeadersServeEnv{Journal: j}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetHeadersHashStop(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	prevID := pow.BlockHashLE(g80[:])
	for i := byte(1); i <= 2; i++ {
		h := append([]byte(nil), g80[:]...)
		copy(h[4:36], prevID[:])
		h[76] ^= i
		if err := j.AppendHeaders([][]byte{h}); err != nil {
			t.Fatal(err)
		}
		prevID = pow.BlockHashLE(h)
	}
	h2, _ := j.ReadHeaderAt(2)
	stop := pow.BlockHashLE(h2)
	genLE := pow.BlockHashLE(g80[:])
	pl, err := wire.EncodeGetHeaders(p.ProtocolVersion, [][32]byte{genLE}, stop)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, body, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "headers" {
			t.Errorf("cmd %q err %v", cmd, err)
			return
		}
		got, err := wire.DecodeHeadersPayload(body)
		if err != nil || len(got) != 1 {
			t.Errorf("decode len=%d err=%v", len(got), err)
		}
	}()
	if err := HandleInboundGetHeaders(context.Background(), mw, GetHeadersServeEnv{Journal: j}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetHeadersEmptyAtTip(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	tipLE := pow.BlockHashLE(g80[:])
	pl, err := wire.EncodeGetHeaders(p.ProtocolVersion, [][32]byte{tipLE}, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, body, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "headers" {
			t.Errorf("cmd=%q err=%v", cmd, err)
			return
		}
		got, err := wire.DecodeHeadersPayload(body)
		if err != nil || len(got) != 0 {
			t.Errorf("len=%d err=%v", len(got), err)
		}
	}()
	if err := HandleInboundGetHeaders(context.Background(), mw, GetHeadersServeEnv{Journal: j}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetHeadersMalformedPayload(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, _ := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	dir := t.TempDir()
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetHeaders(context.Background(), mw, GetHeadersServeEnv{Journal: j}, []byte{0xff}); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestHandleInboundGetHeadersUnknownLocator(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h1 := append([]byte(nil), g80[:]...)
	genHash := pow.BlockHashLE(g80[:])
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x11
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	var unknown [32]byte
	unknown[0] = 0x99
	pl, err := wire.EncodeGetHeaders(p.ProtocolVersion, [][32]byte{unknown}, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, body, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "headers" {
			t.Errorf("cmd=%q err=%v", cmd, err)
			return
		}
		got, err := wire.DecodeHeadersPayload(body)
		if err != nil || len(got) != 1 {
			t.Errorf("len=%d err=%v", len(got), err)
		}
	}()
	if err := HandleInboundGetHeaders(context.Background(), mw, GetHeadersServeEnv{Journal: j}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetHeadersMaxCap(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, g80[:], 2100)
	genLE := pow.BlockHashLE(g80[:])
	pl, err := wire.EncodeGetHeaders(p.ProtocolVersion, [][32]byte{genLE}, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(10 * time.Second))
		cmd, body, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "headers" {
			t.Errorf("cmd=%q err=%v", cmd, err)
			return
		}
		got, err := wire.DecodeHeadersPayload(body)
		if err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		if len(got) != store.MaxHeadersPerMessage {
			t.Errorf("len=%d want cap %d", len(got), store.MaxHeadersPerMessage)
		}
	}()
	if err := HandleInboundGetHeaders(context.Background(), mw, GetHeadersServeEnv{Journal: j}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetHeadersDefersDuringBodyIBD(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderChain(dir, g80[:])
	if err != nil {
		t.Fatal(err)
	}
	appendFakeHeaderChain(t, j, g80[:], 534_000)
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	bs := NewBlockStoreCtx(j, nil, p, rs, nil, nil)
	bs.SeedContiguousTip(10_005)
	if !ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		t.Fatal("fixture should pause getheaders during deep body IBD")
	}
	pl, err := wire.EncodeGetHeaders(p.ProtocolVersion, [][32]byte{pow.BlockHashLE(g80[:])}, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, body, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "headers" {
			t.Errorf("cmd %q err %v", cmd, err)
			return
		}
		got, err := wire.DecodeHeadersPayload(body)
		if err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		if len(got) != 0 {
			t.Errorf("served %d header(s) during body IBD; want empty (keep TCP free for getdata)", len(got))
		}
	}()
	if err := HandleInboundGetHeaders(context.Background(), mw, GetHeadersServeEnv{Journal: j, BlockStore: bs}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}
