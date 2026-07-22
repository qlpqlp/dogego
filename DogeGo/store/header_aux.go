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
	"sync"

	"dogego/wire"
)

// HeaderAuxJournal stores serialized CAuxPow blobs parallel to headers.bin (one length-prefixed record per height).
type HeaderAuxJournal struct {
	path    string
	mu      sync.Mutex
	offsets []int64
}

// OpenHeaderAuxJournal opens or creates headers_aux.bin. headerCount is the current headers.bin record count;
// missing aux records are padded with empty entries so heights stay aligned.
func OpenHeaderAuxJournal(path string, headerCount int64) (*HeaderAuxJournal, error) {
	a := &HeaderAuxJournal{path: path}
	if err := a.rebuildOffsets(); err != nil {
		if repaired, rerr := repairHeaderAuxTornTail(path); rerr == nil && repaired {
			fmt.Fprintf(os.Stderr, "DogeGo: repaired torn headers_aux.bin tail\n")
			if err2 := a.rebuildOffsets(); err2 != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	if headerCount > int64(len(a.offsets)) {
		if err := a.padToCount(headerCount); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// repairHeaderAuxTornTail truncates a force-killed partial aux record at EOF (crash during AppendEntries).
func repairHeaderAuxTornTail(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if len(data) == 0 {
		return false, nil
	}
	var lastGoodEnd int64
	pos := int64(0)
	for pos < int64(len(data)) {
		recordStart := pos
		chunk := data[pos:]
		r := bytes.NewReader(chunk)
		n, err := wire.ReadCompactSize(r)
		if err != nil {
			break
		}
		prefixUsed := int64(len(chunk)) - int64(r.Len())
		if prefixUsed <= 0 {
			break
		}
		recordLen := prefixUsed + int64(n)
		if recordStart+recordLen > int64(len(data)) {
			break
		}
		lastGoodEnd = recordStart + recordLen
		pos = lastGoodEnd
	}
	if lastGoodEnd == int64(len(data)) {
		return false, nil
	}
	if lastGoodEnd == 0 {
		return false, fmt.Errorf("header aux entirely corrupt")
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		return false, err
	}
	if err := f.Truncate(lastGoodEnd); err != nil {
		_ = f.Close()
		return false, err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return false, err
	}
	_ = f.Close()
	return true, nil
}

// RecordCount returns the number of aux records (equals aligned header count after pad).
func (a *HeaderAuxJournal) RecordCount() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return int64(len(a.offsets))
}

func (a *HeaderAuxJournal) rebuildOffsets() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	st, err := os.Stat(a.path)
	if err != nil {
		if os.IsNotExist(err) {
			a.offsets = nil
			return nil
		}
		return err
	}
	if st.Size() == 0 {
		a.offsets = nil
		return nil
	}
	data, err := os.ReadFile(a.path)
	if err != nil {
		return err
	}
	var offs []int64
	pos := int64(0)
	for pos < int64(len(data)) {
		offs = append(offs, pos)
		chunk := data[pos:]
		r := bytes.NewReader(chunk)
		n, err := wire.ReadCompactSize(r)
		if err != nil {
			return fmt.Errorf("header aux corrupt at %d: %w", pos, err)
		}
		prefixUsed := int64(len(chunk)) - int64(r.Len())
		if prefixUsed <= 0 {
			return fmt.Errorf("header aux corrupt record at %d", pos)
		}
		recordLen := prefixUsed + int64(n)
		if recordLen > int64(len(chunk)) {
			return fmt.Errorf("header aux corrupt length at %d", pos)
		}
		pos += recordLen
	}
	a.offsets = offs
	return nil
}

func (a *HeaderAuxJournal) padToCount(n int64) error {
	for int64(len(a.offsets)) < n {
		if err := a.appendLocked(nil); err != nil {
			return err
		}
	}
	return nil
}

// AppendEntries appends one aux record per header (nil or empty = no aux data at this height).
func (a *HeaderAuxJournal) AppendEntries(blobs [][]byte) error {
	if len(blobs) == 0 {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	f, err := os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	base := st.Size()
	var batch bytes.Buffer
	newOffs := make([]int64, 0, len(blobs))
	pos := base
	for _, blob := range blobs {
		newOffs = append(newOffs, pos)
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
		b := rec.Bytes()
		if _, err := batch.Write(b); err != nil {
			return err
		}
		pos += int64(len(b))
	}
	if _, err := f.Write(batch.Bytes()); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	a.offsets = append(a.offsets, newOffs...)
	return nil
}

func (a *HeaderAuxJournal) appendLocked(blob []byte) error {
	return a.AppendEntries([][]byte{blob})
}

// DecodeAuxRecordAt decodes auxpow bytes at height from a SnapshotForBackfill buffer.
func DecodeAuxRecordAt(data []byte, offsets []int64, height int64) ([]byte, error) {
	if height < 0 || height >= int64(len(offsets)) {
		return nil, fmt.Errorf("header aux height %d out of snapshot range (records %d)", height, len(offsets))
	}
	return decodeAuxRecord(data, offsets[height])
}

func decodeAuxRecord(data []byte, pos int64) ([]byte, error) {
	if pos < 0 || pos >= int64(len(data)) {
		return nil, fmt.Errorf("header aux offset %d out of range (file %d bytes)", pos, len(data))
	}
	r := bytes.NewReader(data[pos:])
	n, err := wire.ReadCompactSize(r)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	skip := int(pos) + len(data[pos:]) - r.Len()
	if skip+int(n) > len(data) {
		return nil, fmt.Errorf("header aux truncated at offset %d", pos)
	}
	return append([]byte(nil), data[skip:skip+int(n)]...), nil
}

// SnapshotForBackfill returns one aux file read and height offsets (for batched decode in a height range).
func (a *HeaderAuxJournal) SnapshotForBackfill() ([]byte, []int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.offsets) == 0 {
		return nil, nil, nil
	}
	data, err := os.ReadFile(a.path)
	if err != nil {
		return nil, nil, err
	}
	offs := make([]int64, len(a.offsets))
	copy(offs, a.offsets)
	return data, offs, nil
}

// LoadAllRecords reads the full aux journal into memory (one file read; used for batched backfill).
func (a *HeaderAuxJournal) LoadAllRecords() ([][]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.offsets) == 0 {
		return nil, nil
	}
	data, err := os.ReadFile(a.path)
	if err != nil {
		return nil, err
	}
	out := make([][]byte, len(a.offsets))
	for i, pos := range a.offsets {
		blob, err := decodeAuxRecord(data, pos)
		if err != nil {
			return nil, fmt.Errorf("height %d: %w", i, err)
		}
		out[i] = blob
	}
	return out, nil
}

// recordByteRange returns the on-disk span for one aux record (no full-file read).
func (a *HeaderAuxJournal) recordByteRange(height int64) (start, length int64, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if height < 0 || height >= int64(len(a.offsets)) {
		return 0, 0, fmt.Errorf("header aux height %d out of range (records %d)", height, len(a.offsets))
	}
	start = a.offsets[height]
	var end int64
	if int(height)+1 < len(a.offsets) {
		end = a.offsets[height+1]
	} else {
		st, statErr := os.Stat(a.path)
		if statErr != nil {
			return 0, 0, statErr
		}
		end = st.Size()
	}
	if end < start {
		return 0, 0, fmt.Errorf("header aux corrupt span at height %d", height)
	}
	return start, end - start, nil
}

// ReadAt returns the serialized auxpow bytes at height, or nil if none was stored.
func (a *HeaderAuxJournal) ReadAt(height int64) ([]byte, error) {
	start, length, err := a.recordByteRange(height)
	if err != nil {
		return nil, err
	}
	if length == 0 {
		return nil, nil
	}
	f, err := os.Open(a.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, length)
	if _, err := f.ReadAt(buf, start); err != nil {
		return nil, err
	}
	return decodeAuxRecord(buf, 0)
}

// EnsureRecordCount pads or truncates aux records so len(offsets) equals header journal Count().
func (a *HeaderAuxJournal) EnsureRecordCount(headerCount int64) error {
	if a == nil || headerCount < 0 {
		return nil
	}
	a.mu.Lock()
	cur := int64(len(a.offsets))
	a.mu.Unlock()
	if cur < headerCount {
		return a.padToCount(headerCount)
	}
	if cur > headerCount {
		if headerCount == 0 {
			return a.TruncateToHeight(-1)
		}
		return a.TruncateToHeight(headerCount - 1)
	}
	return nil
}

// TruncateToHeight removes aux records above inclusiveHeight (must match header journal reorg).
func (a *HeaderAuxJournal) TruncateToHeight(inclusiveHeight int64) error {
	if inclusiveHeight < -1 {
		return fmt.Errorf("negative truncate height %d", inclusiveHeight)
	}
	want := inclusiveHeight + 1
	a.mu.Lock()
	defer a.mu.Unlock()
	if int64(len(a.offsets)) <= want {
		// Aux can lag headers.bin during IBD (backfill fills later). Rewinding the main
		// journal above the aux tip does not require growing or failing aux truncate.
		return nil
	}
	if want == 0 {
		if err := os.Remove(a.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		a.offsets = nil
		return nil
	}
	data, err := os.ReadFile(a.path)
	if err != nil {
		return err
	}
	cut := a.offsets[want]
	if cut > int64(len(data)) {
		return fmt.Errorf("header aux corrupt offset at height %d", want)
	}
	if err := os.WriteFile(a.path, data[:cut], 0o600); err != nil {
		return err
	}
	a.offsets = a.offsets[:want]
	return nil
}
