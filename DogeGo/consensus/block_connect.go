// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"fmt"

	"dogego/chain"
	"dogego/primitives"
	"dogego/wire"
)

// blockUndoView tracks outputs created by earlier transactions in the block being connected.
type blockUndoView struct {
	outs map[[36]byte]PrevOut
}

type blockTxWalker func(fn func(uint32, *wire.Tx) error) error

func outpointKey(hash [32]byte, idx uint32) [36]byte {
	var k [36]byte
	copy(k[:32], hash[:])
	binary.LittleEndian.PutUint32(k[32:], idx)
	return k
}

func (b *blockUndoView) Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool) {
	if b == nil || b.outs == nil {
		return PrevOut{}, false
	}
	o, ok := b.outs[outpointKey(prevHash, idx)]
	return o, ok
}

func (b *blockUndoView) addTx(tx *wire.Tx, height int64) {
	if b.outs == nil {
		b.outs = make(map[[36]byte]PrevOut)
	}
	h := tx.TxHash()
	coinbase := IsCoinbaseTx(tx)
	for i, o := range tx.Vout {
		b.outs[outpointKey(h, uint32(i))] = PrevOut{
			Value:    o.Value,
			PkScript: append([]byte(nil), o.PkScript...),
			Height:   height,
			Coinbase: coinbase,
		}
	}
}

// CheckBlockDuplicateTxidsRaw rejects duplicate transaction ids in a serialized block payload.
func CheckBlockDuplicateTxidsRaw(blockRaw []byte) error {
	if len(blockRaw) < 80 {
		return fmt.Errorf("bad-blk-length")
	}
	seen := make(map[[32]byte]struct{})
	var n int
	err := wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
		h := tx.TxHash()
		if _, ok := seen[h]; ok {
			return fmt.Errorf("bad-txns-duplicate: tx %d", i)
		}
		seen[h] = struct{}{}
		n++
		return nil
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("bad-blk-length")
	}
	return nil
}

// CheckBlockDuplicateSpendsRaw rejects duplicate intra-block spends in a serialized block payload.
func CheckBlockDuplicateSpendsRaw(blockRaw []byte) error {
	if len(blockRaw) < 80 {
		return fmt.Errorf("bad-blk-null")
	}
	seen := make(map[[36]byte]struct{})
	return wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
		if i == 0 || IsCoinbaseTx(tx) {
			return nil
		}
		for _, in := range tx.Vin {
			k := outpointKey(in.PrevHash, in.PrevIdx)
			if _, ok := seen[k]; ok {
				return fmt.Errorf("bad-txns-spent-after-spent")
			}
			seen[k] = struct{}{}
		}
		return nil
	})
}

// CheckBlockDuplicateTxids rejects duplicate transaction ids within a block (Core CheckBlock).
func CheckBlockDuplicateTxids(pb *wire.ParsedBlock) error {
	if pb == nil {
		return fmt.Errorf("bad-blk-null")
	}
	seen := make(map[[32]byte]struct{}, len(pb.Txs))
	for i, tx := range pb.Txs {
		h := tx.TxHash()
		if _, ok := seen[h]; ok {
			return fmt.Errorf("bad-txns-duplicate: tx %d", i)
		}
		seen[h] = struct{}{}
	}
	return nil
}

// CheckBlockDuplicateSpends rejects two non-coinbase txs spending the same outpoint in one block.
func CheckBlockDuplicateSpends(pb *wire.ParsedBlock) error {
	if pb == nil {
		return fmt.Errorf("bad-blk-null")
	}
	seen := make(map[[36]byte]struct{})
	for i, tx := range pb.Txs {
		if i == 0 || IsCoinbaseTx(tx) {
			continue
		}
		for _, in := range tx.Vin {
			k := outpointKey(in.PrevHash, in.PrevIdx)
			if _, ok := seen[k]; ok {
				return fmt.Errorf("bad-txns-spent-after-spent")
			}
			seen[k] = struct{}{}
		}
	}
	return nil
}

