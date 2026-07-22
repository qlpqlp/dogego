// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dogego/chain"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func TestHandleInboundGetData_BlockAndNotFound(t *testing.T) {
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantBlock, hash := store.TestMinimalBlock()
	if err := raw.Put(hash, wantBlock); err != nil {
		t.Fatal(err)
	}

	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	var gotCmd string
	var gotPl []byte
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			t.Error(err)
			return
		}
		gotCmd, gotPl = cmd, pl
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd2, pl2, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			t.Error(err)
			return
		}
		if cmd2 != "notfound" {
			t.Errorf("second msg cmd %q want notfound", cmd2)
			return
		}
		missing, err := wire.DecodeInvPayload(pl2)
		if err != nil || len(missing) != 1 || missing[0].Type != wire.InvTypeTx {
			t.Errorf("notfound payload: %v err %v", missing, err)
		}
	}()

	var missHash [32]byte
	missHash[1] = 0xcd
	pl, err := wire.EncodeGetData([]wire.InvEntry{
		{Type: wire.InvTypeBlock, Hash: hash},
		{Type: wire.InvTypeTx, Hash: missHash},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Raw: raw}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
	if gotCmd != "block" || string(gotPl) != string(wantBlock) {
		t.Fatalf("first reply cmd=%q payload=%q", gotCmd, gotPl)
	}
}

