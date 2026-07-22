// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"dogego/pow"
	"dogego/wire"
)

// RewriteAll replaces the entire aux journal with one length-prefixed record per height (0..len(records)-1).
func (a *HeaderAuxJournal) RewriteAll(records [][]byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	tmp := a.path + ".rewrite"
	var file bytes.Buffer
	var offs []int64
	for _, blob := range records {
		offs = append(offs, int64(file.Len()))
		var rec bytes.Buffer
		if len(blob) == 0 {
			if err := wire.WriteCompactSize(&rec, 0); err != nil {
				return err
			}
		} else {
			if err := wire.WriteCompactSize(&rec, uint64(len(blob))); err != nil {
				return err
			}
			if _, err := rec.Write(blob); err != nil {
				return err
			}
		}
		if _, err := file.Write(rec.Bytes()); err != nil {
			return err
		}
	}
	if err := os.WriteFile(tmp, file.Bytes(), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, a.path); err != nil {
		return err
	}
	a.offsets = offs
	return nil
}

// BackfillAuxFromRawBlocks fills empty auxpow slots from stored raw block files (headers must already be in the journal).
// Returns the number of heights where aux data was newly written.
func BackfillAuxFromRawBlocks(j *HeaderJournal, aux *HeaderAuxJournal, raw *RawBlockStore) (int, error) {
	if j == nil || aux == nil || raw == nil {
		return 0, fmt.Errorf("backfill: nil journal, aux, or raw store")
	}
	tip, err := j.TipHeight()
	if err != nil {
		return 0, err
	}
	return BackfillAuxThroughHeight(j, aux, raw, tip)
}

// BackfillAuxThroughHeight fills empty auxpow slots for heights [0, through] inclusive (one aux file read).
func BackfillAuxThroughHeight(j *HeaderJournal, aux *HeaderAuxJournal, raw *RawBlockStore, through int64) (int, error) {
	if j == nil || aux == nil || raw == nil {
		return 0, fmt.Errorf("backfill: nil journal, aux, or raw store")
	}
	tip, err := j.TipHeight()
	if err != nil {
		return 0, err
	}
	if through < 0 {
		return 0, nil
	}
	if through > tip {
		through = tip
	}
	want := tip + 1
	if err := aux.EnsureRecordCount(want); err != nil {
		return 0, err
	}
	auxData, auxOffs, err := aux.SnapshotForBackfill()
	if err != nil {
		return 0, err
	}
	replace := make(map[int64][]byte)
	filled := 0
	for h := int64(0); h <= through; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return filled, err
		}
		if !wire.HeaderHasAuxPowVersion(h80) {
			continue
		}
		if blob, err := decodeAuxRecord(auxData, auxOffs[h]); err != nil {
			return filled, fmt.Errorf("height %d: %w", h, err)
		} else if len(blob) > 0 {
			continue
		}
		hash := pow.BlockHashLE(h80)
		blockRaw, err := raw.Get(hash)
		if err != nil {
			continue
		}
		blob, ok, err := wire.ExtractAuxPowBytesFromBlock(blockRaw)
		if err != nil {
			return filled, fmt.Errorf("height %d: %w", h, err)
		}
		if !ok {
			continue
		}
		replace[h] = blob
		filled++
	}
	if filled == 0 {
		return 0, nil
	}
	if through == tip && tip <= patchAuxInlineMaxTip {
		records, err := aux.LoadAllRecords()
		if err != nil {
			return 0, err
		}
		if int64(len(records)) != want {
			return 0, fmt.Errorf("backfill: aux records %d != header count %d", len(records), want)
		}
		for h, blob := range replace {
			records[h] = blob
		}
		if err := aux.RewriteAll(records); err != nil {
			return 0, err
		}
		return filled, nil
	}
	if err := aux.rewriteRecordsThrough(through, replace); err != nil {
		return 0, err
	}
	return filled, nil
}

// rewriteRecordsThrough re-encodes heights [0, through] (optionally replaced) and keeps the tail unchanged.
func (a *HeaderAuxJournal) rewriteRecordsThrough(through int64, replace map[int64][]byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := len(a.offsets)
	if through < 0 || through >= int64(n) {
		return fmt.Errorf("rewrite aux through %d out of range (records %d)", through, n)
	}
	data, err := os.ReadFile(a.path)
	if err != nil {
		return err
	}
	var prefix bytes.Buffer
	newOffs := make([]int64, 0, n)
	for h := int64(0); h <= through; h++ {
		newOffs = append(newOffs, int64(prefix.Len()))
		if blob, ok := replace[h]; ok {
			enc, err := encodeAuxRecord(blob)
			if err != nil {
				return err
			}
			if _, err := prefix.Write(enc); err != nil {
				return err
			}
			continue
		}
		start := a.offsets[h]
		_, end, err := auxRecordBounds(data, start)
		if err != nil {
			return fmt.Errorf("height %d: %w", h, err)
		}
		if _, err := prefix.Write(data[start:end]); err != nil {
			return err
		}
	}
	var out bytes.Buffer
	if _, err := out.Write(prefix.Bytes()); err != nil {
		return err
	}
	tailBase := int64(out.Len())
	if int(through) < n-1 {
		oldTail := a.offsets[through+1]
		if _, err := out.Write(data[oldTail:]); err != nil {
			return err
		}
		for i := int(through) + 1; i < n; i++ {
			newOffs = append(newOffs, tailBase+a.offsets[i]-oldTail)
		}
	}
	tmp := a.path + ".rewrite"
	if err := os.WriteFile(tmp, out.Bytes(), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, a.path); err != nil {
		return err
	}
	a.offsets = newOffs
	return nil
}

// AuxJournalPath returns the conventional path under chain datadir.
func AuxJournalPath(chainDataDir string) string {
	return filepath.Join(chainDataDir, "headers_aux.bin")
}

// RecoverHeaderAuxJournal rebuilds headers_aux.bin with empty records aligned to headerCount
// when the on-disk index is corrupt (e.g. after an older rebuildOffsets bug). The prior file
// is renamed to path+".corrupt" when present. Call BackfillAuxFromRawBlocks afterward to refill auxpow.
func RecoverHeaderAuxJournal(path string, headerCount int64) (*HeaderAuxJournal, error) {
	if headerCount < 0 {
		return nil, fmt.Errorf("recover header aux: negative count %d", headerCount)
	}
	if st, err := os.Stat(path); err == nil && st.Size() > 0 {
		backup := path + ".corrupt"
		_ = os.Remove(backup)
		if err := os.Rename(path, backup); err != nil {
			return nil, fmt.Errorf("recover header aux: backup: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	const chunkRecords = 1 << 20 // 1 Mi empty slots per write
	buf := make([]byte, chunkRecords)
	for written := int64(0); written < headerCount; {
		n := headerCount - written
		if n > chunkRecords {
			n = chunkRecords
		}
		if _, err := f.Write(buf[:n]); err != nil {
			_ = f.Close()
			return nil, err
		}
		written += n
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return OpenHeaderAuxJournal(path, headerCount)
}
