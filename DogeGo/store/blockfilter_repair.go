// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"dogego/pow"
)

// RepairBlockFiltersReport summarizes a filter backfill pass.
type RepairBlockFiltersReport struct {
	BlocksIndexed int
}

// BlockFilterIndexer builds and stores a basic filter for one block.
type BlockFilterIndexer func(hashLE [32]byte, blockRaw []byte) error

// RepairBlockFiltersIfLag builds missing filters when filter files trail raw block count.
func RepairBlockFiltersIfLag(j *HeaderJournal, chainDir string, minRawBlocks int, index BlockFilterIndexer) (RepairBlockFiltersReport, bool, error) {
	var empty RepairBlockFiltersReport
	if j == nil || index == nil {
		return empty, false, nil
	}
	raw, err := OpenRawBlockStore(chainDir)
	if err != nil {
		return empty, false, err
	}
	rawN, err := raw.Count()
	if err != nil || rawN < minRawBlocks {
		return empty, false, err
	}
	filters, err := OpenBlockFilterIndex(chainDir)
	if err != nil {
		return empty, false, err
	}
	filterN, err := filters.Count()
	if err != nil {
		return empty, false, err
	}
	if filterN >= rawN {
		return empty, false, nil
	}
	rep, err := RepairBlockFiltersFromRaw(j, raw, filters, index)
	if err != nil {
		return empty, false, err
	}
	return rep, true, nil
}

// RepairBlockFiltersFromRaw builds filters for every raw block in journal order (genesis..tip).
func RepairBlockFiltersFromRaw(j *HeaderJournal, raw *RawBlockStore, filters *BlockFilterIndex, index BlockFilterIndexer) (RepairBlockFiltersReport, error) {
	return indexBlockFiltersFromRaw(j, raw, filters, index, true)
}

// RebuildBlockFiltersFromRaw re-encodes every stored block filter (e.g. after SipHash or GCS fixes).
func RebuildBlockFiltersFromRaw(j *HeaderJournal, raw *RawBlockStore, filters *BlockFilterIndex, index BlockFilterIndexer) (RepairBlockFiltersReport, error) {
	return indexBlockFiltersFromRaw(j, raw, filters, index, false)
}

func indexBlockFiltersFromRaw(j *HeaderJournal, raw *RawBlockStore, filters *BlockFilterIndex, index BlockFilterIndexer, skipExisting bool) (RepairBlockFiltersReport, error) {
	return indexBlockFiltersFromRawBounded(j, raw, filters, index, skipExisting, 0)
}

// indexBlockFiltersFromRawBounded indexes missing filters up to contiguous stored bodies (not header tip during IBD).
// maxIndexed <= 0 means no cap.
func indexBlockFiltersFromRawBounded(j *HeaderJournal, raw *RawBlockStore, filters *BlockFilterIndex, index BlockFilterIndexer, skipExisting bool, maxIndexed int) (RepairBlockFiltersReport, error) {
	var rep RepairBlockFiltersReport
	if j == nil || raw == nil || filters == nil || index == nil {
		return rep, nil
	}
	end, err := j.TipHeight()
	if err != nil {
		return rep, err
	}
	if cont, err := ContiguousRawBodyHeight(j, raw); err == nil && cont >= 0 && cont < end {
		end = cont
	}
	for h := int64(0); h <= end; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			continue
		}
		id := pow.BlockHashLE(h80)
		if skipExisting && filters.Has(id) {
			continue
		}
		payload, err := raw.Get(id)
		if err != nil {
			continue
		}
		if err := index(id, payload); err != nil {
			continue
		}
		rep.BlocksIndexed++
		if maxIndexed > 0 && rep.BlocksIndexed >= maxIndexed {
			break
		}
	}
	return rep, nil
}
