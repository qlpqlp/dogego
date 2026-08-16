// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"
	"os"

	"dogego/pow"
	"dogego/wire"
)

// MaxHeadersPerMessage matches Core MAX_HEADERS_RESULTS.
const MaxHeadersPerMessage = 2000

// FindLocatorForkHeight returns the highest journal height matching any locator hash (newest-first order).
// If none match, returns genesis height 0.
//
// One tip→genesis pass against the locator set (segment-sized reads). The old per-hash
// HeightByBlockHashLE loop was O(tip×locator) and wedged IBD when peers asked getheaders
// with locators that only matched genesis (common during early peer IBD).
func FindLocatorForkHeight(j *HeaderJournal, locator [][32]byte) (int64, error) {
	if j == nil {
		return 0, fmt.Errorf("nil journal")
	}
	if len(locator) == 0 {
		return 0, nil
	}
	tip, err := j.TipHeight()
	if err != nil {
		return 0, err
	}
	if tip < 0 {
		return 0, nil
	}
	want := make(map[[32]byte]struct{}, len(locator))
	for _, h := range locator {
		want[h] = struct{}{}
	}
	if tipHash, err := j.LastTipHash(); err == nil {
		if _, ok := want[tipHash]; ok {
			return tip, nil
		}
	}
	if h, ok, err := j.highestLocatorMatch(tip, want); err != nil {
		return 0, err
	} else if ok {
		return h, nil
	}
	return 0, nil
}

// locatorMatchScanCap limits tip→genesis work when answering getheaders during IBD.
// Same-chain peers behind us match within this window; genesis-only locators return 0
// without hashing the entire journal (was freezing header download under inbound load).
const locatorMatchScanCap int64 = 100_000

// highestLocatorMatch walks tip→0 and returns the highest height whose hash is in want.
func (j *HeaderJournal) highestLocatorMatch(tip int64, want map[[32]byte]struct{}) (int64, bool, error) {
	if tip < 0 || len(want) == 0 {
		return 0, false, nil
	}
	if j.seg != nil {
		return j.seg.highestLocatorMatch(tip, want)
	}
	floor := tip - locatorMatchScanCap
	if floor < 0 {
		floor = 0
	}
	for h := tip; h >= floor; h-- {
		hdr, err := j.ReadHeaderAt(h)
		if err != nil {
			return 0, false, err
		}
		if _, ok := want[pow.BlockHashLE(hdr)]; ok {
			return h, true, nil
		}
	}
	return 0, false, nil
}

func (l *headerSegmentLayout) highestLocatorMatch(tip int64, want map[[32]byte]struct{}) (int64, bool, error) {
	if tip < 0 || len(want) == 0 {
		return 0, false, nil
	}
	segSize := int64(HeaderSegmentSize)
	floor := tip - locatorMatchScanCap
	if floor < 0 {
		floor = 0
	}
	segStart := (tip / segSize) * segSize
	for segStart >= 0 {
		if segStart+segSize <= floor {
			break
		}
		b, err := os.ReadFile(l.segmentPath(segStart))
		if err != nil {
			if os.IsNotExist(err) {
				if segStart == 0 || segStart < floor {
					break
				}
				segStart -= segSize
				continue
			}
			return 0, false, err
		}
		maxH := segStart + int64(len(b)/80) - 1
		if maxH > tip {
			maxH = tip
		}
		minH := segStart
		if minH < floor {
			minH = floor
		}
		for h := maxH; h >= minH; h-- {
			off := int((h - segStart) * 80)
			if off+80 > len(b) {
				continue
			}
			if _, ok := want[pow.BlockHashLE(b[off:off+80])]; ok {
				return h, true, nil
			}
		}
		if segStart == 0 || segStart <= floor {
			break
		}
		segStart -= segSize
	}
	return 0, false, nil
}

// HeadersAfterFork builds up to maxHeaders entries after forkHeight (exclusive), stopping before hashStop.
func HeadersAfterFork(j *HeaderJournal, aux *HeaderAuxJournal, forkHeight int64, hashStop [32]byte, maxHeaders int) ([]wire.DecodedHeader, error) {
	if j == nil {
		return nil, fmt.Errorf("nil journal")
	}
	if maxHeaders <= 0 || maxHeaders > MaxHeadersPerMessage {
		maxHeaders = MaxHeadersPerMessage
	}
	tip, err := j.TipHeight()
	if err != nil {
		return nil, err
	}
	if forkHeight >= tip {
		return nil, nil
	}
	var zero [32]byte
	stopOnHash := hashStop != zero
	out := make([]wire.DecodedHeader, 0, maxHeaders)
	var auxData []byte
	var auxOffs []int64
	if aux != nil {
		var err error
		auxData, auxOffs, err = aux.SnapshotForBackfill()
		if err != nil {
			return nil, err
		}
	}
	for h := forkHeight + 1; h <= tip && len(out) < maxHeaders; h++ {
		hdr, err := j.ReadHeaderAt(h)
		if err != nil {
			return nil, err
		}
		id := pow.BlockHashLE(hdr)
		if stopOnHash && id == hashStop {
			break
		}
		dh := wire.DecodedHeader{Header80: hdr}
		if wire.HeaderHasAuxPowVersion(hdr) {
			if aux != nil && h < int64(len(auxOffs)) {
				blob, err := DecodeAuxRecordAt(auxData, auxOffs, h)
				if err != nil {
					return nil, fmt.Errorf("height %d: %w", h, err)
				}
				if len(blob) > 0 {
					dh.AuxWire = blob
				}
			}
			if len(dh.AuxWire) == 0 && dh.Aux == nil {
				return nil, fmt.Errorf("height %d: auxpow header without aux journal entry", h)
			}
		}
		out = append(out, dh)
	}
	return out, nil
}
