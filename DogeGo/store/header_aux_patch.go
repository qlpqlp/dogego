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

	"dogego/chain"
	"dogego/wire"
)

const (
	// patchAuxInlineMaxTip avoids O(file) aux journal rewrites on every block when the header tip is huge.
	patchAuxInlineMaxTip = 16384
	// patchAuxFrontierWindow limits inline patch to heights near the stored body frontier.
	patchAuxFrontierWindow = 256
)

// shouldInlinePatchAux reports whether patching one height on block store is worth doing inline
// (vs batched BackfillAuxThroughHeight during deep IBD).
func shouldInlinePatchAux(net chain.Network, height, tip, contiguous int64) bool {
	if tip <= patchAuxInlineMaxTip {
		return true
	}
	if contiguous >= 0 && height >= contiguous-patchAuxFrontierWindow && height <= contiguous+patchAuxFrontierWindow {
		return true
	}
	if act := auxpowActivationHeight(net); act > 0 {
		if height >= act-patchAuxFrontierWindow && height <= act+patchAuxFrontierWindow*2 {
			// Deep IBD: batch backfill handles the activation window unless bodies are there now.
			if tip > patchAuxInlineMaxTip {
				return contiguous >= 0 && contiguous >= act-patchAuxFrontierWindow
			}
			return true
		}
	}
	return height >= tip-2048
}

func auxpowActivationHeight(net chain.Network) int64 {
	switch net {
	case chain.MainnetDogecoin:
		return 371337
	case chain.RebootTestnet:
		return 0
	default:
		return 0
	}
}

// PatchAuxFromBlockAtHeight writes auxpow from a stored block into headers_aux.bin when the slot is still empty.
func PatchAuxFromBlockAtHeight(j *HeaderJournal, aux *HeaderAuxJournal, net chain.Network, height int64, contiguous int64, blockRaw []byte) (bool, error) {
	if j == nil || aux == nil || len(blockRaw) == 0 {
		return false, nil
	}
	h80, err := j.ReadHeaderAt(height)
	if err != nil {
		return false, err
	}
	if !wire.HeaderHasAuxPowVersion(h80) {
		return false, nil
	}
	tip, err := j.TipHeight()
	if err != nil {
		return false, err
	}
	if height < 0 || height > tip {
		return false, fmt.Errorf("patch aux height %d out of range (tip %d)", height, tip)
	}
	if !shouldInlinePatchAux(net, height, tip, contiguous) {
		return false, nil
	}
	blob, ok, err := wire.ExtractAuxPowBytesFromBlock(blockRaw)
	if err != nil {
		return false, fmt.Errorf("height %d: %w", height, err)
	}
	if !ok || len(blob) == 0 {
		return false, nil
	}
	return aux.PatchRecordAt(height, blob)
}

// PatchRecordAt replaces one empty aux record without loading the full journal into memory.
func (a *HeaderAuxJournal) PatchRecordAt(height int64, blob []byte) (bool, error) {
	if a == nil || height < 0 {
		return false, fmt.Errorf("patch aux: invalid journal or height %d", height)
	}
	encoded, err := encodeAuxRecord(blob)
	if err != nil {
		return false, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if height >= int64(len(a.offsets)) {
		return false, fmt.Errorf("patch aux height %d out of range (records %d)", height, len(a.offsets))
	}
	data, err := os.ReadFile(a.path)
	if err != nil {
		return false, err
	}
	start := a.offsets[height]
	existing, err := decodeAuxRecord(data, start)
	if err != nil {
		return false, err
	}
	if len(existing) > 0 {
		return false, nil
	}
	if len(blob) == 0 {
		return false, nil
	}
	_, oldEnd, err := auxRecordBounds(data, start)
	if err != nil {
		return false, err
	}
	delta := int64(len(encoded)) - (oldEnd - start)
	out := make([]byte, 0, len(data)+int(delta))
	out = append(out, data[:start]...)
	out = append(out, encoded...)
	out = append(out, data[oldEnd:]...)
	if delta != 0 {
		for i := int(height) + 1; i < len(a.offsets); i++ {
			a.offsets[i] += delta
		}
	}
	tmp := a.path + ".patch"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, a.path); err != nil {
		return false, err
	}
	return true, nil
}

func encodeAuxRecord(blob []byte) ([]byte, error) {
	var rec bytes.Buffer
	if len(blob) == 0 {
		if err := wire.WriteCompactSize(&rec, 0); err != nil {
			return nil, err
		}
	} else {
		if err := wire.WriteCompactSize(&rec, uint64(len(blob))); err != nil {
			return nil, err
		}
		if _, err := rec.Write(blob); err != nil {
			return nil, err
		}
	}
	return rec.Bytes(), nil
}

func auxRecordBounds(data []byte, start int64) (int64, int64, error) {
	if start < 0 || start >= int64(len(data)) {
		return 0, 0, fmt.Errorf("aux offset %d out of range", start)
	}
	chunk := data[start:]
	r := bytes.NewReader(chunk)
	n, err := wire.ReadCompactSize(r)
	if err != nil {
		return 0, 0, err
	}
	prefixUsed := int64(len(chunk)) - int64(r.Len())
	end := start + prefixUsed + int64(n)
	if end > int64(len(data)) {
		return 0, 0, fmt.Errorf("aux truncated at %d", start)
	}
	return start, end, nil
}
