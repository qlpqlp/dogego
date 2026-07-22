// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"fmt"
	"sort"

	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func isCoinbaseTx(tx *wire.Tx) bool {
	return len(tx.Vin) == 1 && isCoinbaseInput(&tx.Vin[0])
}

func truncatedMedianInt64(vals []int64) int64 {
	n := len(vals)
	if n == 0 {
		return 0
	}
	s := append([]int64(nil), vals...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	if n%2 == 0 {
		return (s[n/2-1] + s[n/2]) / 2
	}
	return s[n/2]
}

// execGetBlockStats returns Core-shaped per-block statistics from the raw block store.
// Fee-related fields populate when prevouts resolve via UTXO cache (tip at parent height) or tx index + raw blocks.
func execGetBlockStats(j HeaderJournal, raw *store.RawBlockStore, txIndex *store.TxIndex, utxo *store.UtxoCache, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if raw == nil {
		return nil, -18, "getblockstats: raw block store not available"
	}
	hashLE, height, err := resolveBlockLocation(j, params)
	if err != nil {
		return nil, -8, err.Error()
	}
	var statNames []string
	filter := false
	if len(params) > 1 && string(params[1]) != "null" {
		if err := json.Unmarshal(params[1], &statNames); err != nil {
			return nil, -8, "getblockstats: stats must be a JSON array of strings"
		}
		filter = len(statNames) > 0
	}

	payload, err := raw.Get(hashLE)
	if err != nil {
		return nil, -5, "getblockstats: block not stored on disk (headers-only or outside fetched range)"
	}
	if err := wire.ValidateBlockPayload(payload, hashLE); err != nil {
		return nil, -8, "getblockstats: invalid stored block: " + err.Error()
	}
	hdr, err := wire.BlockHeaderFromPayload(payload)
	if err != nil {
		return nil, -8, "getblockstats: corrupt stored block: " + err.Error()
	}
	nTx, err := wire.BlockTxCount(payload)
	if err != nil {
		return nil, -8, "getblockstats: corrupt stored block: " + err.Error()
	}

	pol := consensus.DefaultStandardPolicy()
	dustRelay := consensus.MinRelayTxFeePerKB()
	if paths != nil && paths.Standard != nil {
		pol = paths.Standard()
	}
	view := blockStatsPrevOutView(height-1, utxo, txIndex, raw)
	feesResolvedView := view != nil

	var (
		outputs      int64
		inputs       int64
		totalOut     int64
		totalSize    int64
		totalW       int64
		maxTxSize    int64
		minTxSize    int64 = -1
		txSizes      []int64
		minOutAmt    int64
		maxOutAmt    int64
		haveNonCbOut bool
		coinbaseVoutSum int64
		dustOuts     int64
	)

	if err := wire.ForEachBlockTx(payload, func(i uint32, tx *wire.Tx) error {
		outputs += int64(len(tx.Vout))
		cb := isCoinbaseTx(tx)
		txOutSum := int64(0)
		for vi := range tx.Vout {
			o := &tx.Vout[vi]
			txOutSum += o.Value
			if cb {
				if i == 0 {
					coinbaseVoutSum += o.Value
				}
				continue
			}
			if !haveNonCbOut {
				minOutAmt, maxOutAmt = o.Value, o.Value
				haveNonCbOut = true
			} else {
				if o.Value < minOutAmt {
					minOutAmt = o.Value
				}
				if o.Value > maxOutAmt {
					maxOutAmt = o.Value
				}
			}
			if !cb && consensus.IsOutputDustEffective(*o, pol, dustRelay) {
				dustOuts++
			}
		}
		if cb {
			return nil
		}
		inputs += int64(len(tx.Vin))
		totalOut += txOutSum
		ser, err := tx.Serialize()
		if err != nil {
			return err
		}
		txSz := int64(len(ser))
		totalSize += txSz
		totalW += txSz * 4
		if txSz > maxTxSize {
			maxTxSize = txSz
		}
		if minTxSize < 0 || txSz < minTxSize {
			minTxSize = txSz
		}
		txSizes = append(txSizes, txSz)
		return nil
	}); err != nil {
		return nil, -8, "getblockstats: " + err.Error()
	}

	numNonCb := int(nTx) - 1
	if numNonCb < 0 {
		numNonCb = 0
	}
	if minTxSize < 0 {
		minTxSize = 0
	}

	totalfee := int64(0)
	feesResolved := false
	var minFee, maxFee, medianFee, minFeerate, maxFeerate, avgFeerate int64
	feePerc := []interface{}{int64(0), int64(0), int64(0), int64(0), int64(0)}
	utxoInc := int64(0)
	utxoIncOK := false
	if feesResolvedView {
		if st, ok := consensus.ComputeBlockFeeStatsRaw(payload, view); ok {
			feesResolved = true
			totalfee = st.TotalFee
			minFee, maxFee, medianFee = st.MinFee, st.MaxFee, st.MedianFee
			minFeerate, maxFeerate, avgFeerate = st.MinFeerate, st.MaxFeerate, st.AvgFeerate
			for i, p := range st.FeeratePercentiles {
				feePerc[i] = int64(p)
			}
		}
		if inc, ok := consensus.BlockUtxoSizeIncreaseRaw(payload, view); ok {
			utxoInc = inc
			utxoIncOK = true
		}
	}
	subsidy := coinbaseVoutSum - totalfee
	if subsidy < 0 {
		subsidy = 0
	}

	var avgFee, avgTxSize int64
	if numNonCb > 0 {
		avgFee = totalfee / int64(numNonCb)
		avgTxSize = totalSize / int64(numNonCb)
	}
	if !feesResolved && totalSize > 0 {
		avgFeerate = totalfee / totalSize
	}

	minOutFinal, maxOutFinal := int64(0), int64(0)
	if haveNonCbOut {
		minOutFinal, maxOutFinal = minOutAmt, maxOutAmt
	}

	mt := int64(hdr.Timestamp)
	if m, err := headerMedianTimePast(j, height); err == nil {
		mt = m
	}

	h80 := hdr.EncodeWire80()

	all := map[string]interface{}{
		"avgfee":              avgFee,
		"avgfeerate":          avgFeerate,
		"avgtxsize":           avgTxSize,
		"blockhash":           pow.BlockHashHex(h80[:]),
		"dustouts":            dustOuts,
		"feerate_percentiles": feePerc,
		"height":              height,
		"ins":                 inputs,
		"maxfee":              maxFee,
		"maxfeerate":          maxFeerate,
		"maxoutamount":        maxOutFinal,
		"maxtxsize":           maxTxSize,
		"medianfee":           medianFee,
		"mediantime":          mt,
		"mediantxsize":        truncatedMedianInt64(txSizes),
		"minfee":              minFee,
		"minfeerate":          minFeerate,
		"minoutamount":        minOutFinal,
		"mintxsize":           minTxSize,
		"outs":                outputs,
		"subsidy":             subsidy,
		"time":                int64(hdr.Timestamp),
		"total_out":           totalOut,
		"total_size":          totalSize,
		"total_weight":        totalW,
		"totalfee":            totalfee,
		"txs":                 int64(nTx),
		"utxo_increase":       outputs - inputs,
		"utxo_size_inc":       utxoInc,
	}
	if !filter {
		note := "Transaction weight uses 4 * serialized size (legacy non-witness approximation)."
		if feesResolved {
			note += " Fee fields use resolved prevouts (UTXO cache at parent height and/or tx index + raw blocks)."
		} else {
			note += " totalfee, fee rates, medianfee, and feerate_percentiles are zero when prevouts cannot be resolved; subsidy equals coinbase output sum."
		}
		note += " dustouts uses configured standardness/dust relay policy."
		if !utxoIncOK {
			note += " utxo_size_inc is zero when parent prevouts cannot be resolved."
		} else {
			note += " utxo_size_inc is an approximate net output-script delta (not Core LevelDB undo size)."
		}
		all["dogego_note"] = note
	}

	if !filter {
		return all, 0, ""
	}

	out := make(map[string]interface{}, len(statNames))
	for _, name := range statNames {
		v, ok := all[name]
		if !ok {
			return nil, -8, fmt.Sprintf("Invalid selected statistic %s", name)
		}
		out[name] = v
	}
	return out, 0, ""
}