// ConnectBlock validates non-coinbase transactions (scripts, fees, coinbase amount) when chainView
// can resolve prevouts from the UTXO cache (Core ConnectBlock; txindex is not used here).
func ConnectBlock(pb *wire.ParsedBlock, height int64, net chain.Network, chainView PrevOutView, index TxIndexer, journal HeaderChain) error {
	if pb == nil || chainView == nil {
		return nil
	}
	if err := CheckBlockDuplicateSpends(pb); err != nil {
		return err
	}
	if err := CheckBlockDuplicateTxids(pb); err != nil {
		return err
	}
	return connectBlockInternal(pb.Header, func(walk func(uint32, *wire.Tx) error) error {
		for i, tx := range pb.Txs {
			if err := walk(uint32(i), tx); err != nil {
				return err
			}
		}
		return nil
	}, func() error {
		return CheckBlockSigOpCost(pb, chainView)
	}, height, net, chainView, index, journal)
}

// ConnectBlockRaw is ConnectBlock on serialized block bytes without retaining all decoded txs.
func ConnectBlockRaw(blockRaw []byte, hdr primitives.BlockHeader, height int64, net chain.Network, chainView PrevOutView, index TxIndexer, journal HeaderChain) error {
	if len(blockRaw) < 80 || chainView == nil {
		return nil
	}
	return connectBlockInternal(hdr, func(walk func(uint32, *wire.Tx) error) error {
		return wire.ForEachBlockTx(blockRaw, walk)
	}, func() error {
		return CheckBlockSigOpCostRaw(blockRaw, chainView)
	}, height, net, chainView, index, journal)
}

func connectBlockInternal(hdr primitives.BlockHeader, walk blockTxWalker, sigOpCheck func() error, height int64, net chain.Network, chainView PrevOutView, index TxIndexer, journal HeaderChain) error {
	var ltCtx LockTimeContext
	if journal != nil {
		var err error
		ltCtx, err = LockTimeContextAtBlock(journal, height, true)
		if err != nil {
			return err
		}
	}

	intra := &blockUndoView{}
	view := MultiPrevOutView{intra, chainView}
	scriptChecks := ScriptChecksEnabledAtHeight(height)
	if scriptChecks && sigOpCheck != nil {
		if err := sigOpCheck(); err != nil {
			return err
		}
	}

	sameBlock := make(map[[32]byte]struct{})
	var fees, cbOut int64
	var nTx int
	dupSpend := make(map[[36]byte]struct{})
	seenTxid := make(map[[32]byte]struct{})

	err := walk(func(i uint32, tx *wire.Tx) error {
		if journal != nil {
			if err := CheckTxFinal(tx, ltCtx); err != nil {
				return fmt.Errorf("tx %d: %w", i, err)
			}
		}
		nTx++
		h := tx.TxHash()
		if _, ok := seenTxid[h]; ok {
			return fmt.Errorf("bad-txns-duplicate: tx %d", i)
		}
		seenTxid[h] = struct{}{}

		if i == 0 || IsCoinbaseTx(tx) {
			if i > 0 {
				return fmt.Errorf("bad-cb-multiple")
			}
			for _, o := range tx.Vout {
				cbOut += o.Value
			}
			intra.addTx(tx, height)
			sameBlock[h] = struct{}{}
			return nil
		}
		for _, in := range tx.Vin {
			k := outpointKey(in.PrevHash, in.PrevIdx)
			if _, ok := dupSpend[k]; ok {
				return fmt.Errorf("bad-txns-spent-after-spent")
			}
			dupSpend[k] = struct{}{}
		}
		if journal != nil && EnforceBIP68Sequence(tx) {
			prevH, err := PrevHeightsForTx(tx, nil, journal, height, sameBlock, 0, chainView)
			if err != nil {
				return fmt.Errorf("tx %d prev heights: %w", i, err)
			}
			if err := CheckTxSequenceLocks(tx, SequenceEvalBlock{Height: height}, prevH, journal, net); err != nil {
				return fmt.Errorf("tx %d: %w", i, err)
			}
		}
		if err := CheckTransaction(tx, true); err != nil {
			return fmt.Errorf("tx %d: %w", i, err)
		}
		if err := CheckTxInputsAtHeight(tx, view, height, net); err != nil {
			return fmt.Errorf("tx %d: %w", i, err)
		}
		if scriptChecks {
			if err := VerifyScriptAtHeight(tx, view, height, net, journal); err != nil {
				return fmt.Errorf("tx %d: %w", i, err)
			}
		}
		fee, err := TxFee(tx, view)
		if err != nil {
			return fmt.Errorf("tx %d: %w", i, err)
		}
		if fee < 0 {
			return fmt.Errorf("tx %d: bad-txns-in-belowout", i)
		}
		fees += fee
		intra.addTx(tx, height)
		sameBlock[h] = struct{}{}
		return nil
	})
	if err != nil {
		return err
	}
	if nTx == 0 {
		return fmt.Errorf("bad-blk-length")
	}
	return CheckBlockCoinbaseSubsidyRaw(hdr, height, net, cbOut, fees)
}

