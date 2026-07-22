//go:build !short

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
)

func TestMain(m *testing.M) {
	if os.Getenv("DOGEGO_RAW_PUT_KILLEE") == "1" {
		os.Exit(rawPutKilleeMain())
	}
	if os.Getenv("DOGEGO_HEADER_SEG_KILLEE") == "1" {
		os.Exit(headerSegKilleeMain())
	}
	if os.Getenv("DOGEGO_FILTER_KILLEE") == "1" {
		os.Exit(blockFilterKilleeMain())
	}
	if os.Getenv("DOGEGO_TX_INDEX_KILLEE") == "1" {
		os.Exit(txIndexKilleeMain())
	}
	os.Exit(m.Run())
}

func rawPutKilleeMain() int {
	dir := os.Getenv("DOGEGO_RAW_PUT_DIR")
	if dir == "" {
		return 2
	}
	stallAfterRawPutTmpWrite = 24 * time.Hour
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		return 3
	}
	genesisRaw, id := TestMinimalBlock()
	if err := raw.Put(id, genesisRaw); err != nil {
		return 4
	}
	return 0
}

func headerSegKilleeMain() int {
	dir := os.Getenv("DOGEGO_HEADER_SEG_DIR")
	if dir == "" {
		return 2
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		return 3
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		return 4
	}
	j, err := OpenHeaderChain(dir, gen[:])
	if err != nil {
		return 5
	}
	stallAfterHeaderSegTmpWrite = 24 * time.Hour
	h80 := append([]byte(nil), gen[:]...)
	h80[76] ^= 1
	if err := j.AppendHeaders([][]byte{h80}); err != nil {
		return 6
	}
	_ = j
	return 0
}

func blockFilterKilleeMain() int {
	dir := os.Getenv("DOGEGO_FILTER_KILLEE_DIR")
	if dir == "" {
		return 2
	}
	fx, err := OpenBlockFilterIndex(dir)
	if err != nil {
		return 3
	}
	stallAfterBlockFilterPutTmpWrite = 24 * time.Hour
	var hash [32]byte
	hash[0] = 1
	hdr := make([]byte, 32)
	if err := fx.Put(hash, []byte{0x01, 0x02, 0x03}, hdr); err != nil {
		return 4
	}
	return 0
}

func txIndexKilleeMain() int {
	dir := os.Getenv("DOGEGO_TX_INDEX_KILLEE_DIR")
	if dir == "" {
		return 2
	}
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		return 3
	}
	txIx, err := OpenTxIndex(dir)
	if err != nil {
		return 4
	}
	raw.EnableTxIndexing(txIx, true)
	stallAfterTxIndexTmpWrite = 24 * time.Hour
	genesisRaw, id := TestMinimalBlock()
	if err := raw.Put(id, genesisRaw); err != nil {
		return 5
	}
	return 0
}

// TestSubprocessKillDuringRawPut kills a child mid-Put (after .tmp write) and verifies recovery without manual steps.
func TestSubprocessKillDuringRawPut(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Process.Kill is supported; run on all platforms including Windows.
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^$", "-test.timeout=30s")
	cmd.Env = append(os.Environ(),
		"DOGEGO_RAW_PUT_KILLEE=1",
		"DOGEGO_RAW_PUT_DIR="+dir,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	genesisRaw, id := TestMinimalBlock()
	tmpPath := filepath.Join(dir, "rawblocks", hex.EncodeToString(id[:])+".bin.tmp")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(tmpPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(tmpPath); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("child did not create .tmp before kill window")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()

	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if raw.Has(id) {
		t.Fatal("must not have committed .bin after kill")
	}
	n, err := raw.PurgeStaleRawBlockTemps()
	if err != nil || n != 1 {
		t.Fatalf("purge tmp: n=%d err=%v", n, err)
	}
	if err := raw.Put(id, genesisRaw); err != nil {
		t.Fatal(err)
	}
	if !raw.Has(id) {
		t.Fatal("expected block after recovery Put")
	}
}

// TestSubprocessKillDuringHeaderSegmentAppend kills a child mid-segment append and verifies OpenHeaderChain recovery.
func TestSubprocessKillDuringHeaderSegmentAppend(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^$", "-test.timeout=30s")
	cmd.Env = append(os.Environ(),
		"DOGEGO_HEADER_SEG_KILLEE=1",
		"DOGEGO_HEADER_SEG_DIR="+dir,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	tmpPath := filepath.Join(dir, "headers", "seg", "0000000000.bin.tmp")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(tmpPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(tmpPath); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("child did not create segment .tmp before kill window")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()

	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := OpenHeaderChain(dir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal("expected segment .tmp purged on reopen")
	}
	tip, err := j.TipHeight()
	if err != nil || tip != 0 {
		t.Fatalf("tip after kill recovery=%d err=%v want 0", tip, err)
	}
	h80 := append([]byte(nil), gen[:]...)
	h80[76] ^= 1
	if err := j.AppendHeaders([][]byte{h80}); err != nil {
		t.Fatal(err)
	}
	tip, _ = j.TipHeight()
	if tip != 1 {
		t.Fatalf("tip after re-append=%d want 1", tip)
	}
}

func waitForGlob(t *testing.T, pattern string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no match for " + pattern + " before kill window")
}

func killSubprocess(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
}

// TestSubprocessKillDuringBlockFilterPut kills a child mid-filter Put and verifies purge + retry.
func TestSubprocessKillDuringBlockFilterPut(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^$", "-test.timeout=30s")
	cmd.Env = append(os.Environ(), "DOGEGO_FILTER_KILLEE=1", "DOGEGO_FILTER_KILLEE_DIR="+dir)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForGlob(t, filepath.Join(dir, "filters", "basic", "*.dat.tmp"))
	killSubprocess(t, cmd)

	fx, err := OpenBlockFilterIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	var hash [32]byte
	hash[0] = 1
	if fx.Has(hash) {
		t.Fatal("filter must not be committed after kill")
	}
	n, err := fx.PurgeStaleBlockFilterTemps()
	if err != nil || n != 1 {
		t.Fatalf("purge filter tmp: n=%d err=%v", n, err)
	}
	hdr := make([]byte, 32)
	if err := fx.Put(hash, []byte{0x01, 0x02, 0x03}, hdr); err != nil {
		t.Fatal(err)
	}
	if !fx.Has(hash) {
		t.Fatal("expected filter after recovery Put")
	}
}

// TestSubprocessKillDuringTxIndexWrite kills a child mid tx-index write and verifies purge + retry.
func TestSubprocessKillDuringTxIndexWrite(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^$", "-test.timeout=30s")
	cmd.Env = append(os.Environ(), "DOGEGO_TX_INDEX_KILLEE=1", "DOGEGO_TX_INDEX_KILLEE_DIR="+dir)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForGlob(t, filepath.Join(dir, "indexes", "tx", "*.tmp"))
	killSubprocess(t, cmd)

	genesisRaw, id := TestMinimalBlock()
	raw, err := OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !raw.Has(id) {
		t.Fatal("raw block should be committed before tx index stall point")
	}
	txIx, err := OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	n, err := txIx.PurgeStaleTxIndexTemps()
	if err != nil || n < 1 {
		t.Fatalf("purge tx index tmp: n=%d err=%v", n, err)
	}
	raw.EnableTxIndexing(txIx, true)
	if err := raw.Put(id, genesisRaw); err != nil {
		t.Fatal(err)
	}
	if c, _, err := txIx.Stats(); err != nil || c < 1 {
		t.Fatalf("tx index stats after recovery: count=%d err=%v", c, err)
	}
}
