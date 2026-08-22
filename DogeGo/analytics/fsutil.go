// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DirSizeBytes sums file sizes under root (directories only traversed).
func DirSizeBytes(root string) (int64, error) {
	root = filepath.Clean(root)
	var total int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

// ChainStoreBytes returns best-effort on-disk sizes for DogeGo chain data components.
// total is the sum of the reported components (not a full datadir walk of wallet/analytics DBs).
func ChainStoreBytes(chainRoot string) (headers, rawblocks, txindex, total int64) {
	chainRoot = filepath.Clean(chainRoot)
	headers = SubdirSizeIfExists(filepath.Join(chainRoot, "headers"))
	headers += fileSizeIfExists(filepath.Join(chainRoot, "headers.bin"))
	headers += fileSizeIfExists(filepath.Join(chainRoot, "headers_aux.bin"))
	rawblocks, _ = DirSizeBytes(filepath.Join(chainRoot, "rawblocks"))
	txindex, _ = DirSizeBytes(filepath.Join(chainRoot, "indexes", "tx"))
	utxo := fileSizeIfExists(filepath.Join(chainRoot, "utxo.cache"))
	total = headers + rawblocks + txindex + utxo
	return headers, rawblocks, txindex, total
}

func fileSizeIfExists(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}

// SubdirSizeIfExists returns DirSizeBytes when path exists.
func SubdirSizeIfExists(path string) int64 {
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		if st != nil && !st.IsDir() {
			return st.Size()
		}
		return 0
	}
	n, _ := DirSizeBytes(path)
	return n
}

// IsIgnoredWalkError skips optional missing paths during walks.
func IsIgnoredWalkError(err error) bool {
	return err != nil && (os.IsNotExist(err) || strings.Contains(err.Error(), "not exist"))
}
