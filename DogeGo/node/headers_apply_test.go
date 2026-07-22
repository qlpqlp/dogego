// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

const (
	testHardBits = uint32(0x1e0fffff)
	testEasyBits = uint32(0x207fffff)
)

func setHeaderBits(h []byte, bits uint32) {
	binary.LittleEndian.PutUint32(h[72:76], bits)
}

func TestPrepareHeadersForConnect_reorgTruncate(t *testing.T) {
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
	genHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x11
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	h1b := append([]byte(nil), h1...)
	h1b[76] ^= 0x22
	if err := prepareHeadersForConnect(j, nil, []wire.DecodedHeader{{Header80: h1b}}, nil); err != nil {
		t.Fatal(err)
	}
	tip, err := j.TipHeight()
	if err != nil || tip != 0 {
		t.Fatalf("after reorg prep tip=%d err=%v", tip, err)
	}
}

func TestHeaderChainWriteReorgTruncateNoDeadlock(t *testing.T) {
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
	genHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x11
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	h1b := append([]byte(nil), h1...)
	h1b[76] ^= 0x22
	done := make(chan error, 1)
	go func() {
		done <- withHeaderChainWriteErr(func() error {
			return prepareHeadersForConnectImpl(j, nil, []wire.DecodedHeader{{Header80: h1b}}, nil)
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("prepareHeadersForConnect under write lock: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reorg truncate deadlocked while header write lock held")
	}
	tip, err := j.TipHeight()
	if err != nil || tip != 0 {
		t.Fatalf("after reorg under lock tip=%d err=%v want 0", tip, err)
	}
}

func TestPrepareHeadersForConnect_rejectsLowWorkFork(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	setHeaderBits(g80[:], testHardBits)
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	genHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x11
	setHeaderBits(h1, testHardBits)
	h1Hash := pow.BlockHashLE(h1)
	h2 := append([]byte(nil), h1...)
	copy(h2[4:36], h1Hash[:])
	h2[76] ^= 0x22
	setHeaderBits(h2, testHardBits)
	if err := j.AppendHeaders([][]byte{h1, h2}); err != nil {
		t.Fatal(err)
	}
	low := append([]byte(nil), g80[:]...)
	copy(low[4:36], genHash[:])
	low[76] ^= 0x33
	setHeaderBits(low, testEasyBits)
	err = prepareHeadersForConnect(j, nil, []wire.DecodedHeader{{Header80: low}}, nil)
	if err == nil || !strings.Contains(err.Error(), "insufficient chain work") {
		t.Fatalf("want insufficient chain work error, got %v", err)
	}
	tip, _ := j.TipHeight()
	if tip != 2 {
		t.Fatalf("journal tip should stay 2, got %d", tip)
	}
}

func TestPrepareHeadersForConnect_chainElectionReject(t *testing.T) {
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
	genHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x11
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	h1b := append([]byte(nil), h1...)
	h1b[76] ^= 0x22
	bs := &BlockStoreCtx{
		chainElection: func(_ context.Context, forkAt int64, _ [32]byte, _ []wire.DecodedHeader, _ *big.Int) error {
			if forkAt != 0 {
				t.Fatalf("forkAt %d", forkAt)
			}
			return fmt.Errorf("headers: fork rejected (election test)")
		},
	}
	err = prepareHeadersForConnect(j, nil, []wire.DecodedHeader{{Header80: h1b}}, bs)
	if err == nil || !strings.Contains(err.Error(), "fork rejected") {
		t.Fatalf("got %v", err)
	}
	tip, _ := j.TipHeight()
	if tip != 1 {
		t.Fatalf("tip %d want 1", tip)
	}
}

func TestPrepareHeadersForConnect_preciousOverridesLowWork(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	setHeaderBits(g80[:], testHardBits)
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	genHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x11
	setHeaderBits(h1, testHardBits)
	h1Hash := pow.BlockHashLE(h1)
	h2 := append([]byte(nil), h1...)
	copy(h2[4:36], h1Hash[:])
	h2[76] ^= 0x22
	setHeaderBits(h2, testHardBits)
	if err := j.AppendHeaders([][]byte{h1, h2}); err != nil {
		t.Fatal(err)
	}
	low := append([]byte(nil), g80[:]...)
	copy(low[4:36], genHash[:])
	low[76] ^= 0x33
	setHeaderBits(low, testEasyBits)
	pol, err := store.LoadChainPolicy(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := pol.SetPrecious(pow.BlockHashHex(low)); err != nil {
		t.Fatal(err)
	}
	bs := &BlockStoreCtx{Policy: pol}
	if err := prepareHeadersForConnect(j, nil, []wire.DecodedHeader{{Header80: low}}, bs); err != nil {
		t.Fatalf("precious fork: %v", err)
	}
	tip, _ := j.TipHeight()
	if tip != 0 {
		t.Fatalf("tip %d want 0 after precious reorg prep", tip)
	}
}

func appendChainedHeaders(t *testing.T, j *store.HeaderJournal, g80 []byte, n int, bits uint32) {
	t.Helper()
	prevHash := pow.BlockHashLE(g80[:])
	for i := 0; i < n; i++ {
		h := append([]byte(nil), g80[:]...)
		copy(h[4:36], prevHash[:])
		h[76] ^= byte(i + 1)
		setHeaderBits(h, bits)
		if err := j.AppendHeaders([][]byte{h}); err != nil {
			t.Fatal(err)
		}
		prevHash = pow.BlockHashLE(h)
	}
}

func buildAltForkHeaders(g80 []byte, n int, bits uint32) [][]byte {
	out := make([][]byte, 0, n)
	prevHash := pow.BlockHashLE(g80[:])
	for i := 0; i < n; i++ {
		h := append([]byte(nil), g80[:]...)
		copy(h[4:36], prevHash[:])
		h[76] ^= byte(0xa0 + i)
		setHeaderBits(h, bits)
		out = append(out, h)
		prevHash = pow.BlockHashLE(h)
	}
	return out
}

func TestPrepareHeadersForConnect_marginalReorgDeferred(t *testing.T) {
	dir := t.TempDir()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	setHeaderBits(g80[:], testHardBits)
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	appendChainedHeaders(t, j, g80[:], 100, testHardBits)
	alt := buildAltForkHeaders(g80[:], 101, testHardBits)
	decoded := make([]wire.DecodedHeader, len(alt))
	for i, h := range alt {
		decoded[i] = wire.DecodedHeader{Header80: h}
	}
	err = prepareHeadersForConnect(j, nil, decoded, nil)
	if err == nil || !strings.Contains(err.Error(), "fork deferred") {
		t.Fatalf("got %v", err)
	}
	tip, _ := j.TipHeight()
	if tip != 100 {
		t.Fatalf("tip %d want 100 unchanged", tip)
	}
}

func TestPrepareHeadersForConnect_forkProbeCalled(t *testing.T) {
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
	genHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x11
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	h1b := append([]byte(nil), h1...)
	h1b[76] ^= 0x22
	var probed bool
	bs := &BlockStoreCtx{}
	bs.SetForkProbe(func(forkAt int64, _ [32]byte) {
		if forkAt != 0 {
			t.Fatalf("forkAt %d", forkAt)
		}
		probed = true
	})
	if err := prepareHeadersForConnect(j, nil, []wire.DecodedHeader{{Header80: h1b}}, bs); err != nil {
		t.Fatal(err)
	}
	if !probed {
		t.Fatal("forkProbe not called")
	}
}

func TestApplyHeadersMessage_rejectsInvalidMarkedBlock(t *testing.T) {
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
	genHash := pow.BlockHashLE(g80[:])
	h1 := append([]byte(nil), g80[:]...)
	copy(h1[4:36], genHash[:])
	h1[76] ^= 0x11
	if err := j.AppendHeaders([][]byte{h1}); err != nil {
		t.Fatal(err)
	}
	h1Hash := pow.BlockHashLE(h1)
	h2 := append([]byte(nil), g80[:]...)
	copy(h2[4:36], h1Hash[:])
	h2[76] ^= 0x22
	pol, err := store.LoadChainPolicy(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := pol.AddInvalid(pow.BlockHashHex(h2)); err != nil {
		t.Fatal(err)
	}
	bs := &BlockStoreCtx{Policy: pol}
	pl, err := wire.EncodeHeadersPayload([]wire.DecodedHeader{{Header80: h2}})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ApplyHeadersMessage(j, nil, p, pl, time.Now().Unix(), bs)
	if err == nil || !strings.Contains(err.Error(), "marked invalid") {
		t.Fatalf("got %v", err)
	}
}
