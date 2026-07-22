// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"os"
	"path/filepath"

	"dogego/chain"
	"dogego/store"
)

// MainnetFieldDataDir resolves operator mainnet chain dir (DOGEGO_FIELD_DATADIR or dogedata/mainnet).
func MainnetFieldDataDir() string {
	if d := os.Getenv("DOGEGO_FIELD_DATADIR"); d != "" {
		return d
	}
	for _, rel := range []string{
		filepath.Join("..", "dogedata", "mainnet"),
		filepath.Join("..", "..", "dogedata", "mainnet"),
	} {
		if _, err := os.Stat(filepath.Join(rel, "headers")); err == nil {
			return rel
		}
		if _, err := os.Stat(filepath.Join(rel, "headers.bin")); err == nil {
			return rel
		}
	}
	return filepath.Join("..", "dogedata", "mainnet")
}

func mainnetFieldDataDir() string { return MainnetFieldDataDir() }

// MainnetFieldDiskChain is operator dogedata/mainnet headers + rawblocks (when synced).
type MainnetFieldDiskChain struct {
	ChainDir          string
	Journal           *store.HeaderJournal
	Raw               *store.RawBlockStore
	TxIndex           *store.TxIndex
	BundledContiguous int64
}

// OpenMainnetFieldDiskChain opens operator field datadir headers and raw block store.
func OpenMainnetFieldDiskChain() (*MainnetFieldDiskChain, error) {
	chainDir := MainnetFieldDataDir()
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		return nil, err
	}
	seg := filepath.Join(chainDir, "headers")
	mono := filepath.Join(chainDir, "headers.bin")
	if _, err := os.Stat(seg); err != nil {
		if _, err2 := os.Stat(mono); err2 != nil {
			return nil, fmt.Errorf("mainnet field datadir missing headers: %s", chainDir)
		}
	}
	j, err := store.OpenHeaderChain(chainDir, gen[:80])
	if err != nil {
		return nil, err
	}
	rs, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		return nil, err
	}
	txIx, err := store.OpenTxIndex(chainDir)
	if err != nil {
		return nil, fmt.Errorf("tx index: %w", err)
	}
	cont := store.ReconcileBundledContiguousTip(j, rs, chain.MainnetDogecoin)
	return &MainnetFieldDiskChain{
		ChainDir:          chainDir,
		Journal:           j,
		Raw:               rs,
		TxIndex:           txIx,
		BundledContiguous: cont,
	}, nil
}
