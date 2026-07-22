// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"dogego/pow"
	"dogego/wire"
)

// MempoolConfirmSample is one mempool tx confirmed in a block (feerate + blocks waited).
type MempoolConfirmSample struct {
	TxID         string // RPC display txid (optional)
	FeeratePerKB uint64
	BlocksWaited int
}

// MempoolRawLookup returns raw tx bytes for an RPC display txid when the tx is still pooled.
type MempoolRawLookup interface {
	MempoolRawByDisplayTxid(rpcTxid string) ([]byte, bool)
}

// MempoolConfirmLookup extends raw lookup with confirmation delay (blocks waited).
type MempoolConfirmLookup interface {
	MempoolRawLookup
	BlocksWaitedAtConfirm(displayTxid string, confirmHeight int64) int
}

func collectMempoolConfirmedTx(i uint32, tx *wire.Tx, pool MempoolConfirmLookup, view PrevOutView, confirmHeight int64, out *[]MempoolConfirmSample) {
	if i == 0 || IsCoinbaseTx(tx) {
		return
	}
	th := tx.TxHash()
	id := pow.LEUint256DisplayHex(th[:])
	mraw, ok := pool.MempoolRawByDisplayTxid(id)
	if !ok {
		return
	}
	rate, ok := TxFeeRateKoinuPerKB(tx, mraw, view)
	if !ok || rate == 0 {
		return
	}
	*out = append(*out, MempoolConfirmSample{
		TxID:         id,
		FeeratePerKB: rate,
		BlocksWaited: pool.BlocksWaitedAtConfirm(id, confirmHeight),
	})
}

// CollectMempoolConfirmedSamplesRaw scans a serialized block without retaining all decoded txs.
func CollectMempoolConfirmedSamplesRaw(blockRaw []byte, pool MempoolConfirmLookup, view PrevOutView, confirmHeight int64) []MempoolConfirmSample {
	if pool == nil || view == nil || len(blockRaw) < 80 {
		return nil
	}
	var out []MempoolConfirmSample
	_ = wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
		collectMempoolConfirmedTx(i, tx, pool, view, confirmHeight, &out)
		return nil
	})
	return out
}

// CollectMempoolConfirmedSamples returns feerates for pooled txs in pb with blocks-to-confirm.
func CollectMempoolConfirmedSamples(pb *wire.ParsedBlock, pool MempoolConfirmLookup, view PrevOutView, confirmHeight int64) []MempoolConfirmSample {
	if pb == nil || pool == nil || view == nil {
		return nil
	}
	var out []MempoolConfirmSample
	for i, tx := range pb.Txs {
		collectMempoolConfirmedTx(uint32(i), tx, pool, view, confirmHeight, &out)
	}
	return out
}

// CollectMempoolConfirmedFeeRates returns feerates only (blocks-waited treated as 1 when height unknown).
func CollectMempoolConfirmedFeeRates(pb *wire.ParsedBlock, pool MempoolRawLookup, view PrevOutView) []uint64 {
	samples := collectSamplesAny(pb, pool, view, -1)
	rates := make([]uint64, 0, len(samples))
	for _, s := range samples {
		rates = append(rates, s.FeeratePerKB)
	}
	return rates
}

func collectSamplesAny(pb *wire.ParsedBlock, pool MempoolRawLookup, view PrevOutView, confirmHeight int64) []MempoolConfirmSample {
	if lookup, ok := pool.(MempoolConfirmLookup); ok {
		return CollectMempoolConfirmedSamples(pb, lookup, view, confirmHeight)
	}
	var out []MempoolConfirmSample
	for _, r := range collectRatesLegacy(pb, pool, view) {
		out = append(out, MempoolConfirmSample{FeeratePerKB: r, BlocksWaited: 1})
	}
	return out
}

func collectRatesLegacy(pb *wire.ParsedBlock, pool MempoolRawLookup, view PrevOutView) []uint64 {
	if pb == nil || pool == nil || view == nil {
		return nil
	}
	var rates []uint64
	for i, tx := range pb.Txs {
		if i == 0 || IsCoinbaseTx(tx) {
			continue
		}
		h := tx.TxHash()
		raw, ok := pool.MempoolRawByDisplayTxid(pow.LEUint256DisplayHex(h[:]))
		if !ok {
			continue
		}
		if rate, ok := TxFeeRateKoinuPerKB(tx, raw, view); ok && rate > 0 {
			rates = append(rates, rate)
		}
	}
	return rates
}
