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

	"dogego/bloom"
	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func TestHandleInboundGetData_FilteredBlockServesMerkle(t *testing.T) {
	dir := t.TempDir()
	raw, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	blockRaw, hash := store.TestMinimalBlock()
	pb, err := wire.ParseBlock(blockRaw)
	if err != nil {
		t.Fatal(err)
	}
	if len(pb.Txs) == 0 {
		t.Fatal("empty fixture")
	}
	// Match coinbase txid so merkleblock includes it.
	f, err := bloom.NewEmpty(8, 0.00001, 0, bloom.UpdateNone)
	if err != nil {
		t.Fatal(err)
	}
	txid := pb.Txs[0].TxHash()
	f.Insert(txid[:])

	if err := raw.Put(hash, blockRaw); err != nil {
		t.Fatal(err)
	}
	if pow.BlockHashLE(blockRaw[:80]) != hash {
		t.Fatal("hash mismatch")
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
		if err != nil || cmd != "merkleblock" {
			t.Errorf("first cmd=%q err=%v", cmd, err)
			return
		}
		_, pmt, err := wire.ParseMerkleBlockProof(pl)
		if err != nil {
			t.Errorf("parse merkle: %v", err)
			return
		}
		root, matches, _, ok := pmt.ExtractMatches()
		if !ok {
			t.Error("extract failed")
			return
		}
		var hdrMerkle [32]byte
		copy(hdrMerkle[:], blockRaw[36:68])
		if root != hdrMerkle {
			t.Errorf("merkle root mismatch")
		}
		if len(matches) != 1 || matches[0] != txid {
			t.Errorf("matches=%v want %v", matches, txid)
		}
		cmd2, _, err := wire.ReadMessage(cCli, p.Magic)
		if err != nil || cmd2 != "tx" {
			t.Errorf("second cmd=%q err=%v", cmd2, err)
		}
	}()
	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: wire.InvTypeFilteredBlock, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleInboundGetData(context.Background(), mw, GetDataServeEnv{Raw: raw, Bloom: f}, pl); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestHandleFilterLoadAndClear(t *testing.T) {
	pm := NewPeerMgr(P2PModeSettings{}, chain.Params{}, "/t/", net.Dialer{})
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	mw := NewMsgWriter(c1, [4]byte{0xc0, 0xc0, 0xc0, 0xc0})
	pm.RegisterPrimary("peer:1", c1, mw, nil, &wire.DecodedVersion{RelayTxes: true})

	f, _ := bloom.NewEmpty(4, 0.001, 1, bloom.UpdateAll)
	f.Insert([]byte("x"))
	pl, err := f.EncodeWire()
	if err != nil {
		t.Fatal(err)
	}
	if err := HandleFilterLoad(pm, "peer:1", pl, nil); err != nil {
		t.Fatal(err)
	}
	got := pm.PeerBloom("peer:1")
	if got == nil || !got.Contains([]byte("x")) {
		t.Fatal("bloom not installed")
	}
	HandleFilterClear(pm, "peer:1")
	if pm.PeerBloom("peer:1") != nil {
		t.Fatal("bloom not cleared")
	}
}
