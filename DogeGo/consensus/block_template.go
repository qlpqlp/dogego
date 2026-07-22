// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"slices"

	"dogego/mempool"
	"dogego/wire"
)

// Coinbase template reservations (Core BlockAssembler::resetBlock).
const (
	coinbaseTemplateWeight  = 4000
	coinbaseTemplateSigops  = int64(400)
)

// BlockTemplateTx is one non-coinbase transaction for BIP22 getblocktemplate.
type BlockTemplateTx struct {
	TxID    string
	Hash    string
	Data    string
	Fee     int64
	SigOps  int64
	Weight  int
	Depends []int
}

// BlockTemplateSelection is the result of mempool block assembly.
type BlockTemplateSelection struct {
	Txs         []BlockTemplateTx
	TotalFees   int64
	TotalWeight int
	TotalSigOps int64
}

type templateCandidate struct {
	id   string
	rate int64
}

// SelectBlockTemplateTxs chooses mempool transactions for a block template (ancestor-feerate greedy, Core-shaped).
func SelectBlockTemplateTxs(pool *mempool.Pool, view PrevOutView, maxBlockWeight int) (BlockTemplateSelection, error) {
	var out BlockTemplateSelection
	if pool == nil {
		return out, nil
	}
	if maxBlockWeight <= 0 {
		maxBlockWeight = MaxBlockWeight
	}
	if maxBlockWeight <= coinbaseTemplateWeight {
		return out, nil
	}
	fullView := MultiPrevOutView{&MempoolPrevOutView{Pool: pool}, view}
	fees, sizes := MempoolEvictionMaps(pool, fullView)
	if len(fees) == 0 {
		return out, nil
	}
	txs, parents, _, err := pool.SpendEdges()
	if err != nil {
		return out, err
	}
	cands := make([]templateCandidate, 0, len(txs))
	for id := range txs {
		st, err := pool.PackageStatsForTxID(id, fees, sizes)
		if err != nil || st.AncestorSize <= 0 {
			continue
		}
		ancFees := pool.MiningAncestorFeesKoinu(st, id)
		rate := ancFees * 1000 / int64(st.AncestorSize)
		cands = append(cands, templateCandidate{id: id, rate: rate})
	}
	slices.SortFunc(cands, func(a, b templateCandidate) int {
		if a.rate != b.rate {
			if a.rate > b.rate {
				return -1
			}
			return 1
		}
		return cmpString(a.id, b.id)
	})

	inBlock := make(map[string]bool)
	vtxIndex := make(map[string]int) // coinbase is 0; first selected tx is 1
	curWeight := coinbaseTemplateWeight
	curSigOps := coinbaseTemplateSigops
	intra := &blockUndoView{}
	chainView := MultiPrevOutView{intra, fullView}
	var selected []string
	var selectedRaw [][]byte

	for {
		added := false
		for _, c := range cands {
			if inBlock[c.id] {
				continue
			}
			if !mempoolParentsInBlock(c.id, parents, txs, inBlock) {
				continue
			}
			tx := txs[c.id]
			raw, err := pool.GetRawByTxID(c.id)
			if err != nil {
				continue
			}
			wt, err := TransactionWeight(tx)
			if err != nil {
				continue
			}
			sigCost := GetTransactionSigOpCost(tx, chainView)
			if curWeight+wt > maxBlockWeight {
				continue
			}
			if curSigOps+sigCost > MaxBlockSigopsCost {
				continue
			}
			inBlock[c.id] = true
			vtxIndex[c.id] = len(selected) + 1
			selected = append(selected, c.id)
			selectedRaw = append(selectedRaw, raw)
			curWeight += wt
			curSigOps += sigCost
			out.TotalFees += fees[c.id]
			out.TotalWeight = curWeight
			out.TotalSigOps = curSigOps
			intra.addTx(tx)
			added = true
		}
		if !added {
			break
		}
	}

	out.Txs = make([]BlockTemplateTx, 0, len(selected))
	for i, id := range selected {
		tx := txs[id]
		var deps []int
		for i := range tx.Vin {
			in := &tx.Vin[i]
			if IsNullOutpoint(in) {
				continue
			}
			pid := txidDisplayFromLE(in.PrevHash)
			if idx, ok := vtxIndex[pid]; ok {
				deps = append(deps, idx)
			}
		}
		slices.Sort(deps)
		wt, _ := TransactionWeight(tx)
		wth := tx.WTxHash()
		out.Txs = append(out.Txs, BlockTemplateTx{
			TxID:    id,
			Hash:    txidDisplayFromLE(wth),
			Data:    hex.EncodeToString(selectedRaw[i]),
			Fee:     fees[id],
			SigOps:  GetTransactionSigOpCost(tx, chainView) / WitnessScaleFactor,
			Weight:  wt,
			Depends: deps,
		})
	}
	return out, nil
}

func mempoolParentsInBlock(txid string, parents map[string][]string, txs map[string]*wire.Tx, inBlock map[string]bool) bool {
	for _, pid := range parents[txid] {
		if _, inMempool := txs[pid]; inMempool && !inBlock[pid] {
			return false
		}
	}
	return true
}

func cmpString(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