func TestHandleInboundGetData_TxFromMempool(t *testing.T) {
	pool := mempool.New(10)
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
	rawTx := buf.Bytes()
	if err := pool.Add(rawTx); err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(rawTx)
	if err != nil {
		t.Fatal(err)
	}
	var hash [32]byte
	th := tx.TxHash()
	copy(hash[:], th[:])

	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			t.Error(err)
			return
		}
		if cmd != "tx" || string(pl) != string(rawTx) {
			t.Errorf("got cmd=%q len=%d", cmd, len(pl))
		}
	}()

	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeTx, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Pool: pool}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetData_TxFromIndex(t *testing.T) {
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
	var hash [32]byte
	if err := wire.ForEachBlockTx(payload, func(_ uint32, tx *wire.Tx) error {
		h := tx.TxHash()
		copy(hash[:], h[:])
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil {
			t.Error(err)
			return
		}
		if cmd != "tx" || len(pl) == 0 {
			t.Errorf("got cmd=%q len=%d", cmd, len(pl))
		}
	}()
	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeTx, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	env := GetDataServeEnv{Raw: raw, TxIx: ix}
	if err := HandleInboundGetData(context.Background(), mw, env, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetDataBlockBatchCap(t *testing.T) {
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := store.TestMinimalBlock()
	var entries []wire.InvEntry
	for i := 0; i < 17; i++ {
		payload := append([]byte(nil), base...)
		payload[76] ^= byte(i + 1)
		id := pow.BlockHashLE(payload[:80])
		if err := raw.Put(id, payload); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, wire.InvEntry{Type: wire.InvTypeBlock, Hash: id})
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		blocks := 0
		var sawNotFound bool
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			_ = cCli.SetReadDeadline(time.Now().Add(time.Until(deadline)))
			cmd, _, err := wire.ReadMessage(cCli, p.Magic)
			if err != nil {
				t.Errorf("read: %v", err)
				return
			}
			switch cmd {
			case "block":
				blocks++
			case "notfound":
				sawNotFound = true
				if blocks != maxServeBlocksPerGetData {
					t.Errorf("blocks=%d before notfound want %d", blocks, maxServeBlocksPerGetData)
				}
				return
			}
		}
		if !sawNotFound {
			t.Errorf("blocks=%d no notfound", blocks)
		}
	}()
	pl, err := wire.EncodeGetData(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Raw: raw}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetData_TxBatchCap(t *testing.T) {
	pool := mempool.New(20)
	var entries []wire.InvEntry
	for i := 0; i < 9; i++ {
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
		_ = binary.Write(&buf, binary.LittleEndian, int64(8800000000+int64(i)))
		_ = wire.WriteCompactSize(&buf, 2)
		_, _ = buf.Write([]byte{0x51, 0x51})
		_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
		rawTx := buf.Bytes()
		if err := pool.Add(rawTx); err != nil {
			t.Fatal(err)
		}
		tx, err := wire.DeserializeTx(rawTx)
		if err != nil {
			t.Fatal(err)
		}
		h := tx.TxHash()
		var hash [32]byte
		copy(hash[:], h[:])
		entries = append(entries, wire.InvEntry{Type: wire.InvTypeTx, Hash: hash})
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		txs := 0
		var sawNotFound bool
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			_ = cCli.SetReadDeadline(time.Now().Add(time.Until(deadline)))
			cmd, _, err := wire.ReadMessage(cCli, p.Magic)
			if err != nil {
				t.Errorf("read: %v", err)
				return
			}
			switch cmd {
			case "tx":
				txs++
			case "notfound":
				sawNotFound = true
				if txs != maxServeTxsPerGetData {
					t.Errorf("txs=%d before notfound want %d", txs, maxServeTxsPerGetData)
				}
				return
			}
		}
		if !sawNotFound {
			t.Errorf("txs=%d no notfound", txs)
		}
	}()
	pl, err := wire.EncodeGetData(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Pool: pool}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetData_MixedBatchCap(t *testing.T) {
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := store.TestMinimalBlock()
	var entries []wire.InvEntry
	for i := 0; i < 17; i++ {
		payload := append([]byte(nil), base...)
		payload[76] ^= byte(i + 1)
		id := pow.BlockHashLE(payload[:80])
		if err := raw.Put(id, payload); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, wire.InvEntry{Type: wire.InvTypeBlock, Hash: id})
	}
	pool := mempool.New(20)
	for i := 0; i < 9; i++ {
		var buf bytes.Buffer
		getdataTestTxWrite(&buf, i)
		rawTx := buf.Bytes()
		if err := pool.Add(rawTx); err != nil {
			t.Fatal(err)
		}
		tx, err := wire.DeserializeTx(rawTx)
		if err != nil {
			t.Fatal(err)
		}
		h := tx.TxHash()
		var hash [32]byte
		copy(hash[:], h[:])
		entries = append(entries, wire.InvEntry{Type: wire.InvTypeTx, Hash: hash})
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		blocks, txs := 0, 0
		var sawNotFound bool
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			_ = cCli.SetReadDeadline(time.Now().Add(time.Until(deadline)))
			cmd, _, err := wire.ReadMessage(cCli, p.Magic)
			if err != nil {
				t.Errorf("read: %v", err)
				return
			}
			switch cmd {
			case "block":
				blocks++
			case "tx":
				txs++
			case "notfound":
				sawNotFound = true
				if blocks != maxServeBlocksPerGetData {
					t.Errorf("blocks=%d want %d", blocks, maxServeBlocksPerGetData)
				}
				if txs != maxServeTxsPerGetData {
					t.Errorf("txs=%d want %d", txs, maxServeTxsPerGetData)
				}
				return
			}
		}
		if !sawNotFound {
			t.Errorf("blocks=%d txs=%d no notfound", blocks, txs)
		}
	}()
	pl, err := wire.EncodeGetData(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Raw: raw, Pool: pool}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func getdataTestTxWrite(buf *bytes.Buffer, i int) {
	_ = binary.Write(buf, binary.LittleEndian, int32(1))
	_ = wire.WriteCompactSize(buf, 1)
	var zeros [32]byte
	_, _ = buf.Write(zeros[:])
	_ = binary.Write(buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(buf, 1)
	_, _ = buf.Write([]byte{0x00})
	_ = binary.Write(buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(buf, 1)
	_ = binary.Write(buf, binary.LittleEndian, int64(8800000000+int64(i)))
	_ = wire.WriteCompactSize(buf, 2)
	_, _ = buf.Write([]byte{0x51, 0x51})
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
}

func TestHandleInboundGetData_WitnessInvNotFound(t *testing.T) {
	pool := mempool.New(5)
	var buf bytes.Buffer
	getdataTestTxWrite(&buf, 0)
	rawTx := buf.Bytes()
	if err := pool.Add(rawTx); err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(rawTx)
	if err != nil {
		t.Fatal(err)
	}
	h := tx.TxHash()
	var hash [32]byte
	copy(hash[:], h[:])
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "notfound" {
			t.Errorf("cmd=%q err=%v", cmd, err)
			return
		}
		missing, err := wire.DecodeInvPayload(pl)
		if err != nil || len(missing) != 1 || missing[0].Type != wire.InvTypeWitnessTx {
			t.Errorf("missing=%v err=%v", missing, err)
		}
	}()
	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeWitnessTx, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Pool: pool}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetData_EmptyPayload(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, _, err := wire.ReadMessage(cCli, p.Magic)
		if err == nil {
			t.Error("expected no reply for empty getdata")
		}
	}()
	pl, err := wire.EncodeGetData(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetData_MalformedPayload(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, _ := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{}, []byte{0xff}); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestHandleInboundGetData_UnknownInvTypeNotFound(t *testing.T) {
	var hash [32]byte
	hash[0] = 0xab
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "notfound" {
			t.Errorf("cmd=%q err=%v", cmd, err)
			return
		}
		missing, err := wire.DecodeInvPayload(pl)
		if err != nil || len(missing) != 1 || missing[0].Type != wire.InvTypeCmpctBlock {
			t.Errorf("missing=%v err=%v", missing, err)
		}
	}()
	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeCmpctBlock, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetData_CmpctBlockFallsBackToFullBlockForAuxpow(t *testing.T) {
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	blockRaw := minimalAuxpowBlockNodeTest(t)
	hash := pow.BlockHashLE(blockRaw[:80])
	blockPath := filepath.Join(raw.Dir(), hex.EncodeToString(hash[:])+".bin")
	if err := os.WriteFile(blockPath, blockRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	before := cmpctMetrics.FallbackFullBlock.Load()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "block" || string(pl) != string(blockRaw) {
			t.Errorf("cmd=%q len=%d err=%v", cmd, len(pl), err)
		}
	}()
	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeCmpctBlock, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Raw: raw}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
	if cmpctMetrics.FallbackFullBlock.Load() != before+1 {
		t.Fatalf("FallbackFullBlock %d want %d", cmpctMetrics.FallbackFullBlock.Load(), before+1)
	}
}

func TestHandleInboundGetData_WitnessBlockServes(t *testing.T) {
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantBlock, hash := store.TestMinimalBlock()
	if err := raw.Put(hash, wantBlock); err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
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
			t.Errorf("cmd=%q len=%d err=%v", cmd, len(pl), err)
		}
	}()
	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeWitnessBlock, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Raw: raw}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetData_DuplicateBlockInv(t *testing.T) {
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantBlock, hash := store.TestMinimalBlock()
	if err := raw.Put(hash, wantBlock); err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		for i := 0; i < 2; i++ {
			cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
			if err != nil || cmd != "block" || string(pl) != string(wantBlock) {
				t.Errorf("reply %d cmd=%q len=%d err=%v", i, cmd, len(pl), err)
				return
			}
		}
	}()
	pl, err := wire.EncodeGetData([]wire.InvEntry{
		{Type: wire.InvTypeBlock, Hash: hash},
		{Type: wire.InvTypeBlock, Hash: hash},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Raw: raw}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleInboundGetData_FilteredBlockNotFound(t *testing.T) {
	var hash [32]byte
	hash[0] = 0xcd
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, p.Magic)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cCli.SetReadDeadline(time.Now().Add(5 * time.Second))
		cmd, pl, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd != "notfound" {
			t.Errorf("cmd=%q err=%v", cmd, err)
			return
		}
		missing, err := wire.DecodeInvPayload(pl)
		if err != nil || len(missing) != 1 || missing[0].Type != wire.InvTypeFilteredBlock {
			t.Errorf("missing=%v err=%v", missing, err)
		}
	}()
	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeFilteredBlock, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}
