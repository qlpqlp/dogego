// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"fmt"
	"math"

	"dogego/applog"
	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
)

// defaultVerifyChainBlocks mirrors Core's typical -checkblocks default when RPC omits nblocks.
const defaultVerifyChainBlocks = 6

// execVerifyChain checks stored header linkage (and PoW when chain params do not use RelaxedPoW).
// checklevel 3+: contextual header validation (Digishield, auxpow when headers_aux.bin present).
// checklevel 4: also ConnectBlock on stored raw blocks in range (native rawblocks + txindex, or Core blk/chainstate).
// Optional params: (checklevel 0..4, nblocks >= 0; nblocks 0 = entire chain; verbose bool returns RPC error on failure).
// Returns Core-shaped boolean when verbose is false.
func execVerifyChain(chainName string, j HeaderJournal, aux *store.HeaderAuxJournal, raw *store.RawBlockStore, txIndex *store.TxIndex, paths *DataPaths, utxo *store.UtxoCache, params []json.RawMessage) (interface{}, int, string) {
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}

	checkLevel := 3
	nBlocks := defaultVerifyChainBlocks
	verbose := false
	if len(params) > 0 {
		var v float64
		if err := json.Unmarshal(params[0], &v); err != nil || v < 0 || v > 4 || v != float64(int64(v)) {
			return nil, -8, "verifychain: checklevel must be integer 0..4"
		}
		checkLevel = int(v)
	}
	if len(params) > 1 {
		var v float64
		if err := json.Unmarshal(params[1], &v); err != nil || v < 0 || v > float64(math.MaxInt64) || v != float64(int64(v)) {
			return nil, -8, "verifychain: nblocks must be integer >= 0"
		}
		nBlocks = int(v)
	}
	if len(params) > 2 {
		if err := json.Unmarshal(params[2], &verbose); err != nil {
			return nil, -8, "verifychain: verbose must be boolean"
		}
	}
	chainTip, _, _ := activeChainFromJournal(j, raw, paths)
	n := chainTip + 1
	if n < 1 {
		return false, 0, ""
	}

	var start int64
	depth := n
	if nBlocks > 0 {
		d := int64(nBlocks)
		if d > n {
			d = n
		}
		depth = d
		start = n - depth
	}
	idx := verifyChainTxIndexer(txIndex, paths)
	if checkLevel >= 4 {
		if raw == nil {
			return nil, -8, "verifychain: level 4 requires raw block store (stored block bodies)"
		}
		if idx == nil {
			return nil, -8, "verifychain: level 4 requires tx index (enable -txindex or Core -txindex)"
		}
	}

	hs := make([][]byte, 0, depth)
	for h := start; h < n; h++ {
		row, err := j.ReadHeaderAt(h)
		if err != nil {
			return nil, -8, "verifychain: " + err.Error()
		}
		if len(row) != 80 {
			return nil, -8, fmt.Sprintf("verifychain: header at height %d: want 80 bytes", h)
		}
		cp := make([]byte, 80)
		copy(cp, row)
		hs = append(hs, cp)
	}

	var prevTip [32]byte
	if start > 0 {
		prev80, err := j.ReadHeaderAt(start - 1)
		if err != nil {
			return nil, -8, "verifychain: " + err.Error()
		}
		if len(prev80) != 80 {
			return nil, -8, fmt.Sprintf("verifychain: header at height %d: want 80 bytes", start-1)
		}
		prevTip = pow.BlockHashLE(prev80)
	}

	endHeight := start + int64(len(hs)) - 1
	fail := func(err error) (interface{}, int, string) {
		if err == nil {
			return false, 0, ""
		}
		if verbose {
			return nil, -8, "verifychain: " + err.Error()
		}
		applog.Line("verifychain", err.Error())
		return false, 0, ""
	}
	if checkLevel >= 3 {
		if sj, ok := j.(*store.HeaderJournal); ok {
			needsAux, err := consensus.StoredHeaderRangeNeedsAux(sj, start, endHeight)
			if err != nil {
				return fail(err)
			}
			if needsAux && aux == nil {
				return fail(fmt.Errorf("level %d requires headers_aux.bin (auxpow headers in range %d..%d)", checkLevel, start, endHeight))
			}
			if err := consensus.ValidateStoredHeaders(sj, aux, p, start, endHeight, rpcNetworkNowUnix(paths)); err != nil {
				return fail(err)
			}
			if checkLevel >= 4 {
				utxoSrc := verifyChainUtxoSource(paths, utxo, endHeight)
				if err := consensus.ValidateStoredBlockBodies(sj, raw, idx, utxoSrc, p.Net, start, endHeight); err != nil {
					return fail(err)
				}
			}
			return true, 0, ""
		}
	}
	validateP := verifyChainParamsForLevel(p, checkLevel, false)
	if err := consensus.ValidateHeaderChain(validateP, prevTip, hs); err != nil {
		return fail(err)
	}
	return true, 0, ""
}

func verifyChainParamsForLevel(p chain.Params, checkLevel int, storedJournal bool) chain.Params {
	if !storedJournal || checkLevel < 3 {
		p.RelaxedPoW = true
	}
	return p
}

func verifyChainTxIndexer(txIndex *store.TxIndex, paths *DataPaths) consensus.TxIndexer {
	if txIndex != nil {
		return txIndex
	}
	return nil
}

func verifyChainUtxoSource(paths *DataPaths, utxo *store.UtxoCache, endHeight int64) consensus.UtxoOutpointSource {
	if utxo != nil && utxo.TipHeight() >= endHeight {
		return utxo
	}
	return nil
}
