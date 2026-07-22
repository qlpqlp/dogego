// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dogego/chain"
)

// PrepareChainDataDir returns the per-network chain root (datadir/<mainnet|testnet>/),
// creating it and optionally migrating legacy headers.bin / rawblocks / wallet.json
// from the base datadir when the legacy genesis matches g80.
func PrepareChainDataDir(baseDataDir string, net chain.Network, g80 [80]byte) (chainRoot string, migrated bool, err error) {
	sub, err := chain.ChainDataDirName(net)
	if err != nil {
		return "", false, err
	}
	chainRoot = filepath.Join(baseDataDir, sub)
	if err := os.MkdirAll(chainRoot, 0o700); err != nil {
		return "", false, err
	}
	migrated, err = migrateLegacyFlatDir(baseDataDir, chainRoot, g80, net)
	if err != nil {
		return "", false, err
	}
	return chainRoot, migrated, nil
}

func readFirstHeader80(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var buf [80]byte
	if _, err := io.ReadFull(f, buf[:]); err != nil {
		return nil, err
	}
	return buf[:], nil
}

func migrateLegacyFlatDir(baseDir, chainRoot string, g80 [80]byte, net chain.Network) (bool, error) {
	newHdr := filepath.Join(chainRoot, "headers.bin")
	if _, err := os.Stat(newHdr); err == nil {
		return false, nil
	}
	legacyHdr := filepath.Join(baseDir, "headers.bin")
	st, err := os.Stat(legacyHdr)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if st.IsDir() {
		return false, fmt.Errorf("unexpected directory at %s", legacyHdr)
	}
	first, err := readFirstHeader80(legacyHdr)
	if err != nil {
		return false, fmt.Errorf("read legacy genesis: %w", err)
	}
	if !bytes.Equal(first, g80[:]) {
		netName := "testnet"
		if net == chain.MainnetDogecoin {
			netName = "mainnet"
		}
		return false, fmt.Errorf(
			"legacy headers.bin at %q does not match %s genesis (likely old -network data). "+
				"Remove that file or switch -network back, or delete it and use chain data under %q",
			legacyHdr, netName, chainRoot)
	}
	if err := os.Rename(legacyHdr, newHdr); err != nil {
		return false, fmt.Errorf("migrate headers.bin into %s: %w", chainRoot, err)
	}
	migrated := true

	legacyRB := filepath.Join(baseDir, "rawblocks")
	newRB := filepath.Join(chainRoot, "rawblocks")
	if _, err := os.Stat(legacyRB); err == nil {
		if _, err := os.Stat(newRB); err != nil {
			if os.IsNotExist(err) {
				if err := os.Rename(legacyRB, newRB); err != nil {
					fmt.Fprintf(os.Stderr, "DogeGo: could not move rawblocks into %s: %v\n", chainRoot, err)
				} else {
					migrated = true
				}
			}
		}
	}

	if net == chain.RebootTestnet {
		lw := filepath.Join(baseDir, "wallet.json")
		nw := filepath.Join(chainRoot, "wallet.json")
		if _, err := os.Stat(nw); os.IsNotExist(err) {
			if _, err := os.Stat(lw); err == nil {
				if err := os.Rename(lw, nw); err != nil {
					fmt.Fprintf(os.Stderr, "DogeGo: could not move wallet.json into %s: %v\n", chainRoot, err)
				} else {
					migrated = true
				}
			}
		}
	}

	return migrated, nil
}