// CheckBlockCoinbaseSubsidyRaw enforces the Core ConnectBlock coinbase cap (subsidy + fees).
func CheckBlockCoinbaseSubsidyRaw(hdr primitives.BlockHeader, height int64, net chain.Network, cbOut, fees int64) error {
	subsidy := BlockSubsidy(height, hdr.PrevBlock, net)
	if cbOut > subsidy+fees {
		return fmt.Errorf("bad-cb-amount: out %d subsidy %d fees %d", cbOut, subsidy, fees)
	}
	return nil
}

// CheckBlockCoinbaseSubsidyPayload is CheckBlockCoinbaseSubsidyRaw on serialized block bytes.
// When chainView is nil, fees are treated as 0 (subsidy-only cap).
func CheckBlockCoinbaseSubsidyPayload(blockRaw []byte, height int64, net chain.Network, chainView PrevOutView) error {
	if len(blockRaw) < 80 {
		return fmt.Errorf("bad-blk-length")
	}
	hdr, err := wire.BlockHeaderFromPayload(blockRaw)
	if err != nil {
		return err
	}
	cb, _, err := wire.ReadTxAtIndex(blockRaw, 0)
	if err != nil {
		return fmt.Errorf("bad-blk-length: %w", err)
	}
	if !IsCoinbaseTx(cb) {
		return fmt.Errorf("bad-cb-missing")
	}
	var cbOut int64
	for _, o := range cb.Vout {
		cbOut += o.Value
	}
	fees := int64(0)
	if chainView != nil {
		err = wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
			if i == 0 || IsCoinbaseTx(tx) {
				return nil
			}
			fee, err := TxFee(tx, chainView)
			if err != nil {
				return err
			}
			if fee < 0 {
				return fmt.Errorf("tx %d: bad-txns-in-belowout", i)
			}
			fees += fee
			return nil
		})
		if err != nil {
			return err
		}
	}
	return CheckBlockCoinbaseSubsidyRaw(hdr, height, net, cbOut, fees)
}

// ConnectSparseCoinbaseBlockRaw runs the ConnectBlock coinbase subsidy path for coinbase-only
// blocks without ancestor raw blocks in the store (mainnet field evidence at sparse heights).
func ConnectSparseCoinbaseBlockRaw(blockRaw []byte, height int64, net chain.Network) error {
	if len(blockRaw) < 80 {
		return fmt.Errorf("bad-blk-length")
	}
	hdr, err := wire.BlockHeaderFromPayload(blockRaw)
	if err != nil {
		return err
	}
	var nTx int
	err = wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
		if i > 0 {
			return fmt.Errorf("sparse connect: unexpected tx %d (coinbase-only)", i)
		}
		nTx++
		return nil
	})
	if err != nil {
		return err
	}
	if nTx != 1 {
		return fmt.Errorf("sparse connect: want 1 tx got %d", nTx)
	}
	return ConnectBlockRaw(blockRaw, hdr, height, net, MultiPrevOutView{}, nil, nil)
}
