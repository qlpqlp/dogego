// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"log"
	"os"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// ValidateStoredBlockBodies runs CheckBlockPayload and ConnectBlockRaw on stored raw blocks from startHeight through endHeight.
// Heights without a raw block file are skipped. Connect requires raw blocks for all ancestors through height-1.
func ValidateStoredBlockBodies(j *store.HeaderJournal, raw *store.RawBlockStore, index TxIndexer, utxo UtxoOutpointSource, net chain.Network, startHeight, endHeight int64) error {
	return WithFullScriptVerification(func() error {
		return validateStoredBlockBodies(j, raw, index, utxo, net, startHeight, endHeight)
	})
}

func validateStoredBlockBodies(j *store.HeaderJournal, raw *store.RawBlockStore, index TxIndexer, utxo UtxoOutpointSource, net chain.Network, startHeight, endHeight int64) error {
	if j == nil || raw == nil || index == nil {
		return fmt.Errorf("validate block bodies: nil journal, raw store, or tx index")
	}
	if startHeight < 0 || endHeight < startHeight {
		return fmt.Errorf("invalid height range %d..%d", startHeight, endHeight)
	}
	chainView := ConnectBlockPrevOutView(index, raw, utxo)
	span := endHeight - startHeight + 1
	verbose := os.Getenv("DOGEGO_FIELD_DISK_CONNECT_VERBOSE") == "1"
	// Incremental ancestor frontier - storedBodiesThrough is O(h) per height; avoid O(n²) on deep tiers.
	ancestorFrontier := int64(-1)
	for h := startHeight; h <= endHeight; h++ {
		if verbose && span >= 256 && (h-startHeight)%256 == 0 && h > startHeight {
			log.Printf("dogego: disk connect progress height %d / %d", h, endHeight)
		}
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return err
		}
		id := pow.BlockHashLE(h80)
		if !raw.Has(id) {
			continue
		}
		if h > 0 {
			if ancestorFrontier < h-1 {
				if !extendStoredBodiesFrontier(j, raw, &ancestorFrontier, h-1) {
					return fmt.Errorf("height %d: missing ancestor raw blocks for connect", h)
				}
			}
		}
		payload, err := raw.Get(id)
		if err != nil {
			return fmt.Errorf("height %d: %w", h, err)
		}
		if err := wire.ValidateBlockPayload(payload, id); err != nil {
			return fmt.Errorf("height %d payload: %w", h, err)
		}
		if err := CheckBlockPayload(payload, id, h, net); err != nil {
			return fmt.Errorf("height %d payload check: %w", h, err)
		}
		hdr, err := wire.BlockHeaderFromPayload(payload)
		if err != nil {
			return fmt.Errorf("height %d header: %w", h, err)
		}
		if err := ConnectBlockRaw(payload, hdr, h, net, chainView, index, j); err != nil {
			return fmt.Errorf("height %d connect: %w", h, err)
		}
	}
	return nil
}

// storedBodiesThrough reports whether raw block files exist for heights [0, height).
func storedBodiesThrough(j *store.HeaderJournal, raw *store.RawBlockStore, height int64) bool {
	var frontier int64 = -1
	return extendStoredBodiesFrontier(j, raw, &frontier, height-1)
}

// extendStoredBodiesFrontier advances *frontier through want inclusive when ancestors exist.
func extendStoredBodiesFrontier(j *store.HeaderJournal, raw *store.RawBlockStore, frontier *int64, want int64) bool {
	if j == nil || raw == nil || frontier == nil {
		return false
	}
	if want < 0 {
		return true
	}
	for h := *frontier + 1; h <= want; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return false
		}
		if !raw.Has(pow.BlockHashLE(h80)) {
			return false
		}
		*frontier = h
	}
	return *frontier >= want
}
