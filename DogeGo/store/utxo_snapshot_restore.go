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
	"sort"
	"strings"
)

// FindQuarantinedUtxoSnapshots returns stale utxo.cache paths sorted by descending tip height.
func FindQuarantinedUtxoSnapshots(chainDir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(chainDir, "utxo.cache.stale*"))
	if err != nil {
		return nil, err
	}
	type scored struct {
		path string
		tip  int64
	}
	var rows []scored
	for _, p := range matches {
		tip, _, err := ReadUtxoSnapshotTipAndCount(p)
		if err != nil || tip < 0 {
			continue
		}
		rows = append(rows, scored{path: p, tip: tip})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].tip != rows[j].tip {
			return rows[i].tip > rows[j].tip
		}
		return rows[i].path > rows[j].path
	})
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.path)
	}
	return out, nil
}

// RestoreQuarantinedUtxoSnapshot copies the best stale snapshot back to utxo.cache when it
// carries more chainstate than the current file (operator / startup recovery).
func RestoreQuarantinedUtxoSnapshot(chainDir, stalePath string) (restoredTip int64, err error) {
	if chainDir == "" || stalePath == "" {
		return -1, fmt.Errorf("utxo restore: missing path")
	}
	staleTip, staleCoins, err := ReadUtxoSnapshotTipAndCount(stalePath)
	if err != nil {
		return -1, err
	}
	if staleTip < 0 {
		return -1, fmt.Errorf("utxo restore: stale file missing or empty")
	}
	dst := UtxoSnapshotPath(chainDir)
	curTip, curCoins, _ := ReadUtxoSnapshotTipAndCount(dst)
	if curTip >= staleTip && curCoins >= staleCoins && curCoins > 0 {
		return curTip, nil
	}
	if err := copyFileAtomic(stalePath, dst); err != nil {
		return -1, err
	}
	return staleTip, nil
}

// TryRestoreBestQuarantinedUtxoSnapshot picks the highest-tip stale snapshot when the active
// utxo.cache is absent or clearly superseded by a quarantined replay snapshot.
func TryRestoreBestQuarantinedUtxoSnapshot(chainDir string) (restoredTip int64, restoredFrom string, err error) {
	stalePaths, err := FindQuarantinedUtxoSnapshots(chainDir)
	if err != nil {
		return -1, "", err
	}
	if len(stalePaths) == 0 {
		return -1, "", nil
	}
	dst := UtxoSnapshotPath(chainDir)
	curTip, curCoins, _ := ReadUtxoSnapshotTipAndCount(dst)
	for _, stalePath := range stalePaths {
		staleTip, staleCoins, err := ReadUtxoSnapshotTipAndCount(stalePath)
		if err != nil || staleTip < 0 {
			continue
		}
		if staleTip <= curTip && staleCoins <= curCoins {
			continue
		}
		// Prefer real chainstate (coin map) over a tiny misaligned save.
		if staleTip >= 512 && staleCoins < 8 && curTip >= 0 {
			continue
		}
		if tip, err := RestoreQuarantinedUtxoSnapshot(chainDir, stalePath); err != nil {
			return -1, "", err
		} else {
			return tip, stalePath, nil
		}
	}
	return -1, "", nil
}

func copyFileAtomic(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".restore.tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// StaleUtxoSnapshotReason extracts the reason suffix from utxo.cache.stale.<reason>.
func StaleUtxoSnapshotReason(path string) string {
	base := filepath.Base(path)
	const prefix = "utxo.cache.stale."
	if strings.HasPrefix(base, prefix) {
		return strings.TrimPrefix(base, prefix)
	}
	return ""
}
