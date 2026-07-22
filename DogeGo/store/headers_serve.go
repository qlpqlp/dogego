// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"

	"dogego/pow"
	"dogego/wire"
)

// MaxHeadersPerMessage matches Core MAX_HEADERS_RESULTS.
const MaxHeadersPerMessage = 2000

// FindLocatorForkHeight returns the highest journal height matching any locator hash (newest-first order).
// If none match, returns genesis height 0.
func FindLocatorForkHeight(j *HeaderJournal, locator [][32]byte) (int64, error) {
	if j == nil {
		return 0, fmt.Errorf("nil journal")
	}
	for _, h := range locator {
		height, err := j.HeightByBlockHashLE(h)
		if err == nil {
			return height, nil
		}
	}
	if _, err := j.Count(); err != nil {
		return 0, err
	}
	return 0, nil
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
