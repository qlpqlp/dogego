// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
)

// TestStartupRecoveryConvergence stitches crash-style corruption with autoRecoverSweep:
// torn headers tail repair on reopen, corrupt index/filter artifacts, stale raw tmp - no manual steps.
func TestStartupRecoveryConvergence(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	chainDir := t.TempDir()
	j, err := store.OpenHeaderChain(chainDir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	for h := int64(1); h <= 3; h++ {
		if _, err := rpc.AppendRebootTestnetMinedFixture(j, int(h)); err != nil {
			t.Fatal(err)
		}
	}
	tipBefore, err := j.TipHeight()
	if err != nil || tipBefore != 3 {
		t.Fatalf("tip before crash=%d err=%v", tipBefore, err)
	}

	f, err := os.OpenFile(filepath.Join(chainDir, "headers", "seg", "0000000000.bin"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, 19)); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	j, err = store.OpenHeaderChain(chainDir, gen[:])
	if err != nil {
		t.Fatal(err)
	}
	tipAfterOpen, _ := j.TipHeight()
	if tipAfterOpen != tipBefore {
		t.Fatalf("tail repair: tip=%d want %d", tipAfterOpen, tipBefore)
	}

	raw, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	txIx, err := store.OpenTxIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	filterIx, err := store.OpenBlockFilterIndex(chainDir)
	if err != nil {
		t.Fatal(err)
	}
	genesisRaw := store.MakeTestBlockRaw(t, gen[:])
	genID := pow.BlockHashLE(genesisRaw[:80])
	tmpPath := filepath.Join(raw.Dir(), hex.EncodeToString(genID[:])+".bin.tmp")
	if err := os.WriteFile(tmpPath, genesisRaw[:120], 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(txIx.RootDir(), "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"), []byte{0x01}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filterIx.Dir(), "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd.dat.tmp"), []byte{0x02}, 0o600); err != nil {
		t.Fatal(err)
	}

	auxPath := filepath.Join(chainDir, "headers_aux.bin")
	aux, err := store.OpenHeaderAuxJournal(auxPath, tipBefore+1)
	if err != nil {
		t.Fatal(err)
	}
	if err := aux.EnsureRecordCount(tipBefore + 1); err != nil {
		t.Fatal(err)
	}
	af, err := os.OpenFile(auxPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := af.Write([]byte{0xff, 0xff, 0xff}); err != nil {
		_ = af.Close()
		t.Fatal(err)
	}
	_ = af.Close()

	bs := NewBlockStoreCtx(j, nil, p, raw, txIx, nil)
	repair := autoRecoverFilterRepairFn(j, chainDir, filterIx, txIx, raw)
	rewound, sweepErr := autoRecoverSweep(chainDir, j, nil, p, bs, repair)
	if sweepErr != nil {
		t.Fatalf("convergence sweep: %v", sweepErr)
	}
	if rewound {
		t.Fatal("did not expect header rewind on reboot-testnet convergence fixture")
	}
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal("expected raw .bin.tmp purged")
	}
	if _, err := os.Stat(filepath.Join(filterIx.Dir(), "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd.dat.tmp")); !os.IsNotExist(err) {
		t.Fatal("expected filter .dat.tmp purged")
	}
	tipFinal, _ := j.TipHeight()
	if tipFinal != tipBefore {
		t.Fatalf("tip after sweep=%d want %d", tipFinal, tipBefore)
	}
	aux2, err := store.OpenHeaderAuxJournal(auxPath, tipBefore+1)
	if err != nil {
		t.Fatalf("headers_aux after convergence: %v", err)
	}
	if aux2.RecordCount() != tipBefore+1 {
		t.Fatalf("aux count=%d want %d", aux2.RecordCount(), tipBefore+1)
	}
}
