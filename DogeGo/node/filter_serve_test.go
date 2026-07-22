// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/binary"
	"net"
	"path/filepath"
	"testing"

	"dogego/consensus"
	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
	"dogego/wire"
)

func TestHandleInboundGetCFHeadersServesMessage(t *testing.T) {
	rawBlk, id := store.TestMinimalBlock()
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), rawBlk[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.OpenRawBlockStore(dir)
	ix, _ := store.OpenTxIndex(dir)
	fx, _ := store.OpenBlockFilterIndex(dir)
	_ = raw.Put(id, rawBlk)
	_ = ix.IndexBlock(id, rawBlk)
	if err := rpc.IndexBasicBlockFilter(fx, id, rawBlk, j, raw, ix); err != nil {
		t.Fatal(err)
	}
	var req wire.FilterRangeRequest
	req.FilterType = wire.FilterTypeBasic
	req.StartHeight = 0
	req.StopHashLE = id
	pl := make([]byte, 37)
	pl[0] = req.FilterType
	binary.LittleEndian.PutUint32(pl[1:5], req.StartHeight)
	copy(pl[5:], req.StopHashLE[:])

	var magic [4]byte
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, magic)
	done := make(chan error, 1)
	go func() {
		done <- HandleInboundGetCFHeaders(mw, j, raw, ix, fx, pl)
	}()
	cmd, body, err := wire.ReadMessage(cCli, magic)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if cmd != "cfheaders" {
		t.Fatalf("cmd %q", cmd)
	}
	dec, err := wire.DecodeCFHeadersPayload(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec.FilterHashes) != 1 {
		t.Fatalf("hashes %d", len(dec.FilterHashes))
	}
	enc, _, _ := fx.Get(id)
	want := consensus.BlockFilterHash(enc)
	if dec.FilterHashes[0] != want {
		t.Fatalf("hash mismatch")
	}
}

func TestResolveFilterRangeUnknownStopHash(t *testing.T) {
	dir := t.TempDir()
	rawBlk, _ := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), rawBlk[:80])
	if err != nil {
		t.Fatal(err)
	}
	var stop [32]byte
	stop[0] = 0xab
	req := wire.FilterRangeRequest{FilterType: wire.FilterTypeBasic, StartHeight: 0, StopHashLE: stop}
	_, _, err = resolveFilterRange(j, req, maxGetCFiltersRange)
	if err == nil || err.Error() != "unknown stop hash" {
		t.Fatalf("got %v", err)
	}
}

func TestResolveFilterRangeTooLarge(t *testing.T) {
	dir := t.TempDir()
	rawBlk, _ := store.TestMinimalBlock()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), rawBlk[:80])
	if err != nil {
		t.Fatal(err)
	}
	h2 := append([]byte(nil), rawBlk[:80]...)
	h2[76] ^= 0x22
	if err := j.AppendHeaders([][]byte{h2}); err != nil {
		t.Fatal(err)
	}
	stopLE := pow.BlockHashLE(h2)
	req := wire.FilterRangeRequest{FilterType: wire.FilterTypeBasic, StartHeight: 0, StopHashLE: stopLE}
	_, _, err = resolveFilterRange(j, req, 1)
	if err == nil || err.Error() != "filter range too large" {
		t.Fatalf("got %v", err)
	}
}

func TestHandleInboundGetCFiltersServesMessage(t *testing.T) {
	rawBlk, id := store.TestMinimalBlock()
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), rawBlk[:80])
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := store.OpenRawBlockStore(dir)
	ix, _ := store.OpenTxIndex(dir)
	fx, _ := store.OpenBlockFilterIndex(dir)
	_ = raw.Put(id, rawBlk)
	_ = ix.IndexBlock(id, rawBlk)
	pl := make([]byte, 37)
	pl[0] = wire.FilterTypeBasic
	binary.LittleEndian.PutUint32(pl[1:5], 0)
	copy(pl[5:], id[:])

	var magic [4]byte
	cSrv, cCli := net.Pipe()
	mw := NewMsgWriter(cSrv, magic)
	done := make(chan error, 1)
	go func() {
		done <- HandleInboundGetCFilters(mw, j, raw, ix, fx, pl)
	}()
	cmd, body, err := wire.ReadMessage(cCli, magic)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if cmd != "cfilter" {
		t.Fatalf("cmd %q", cmd)
	}
	if len(body) < 34 {
		t.Fatalf("body len %d", len(body))
	}
	var gotHash [32]byte
	copy(gotHash[:], body[:32])
	if gotHash != id {
		t.Fatalf("block hash mismatch")
	}
}

func TestHandleInboundGetCFiltersMalformedPayload(t *testing.T) {
	rawBlk, _ := store.TestMinimalBlock()
	dir := t.TempDir()
	j, _ := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), rawBlk[:80])
	raw, _ := store.OpenRawBlockStore(dir)
	ix, _ := store.OpenTxIndex(dir)
	fx, _ := store.OpenBlockFilterIndex(dir)
	var magic [4]byte
	mw := NewMsgWriter(nil, magic)
	if err := HandleInboundGetCFilters(mw, j, raw, ix, fx, []byte{0xff}); err == nil {
		t.Fatal("expected decode error")
	}
}
