// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "dogego/store"

// execGetTxOutSetInfo implements gettxoutsetinfo (Dogecoin Core blockchain.cpp) for JSON shape compatibility.
// When utxo is synced through the active chain tip, txouts/total_amount reflect the in-memory cache.
func execGetTxOutSetInfo(j HeaderJournal, raw *store.RawBlockStore, utxo *store.UtxoCache, paths *DataPaths, syncUtxo func() error) (map[string]interface{}, int, string) {
	if j == nil {
		return nil, -1, "gettxoutsetinfo: no chain state"
	}
	if syncUtxo != nil {
		_ = syncUtxo()
	}
	headerTip, _, _ := activeChainFromJournal(j, raw, paths)
	utxoTip := int64(-1)
	if utxo != nil {
		utxoTip = utxo.TipHeight()
	}
	reportTip := headerTip
	if utxoTip >= 0 {
		reportTip = utxoTip
	}
	best, err := blockHashHexAt(j, reportTip)
	if err != nil {
		best = ""
	}
	txouts := int64(0)
	bytesSer := int64(0)
	totalKoinu := int64(0)
	hashSer := "0000000000000000000000000000000000000000000000000000000000000000"
	note := "UTXO set unavailable: run a synced full node with txindex to populate gettxoutsetinfo."
	if utxo != nil && utxoTip >= 0 {
		n, k, b := utxo.Stats()
		txouts = int64(n)
		bytesSer = b
		totalKoinu = k
		if hj, ok := j.(*store.HeaderJournal); ok {
			hashSer = utxo.SerializedHashAtTip(hj)
		} else {
			hashSer = utxo.SerializedHash()
		}
		note = "Counts from in-memory UTXO cache at chainActive (hash_serialized uses Core GetUTXOStats algorithm)."
		if paths != nil && paths.ContiguousRawHeight != nil {
			if cont := paths.ContiguousRawHeight(); cont > utxoTip {
				note += "; connect catch-up in progress (stored bodies ahead of chainActive)."
			}
		}
	}
	return map[string]interface{}{
		"height":           reportTip,
		"bestblock":        best,
		"transactions":     txouts,
		"txouts":           txouts,
		"bytes_serialized": bytesSer,
		"hash_serialized":  hashSer,
		"total_amount":     float64(totalKoinu) / 1e8,
		"dogego_utxo_note": note,
	}, 0, ""
}
