// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"fmt"
	"strings"

	"dogego/pow"
	"dogego/wire"
)

var errSpendFound = errors.New("spend found")

// HeaderChain is the header journal surface needed for on-chain spend scans.
type HeaderChain interface {
	TipHeight() (int64, error)
	ReadHeaderAt(height int64) ([]byte, error)
	HeightByDisplayHash(displayHex string) (int64, error)
}

// ChainSpendView detects whether an outpoint is already spent in stored blocks.
type ChainSpendView struct {
	Journal HeaderChain
	Raw     RawBlockGetter
	Index   TxIndexer
}

// NewChainSpendView returns a chain spend scanner, or nil if required stores are missing.
func NewChainSpendView(journal HeaderChain, raw RawBlockGetter, index TxIndexer) *ChainSpendView {
	if journal == nil || raw == nil || index == nil {
		return nil
	}
	return &ChainSpendView{Journal: journal, Raw: raw, Index: index}
}

// OutpointSpent reports whether prevHash:vout is spent in a block at or after the funding tx.
// Returns an error when raw block coverage is insufficient to decide (same rule as gettxout).
func (c *ChainSpendView) OutpointSpent(prevHash [32]byte, vout uint32) (bool, error) {
	if c == nil {
		return false, nil
	}
	txid := txidDisplayFromLE(prevHash)
	blockHash, pos, err := c.Index.Lookup(txid)
	if err != nil {
		return false, nil // funding tx not indexed - not a confirmed spend we track
	}
	blockHex := pow.LEUint256DisplayHex(blockHash[:])
	coinH, err := c.Journal.HeightByDisplayHash(blockHex)
	if err != nil {
		return false, fmt.Errorf("spend scan: block height: %w", err)
	}
	return OutpointSpentInBlocks(c.Journal, c.Raw, coinH, int64(pos), txid, vout)
}

// OutpointSpentInBlocks scans rawblocks from coinHeight through tip for a spend of rpcTxid:vout.
func OutpointSpentInBlocks(journal HeaderChain, raw RawBlockGetter, coinHeight, fundingTxIndex int64, rpcTxid string, vout uint32) (bool, error) {
	if raw == nil || journal == nil {
		return false, fmt.Errorf("spend scan: chain store unavailable")
	}
	tip, err := journal.TipHeight()
	if err != nil {
		return false, err
	}
	rpcTxid = strings.TrimSpace(strings.ToLower(rpcTxid))
	for h := coinHeight; h <= tip; h++ {
		h80, err := journal.ReadHeaderAt(h)
		if err != nil {
			return false, err
		}
		payload, err := raw.Get(pow.BlockHashLE(h80))
		if err != nil {
			return false, fmt.Errorf("spend scan: missing raw block at height %d (need coverage from %d through %d)", h, coinHeight, tip)
		}
		txStart := uint32(0)
		if h == coinHeight {
			txStart = uint32(fundingTxIndex + 1)
		}
		var found bool
		scanErr := wire.ForEachBlockTx(payload, func(ti uint32, tx *wire.Tx) error {
			if ti < txStart {
				return nil
			}
			for vi := range tx.Vin {
				in := &tx.Vin[vi]
				if IsNullOutpoint(in) {
					continue
				}
				if in.PrevIdx != vout {
					continue
				}
				if txidDisplayFromLE(in.PrevHash) == rpcTxid {
					found = true
					return errSpendFound
				}
			}
			return nil
		})
		if found || errors.Is(scanErr, errSpendFound) {
			return true, nil
		}
		if scanErr != nil {
			return false, fmt.Errorf("spend scan: corrupt block at height %d: %w", h, scanErr)
		}
	}
	return false, nil
}
