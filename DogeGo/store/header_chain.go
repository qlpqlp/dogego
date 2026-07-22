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
)

const headerMigrateMaxRecords = 2_000_000

// OpenHeaderChain opens header storage for chainDir: segment files (default), with one-time migration from headers.bin.
func OpenHeaderChain(chainDir string, genesis80 []byte) (*HeaderJournal, error) {
	if len(genesis80) != 80 {
		return nil, fmt.Errorf("genesis header must be 80 bytes")
	}
	monolith := filepath.Join(chainDir, "headers.bin")
	if _, err := os.Stat(headerManifestPath(chainDir)); err == nil {
		l, err := openHeaderSegmentLayout(chainDir)
		if err != nil {
			return nil, err
		}
		if n, _ := l.purgeStaleTemps(); n > 0 {
			fmt.Fprintf(os.Stderr, "DogeGo: removed %d incomplete header segment .tmp file(s)\n", n)
		}
		_ = l.repairFromCheckpoint()
		j := &HeaderJournal{chainDir: chainDir, seg: l}
		if l.recordCount() == 0 {
			if err := l.writeGenesis(genesis80); err != nil {
				return nil, err
			}
		}
		j.refreshCountCache()
		return j, nil
	}
	if st, err := os.Stat(monolith); err == nil && st.Size() > 0 {
		if repaired, rerr := RepairMonolithFromCheckpoint(chainDir, monolith); rerr == nil && repaired {
			fmt.Fprintf(os.Stderr, "DogeGo: repaired headers.bin from headers_sync.json checkpoint\n")
		}
		if st.Size()/80 <= headerMigrateMaxRecords {
			l, err := migrateMonolithToSegments(chainDir, monolith)
			if err != nil {
				fmt.Fprintf(os.Stderr, "DogeGo: header segment migration failed (%v); using headers.bin\n", err)
			} else {
				_ = l.repairFromCheckpoint()
				j := &HeaderJournal{chainDir: chainDir, seg: l}
				j.refreshCountCache()
				return j, nil
			}
		}
		j, err := OpenHeaderJournal(monolith, genesis80)
		if err != nil {
			return nil, err
		}
		j.chainDir = chainDir
		return j, nil
	}
	l, err := openHeaderSegmentLayout(chainDir)
	if err != nil {
		return nil, err
	}
	if err := l.writeGenesis(genesis80); err != nil {
		return nil, err
	}
	j := &HeaderJournal{chainDir: chainDir, seg: l}
	j.refreshCountCache()
	return j, nil
}

// HeaderLayout reports "segments" or "monolith".
func (j *HeaderJournal) HeaderLayout() string {
	if j != nil && j.seg != nil {
		return headerLayoutSegments
	}
	return "monolith"
}

// ChainDir returns the network datadir when known.
func (j *HeaderJournal) ChainDir() string {
	if j == nil {
		return ""
	}
	if j.chainDir != "" {
		return j.chainDir
	}
	if j.path != "" {
		return filepath.Dir(j.path)
	}
	return ""
}
