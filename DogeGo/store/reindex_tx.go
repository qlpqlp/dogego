// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"
	"os"
	"path/filepath"

	"dogego/pow"
)

// ReindexTxReport summarizes a full rawblocks → indexes/tx rebuild.
type ReindexTxReport struct {
	BlocksIndexed int
	TxFiles       int
	AddrRecvFiles int
	AddrSpendFiles int
	OutSpendFiles int
	Skipped       int
}

// ReindexTxFromRawBlocks rebuilds indexes/tx and indexes/addr from every *.bin under rawblocks/.
// When clearFirst is true, existing tx and address index files are removed before indexing.
// Pass live txIx/addrIx when repairing during sync so writes share the indexes/tx dir lock.
func ReindexTxFromRawBlocks(chainDir string, clearFirst bool) (ReindexTxReport, error) {
	return ReindexTxFromRawBlocksWithIndex(chainDir, clearFirst, nil, nil)
}

// ReindexTxFromRawBlocksWithIndex rebuilds indexes using optional live index handles.
func ReindexTxFromRawBlocksWithIndex(chainDir string, clearFirst bool, txIx *TxIndex, addrIx *AddrIndex) (ReindexTxReport, error) {
	var rep ReindexTxReport
	var err error
	if txIx == nil {
		txIx, err = OpenTxIndex(chainDir)
		if err != nil {
			return rep, err
		}
	}
	if addrIx == nil {
		addrIx, err = OpenAddrIndex(chainDir)
		if err != nil {
			return rep, err
		}
	}
	if clearFirst {
		if err := ClearTxIndex(txIx); err != nil {
			return rep, err
		}
		if err := ClearAddrIndex(addrIx); err != nil {
			return rep, err
		}
	}
	raw, err := OpenRawBlockStore(chainDir)
	if err != nil {
		return rep, err
	}
	addrIx.SetResolver(txIx, raw)
	dir := raw.Dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return rep, err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".bin" {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".bin" || len(name) != 68 {
			rep.Skipped++
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return rep, fmt.Errorf("read %s: %w", name, err)
		}
		if len(b) < 80 {
			rep.Skipped++
			continue
		}
		id := pow.BlockHashLE(b[:80])
		if err := txIx.IndexBlock(id, b); err != nil {
			return rep, fmt.Errorf("index %s: %w", name[:16], err)
		}
		if err := addrIx.IndexBlock(id, b); err != nil {
			return rep, fmt.Errorf("addr index %s: %w", name[:16], err)
		}
		rep.BlocksIndexed++
	}
	n, _, err := txIx.Stats()
	if err != nil {
		return rep, err
	}
	rep.TxFiles = n
	rep.AddrRecvFiles, _ = countIndexFiles(addrIx.recvRoot)
	rep.AddrSpendFiles, _ = countIndexFiles(addrIx.spendRoot)
	rep.OutSpendFiles, _ = countIndexFiles(addrIx.outRoot)
	return rep, nil
}

func countIndexFiles(root string) (int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			n++
		}
	}
	return n, nil
}

// ClearTxIndex removes all txid files under indexes/tx.
func ClearTxIndex(x *TxIndex) error {
	if x == nil {
		return fmt.Errorf("nil tx index")
	}
	x.mu.Lock()
	root := x.root
	x.mu.Unlock()
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(root, e.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
