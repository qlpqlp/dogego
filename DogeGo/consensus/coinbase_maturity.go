// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"fmt"

	"dogego/chain"
	"dogego/pow"
	"dogego/wire"
)

// ErrCoinbaseImmature is returned when a spend uses a coinbase output before maturity.
var ErrCoinbaseImmature = errors.New("consensus: bad-txns-premature-spend-of-coinbase")

// CoinbaseFundingHeight returns the block height of a confirmed coinbase funding tx, if any.
func CoinbaseFundingHeight(index TxIndexer, journal HeaderChain, prevHash [32]byte) (height int64, isCoinbase bool, err error) {
	if index == nil || journal == nil {
		return 0, false, nil
	}
	txid := txidDisplayFromLE(prevHash)
	blockHash, txIdx, err := index.Lookup(txid)
	if err != nil {
		return 0, false, nil
	}
	if txIdx != 0 {
		return 0, false, nil
	}
	blockHex := pow.LEUint256DisplayHex(blockHash[:])
	h, err := journal.HeightByDisplayHash(blockHex)
	if err != nil {
		return 0, false, fmt.Errorf("coinbase maturity: block height: %w", err)
	}
	return h, true, nil
}

// CheckTxCoinbaseMaturity rejects spends of immature coinbase outputs (Core ConnectBlock / mempool).
func CheckTxCoinbaseMaturity(tx *wire.Tx, spendHeight int64, net chain.Network, index TxIndexer, journal HeaderChain) error {
	if tx == nil || IsCoinbaseTx(tx) || index == nil || journal == nil {
		return nil
	}
	maturity := LookupConsensus(net, spendHeight).CoinbaseMaturity
	if maturity <= 0 {
		maturity = 30
	}
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if IsNullOutpoint(in) {
			continue
		}
		coinH, isCB, err := CoinbaseFundingHeight(index, journal, in.PrevHash)
		if err != nil {
			return fmt.Errorf("input %d: %w", i, err)
		}
		if !isCB {
			continue
		}
		if spendHeight-coinH < int64(maturity) {
			return fmt.Errorf("%w (input %d, need %d blocks, have %d)", ErrCoinbaseImmature, i, maturity, spendHeight-coinH)
		}
	}
	return nil
}
