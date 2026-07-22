// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"strings"

	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
)

// execGetMiningInfo returns a Core-shaped subset (DogeGo estimates difficulty-based hashrate from tip header).
func execGetMiningInfo(j HeaderJournal, pool *mempool.Pool, txIndex *store.TxIndex, rawBlocks *store.RawBlockStore, chainName string, paths *DataPaths, blockMaxWeight int) (map[string]interface{}, int, string) {
	sync := computeChainIBDState(j, chainName, rawBlocks, paths)
	blocks := sync.blocks
	h80, err := j.ReadHeaderAt(blocks)
	if err != nil {
		return nil, -1, err.Error()
	}
	bitsU := binary.LittleEndian.Uint32(h80[72:76])
	diff, err := pow.DifficultyFromCompact(bitsU)
	if err != nil {
		return nil, -1, err.Error()
	}
	const targetSpacing = 60.0
	networkHashPS := diff * float64(1<<32) / targetSpacing
	if v, code, _ := execGetNetworkHashPS(j, rawBlocks, paths, networkFromChainName(chainName), nil); code == 0 {
		if f, ok := v.(float64); ok && f > 0 {
			networkHashPS = f
		}
	}
	pooled := 0
	if pool != nil {
		pooled = pool.Count()
	}
	cw := "0"
	if s, err := cumulativeChainworkHex(j, blocks); err == nil {
		cw = s
	}
	cn := strings.ToLower(strings.TrimSpace(chainName))
	testnet := cn == "test" || cn == "testnet"
	net := networkFromChainName(chainName)
	chainWarns := consensus.ChainWarnings(j, net)
	errStr := ""
	if len(chainWarns) > 0 {
		errStr = strings.Join(chainWarns, "; ")
	}
	networkActive := true
	if paths != nil && paths.NetworkActive != nil {
		networkActive = paths.NetworkActive()
	}
	weightLimit := blockMaxWeight
	if weightLimit <= 0 {
		weightLimit = consensus.MaxBlockWeight
	}
	view := consensus.AdmissionPrevOutView(pool, txIndex, rawBlocks)
	sel, _ := consensus.SelectBlockTemplateTxs(pool, view, weightLimit)
	blockTx := len(sel.Txs)
	blockWeight := sel.TotalWeight
	blockSize := 0
	for _, tx := range sel.Txs {
		blockSize += len(tx.Data) / 2
	}
	return map[string]interface{}{
		"blocks":                 blocks,
		"headers":                sync.headers,
		"initialblockdownload":   sync.ibd,
		"difficulty":             diff,
		"networkhashps":        networkHashPS,
		"pooledtx":             pooled,
		"chain":                chainName,
		"chainwork":            cw,
		"errors":               errStr,
		"testnet":              testnet,
		"networkactive":        networkActive,
		"currentblocksize":     blockSize,
		"currentblockweight":   blockWeight,
		"currentblocktx":       blockTx,
		"weightlimit":          weightLimit,
		"dogego_note":          "networkhashps matches getnetworkhashps when the header window yields a positive rate; else tip-difficulty heuristic (60s target); not stratum pool hashrate",
		"dogego_warnings":      chainWarns,
		"dogego_mining_stubs": "currentblock* reflect getblocktemplate mempool selection (coinbase not included in tx count); use generate/generatetoaddress or createauxblock for block production",
	}, 0, ""
}
