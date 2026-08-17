// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/pow"
	"dogego/wire"
)

// FundingTxHeight returns the block height that confirmed the funding transaction, if indexed.
func FundingTxHeight(index TxIndexer, journal HeaderChain, prevHash [32]byte) (height int64, ok bool, err error) {
	if index == nil || journal == nil {
		return 0, false, nil
	}
	txid := txidDisplayFromLE(prevHash)
	blockHash, _, err := index.Lookup(txid)
	if err != nil {
		return 0, false, nil
	}
	h, err := headerHeightForBlockHash(journal, blockHash)
	if err != nil {
		return 0, false, fmt.Errorf("funding height: %w", err)
	}
	return h, true, nil
}

type headerHeightByHashLE interface {
	HeightByBlockHashLE(hashLE [32]byte) (int64, error)
}

func headerHeightForBlockHash(journal HeaderChain, blockHashLE [32]byte) (int64, error) {
	if journal == nil {
		return -1, fmt.Errorf("nil journal")
	}
	if hb, ok := journal.(headerHeightByHashLE); ok {
		return hb.HeightByBlockHashLE(blockHashLE)
	}
	return journal.HeightByDisplayHash(pow.LEUint256DisplayHex(blockHashLE[:]))
}

// utxoHeightLookup resolves funding height from the connected UTXO set when txindex lags during IBD.
type utxoHeightLookup interface {
	UnspentHeight(prevHash [32]byte, vout uint32) (height int64, ok bool)
}

func utxoHeightFromView(view PrevOutView, prevHash [32]byte, vout uint32) (int64, bool) {
	if view == nil {
		return 0, false
	}
	if uh, ok := view.(utxoHeightLookup); ok {
		return uh.UnspentHeight(prevHash, vout)
	}
	if mv, ok := view.(MultiPrevOutView); ok {
		for _, sub := range mv {
			if h, ok := utxoHeightFromView(sub, prevHash, vout); ok {
				return h, true
			}
		}
	}
	return 0, false
}

// PrevHeightsResolvableForSequenceLocks reports whether every non-coinbase input has a
// confirmation height via txindex or UTXO view (mempool admission without txindex).
func PrevHeightsResolvableForSequenceLocks(tx *wire.Tx, index TxIndexer, view PrevOutView) bool {
	if tx == nil || IsCoinbaseTx(tx) {
		return false
	}
	if index != nil {
		return true
	}
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if IsNullOutpoint(in) {
			continue
		}
		if _, ok := utxoHeightFromView(view, in.PrevHash, in.PrevIdx); !ok {
			return false
		}
	}
	return true
}

// PrevHeightsForTx resolves the confirmation height of each input's prevout.
// sameBlock marks prev txs confirmed in the block at blockHeight (intra-block spends).
// unconfirmedHeight is used when the parent is not on chain (mempool: tip+1).
// chainView may supply UTXO confirmation heights when txindex is incomplete during connect catch-up.
func PrevHeightsForTx(tx *wire.Tx, index TxIndexer, journal HeaderChain, blockHeight int64, sameBlock map[[32]byte]struct{}, unconfirmedHeight int, chainView PrevOutView) ([]int, error) {
	if tx == nil {
		return nil, fmt.Errorf("nil tx")
	}
	out := make([]int, len(tx.Vin))
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if IsNullOutpoint(in) {
			out[i] = 0
			continue
		}
		if sameBlock != nil {
			if _, intra := sameBlock[in.PrevHash]; intra {
				out[i] = int(blockHeight)
				continue
			}
		}
		// Prefer UTXO confirmation height (O(1)). Txindex + journal HeightByDisplayHash
		// scans headers.bin under the journal lock; that stalled live mainnet connect at ~17k
		// with 6.3M headers on disk (Core CBlockIndex nHeight is O(1)).
		if h, ok := utxoHeightFromView(chainView, in.PrevHash, in.PrevIdx); ok {
			out[i] = int(h)
			continue
		}
		// ConnectBlock (unconfirmedHeight==0): Core uses AccessCoins()->nHeight only; no txindex.
		if unconfirmedHeight <= 0 {
			return nil, fmt.Errorf("input %d: missing funding height", i)
		}
		h, found, err := FundingTxHeight(index, journal, in.PrevHash)
		if err != nil {
			return nil, fmt.Errorf("input %d: %w", i, err)
		}
		if !found {
			if unconfirmedHeight <= 0 {
				return nil, fmt.Errorf("input %d: missing funding height", i)
			}
			out[i] = unconfirmedHeight
			continue
		}
		out[i] = int(h)
	}
	return out, nil
}
