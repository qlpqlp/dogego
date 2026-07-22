// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
)

// execGetBlockTemplate returns a BIP22-shaped template with chain context and mempool transaction selection.
func execGetBlockTemplate(j HeaderJournal, pool *mempool.Pool, txIndex *store.TxIndex, rawBlocks *store.RawBlockStore, paths *DataPaths, chainName string, blockMaxWeight int, params []json.RawMessage) (interface{}, int, string) {
	if j == nil {
		return nil, -1, "getblocktemplate: header journal not available"
	}
	if len(params) > 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	req := map[string]interface{}{}
	if len(params) == 1 && strings.TrimSpace(string(params[0])) != "null" {
		if err := json.Unmarshal(params[0], &req); err != nil || req == nil {
			return nil, -8, "getblocktemplate: JSON object or null expected for template request"
		}
	}
	mode := "template"
	if m, ok := req["mode"].(string); ok && m != "" {
		mode = m
	}
	if mode == "proposal" {
		if data, ok := req["data"].(string); ok && data != "" {
			return validateGBTProposal(j, pool, txIndex, rawBlocks, paths, chainName, blockMaxWeight, req, data), 0, ""
		}
		return nil, -8, "getblocktemplate: Missing data String key for proposal"
	}
	clientRules := gbtClientRules(req)
	if _, ok := clientRules["segwit"]; ok {
		return nil, -8, "getblocktemplate: segwit rules are disabled on Dogecoin (matches Core IsWitnessEnabled)"
	}

	// BIP22 longpoll: when longpollid matches the current tip+mempool state, wait until it changes.
	if clientLP, ok := req["longpollid"].(string); ok && strings.TrimSpace(clientLP) != "" {
		clientLP = strings.TrimSpace(clientLP)
		waitGBTLongpoll(gbtLongpollTimeout, func() bool {
			return gbtLongpollID(j, pool, rawBlocks, paths) == clientLP
		})
	} else if mode == "longpoll" {
		// mode=longpoll without id: wait one wake or short timeout then return a fresh template.
		waitGBTLongpoll(5*time.Second, func() bool { return true })
	}

	tip, _, _ := activeChainFromJournal(j, rawBlocks, paths)
	h80, err := j.ReadHeaderAt(tip)
	if err != nil {
		return nil, -1, "getblocktemplate: " + err.Error()
	}
	next := tip + 1
	net, _ := chain.ParseNetwork(chainName)
	prevHash := pow.BlockHashHex(h80[:])
	mtp, err := medianTimePastAfterPrev(j, tip)
	if err != nil {
		return nil, -1, "getblocktemplate: " + err.Error()
	}
	cur := time.Now().Unix()
	if cur < mtp {
		cur = mtp
	}
	blockTime := uint32(cur)
	bitsU, err := consensus.NextBlockBits(j, net, next, blockTime)
	if err != nil {
		return nil, -1, "getblocktemplate: next bits: " + err.Error()
	}
	ver := int64(int32(consensus.GBTBlockVersion(j, net, tip, clientRules)))
	if maxVer, ok := gbtLegacyMaxVersion(req, clientRules); ok && ver > maxVer {
		ver = maxVer
	}
	mutable := []string{"time", "transactions", "prevblock"}
	if gbtLegacyVersionForce(req, clientRules) {
		mutable = append(mutable, "version/force")
	}
	var prevLE [32]byte
	copy(prevLE[:], h80[4:36])
	subsidy := consensus.BlockSubsidy(next, prevLE, net)
	if blockMaxWeight <= 0 {
		blockMaxWeight = consensus.MaxBlockWeight
	}
	mempoolSeq := uint64(0)
	if pool != nil {
		mempoolSeq = pool.ChangeSequence()
	}
	lpID := fmt.Sprintf("%s/%d/%d", prevHash, next, mempoolSeq)
	view := consensus.AdmissionPrevOutView(pool, txIndex, rawBlocks)
	sel, _ := consensus.SelectBlockTemplateTxs(pool, view, blockMaxWeight)
	txEntries := make([]interface{}, 0, len(sel.Txs))
	for _, tx := range sel.Txs {
		deps := make([]interface{}, len(tx.Depends))
		for i, d := range tx.Depends {
			deps[i] = d
		}
		txEntries = append(txEntries, map[string]interface{}{
			"data":    tx.Data,
			"txid":    tx.TxID,
			"hash":    tx.Hash,
			"depends": deps,
			"fee":     tx.Fee,
			"sigops":  tx.SigOps,
			"weight":  tx.Weight,
		})
	}
	coinbaseValue := subsidy + sel.TotalFees
	vbBits, err := consensus.GBTVersionBits(j, net, tip, clientRules)
	if err != nil {
		return nil, -8, "getblocktemplate: "+err.Error()
	}
	out := map[string]interface{}{
		"capabilities": []string{"proposal", "longpoll"},
		"version":      ver,
		"rules":        vbBits.Rules,
		"vbrequired":   0,
		"coinbaseaux": map[string]interface{}{
			"flags": "",
		},
		"previousblockhash": prevHash,
		"bits":              pow.BitsHex(bitsU),
		"target":            pow.TargetHexFromCompact(bitsU),
		"height":            next,
		"curtime":           cur,
		"mintime":             mtp,
		"sigoplimit":          consensus.MaxBlockSigopsCost,
		"sizelimit":           consensus.MaxBlockBaseSize,
		"weightlimit":         blockMaxWeight,
		"coinbasevalue":       coinbaseValue,
		"transactions":        txEntries,
		"mutable":             mutable,
		"noncerange":          "00000000ffffffff",
		"longpollid":          lpID,
		"dogego_note": fmt.Sprintf(
			"selected %d mempool tx(s) by ancestor feerate (coinbase not included); bits from NextBlockBits",
			len(sel.Txs),
		),
	}
	if len(vbBits.VBAvailable) > 0 {
		out["vbavailable"] = vbBits.VBAvailable
	}
	return out, 0, ""
}

func gbtLongpollID(j HeaderJournal, pool *mempool.Pool, rawBlocks *store.RawBlockStore, paths *DataPaths) string {
	if j == nil {
		return ""
	}
	tip, _, _ := activeChainFromJournal(j, rawBlocks, paths)
	h80, err := j.ReadHeaderAt(tip)
	if err != nil {
		return ""
	}
	seq := uint64(0)
	if pool != nil {
		seq = pool.ChangeSequence()
	}
	return fmt.Sprintf("%s/%d/%d", pow.BlockHashHex(h80[:]), tip+1, seq)
}

// execGetBlockTemplateLegacy calls execGetBlockTemplate with nil chain/mempool stores (param validation tests only).
func execGetBlockTemplateLegacy(params []json.RawMessage) (interface{}, int, string) {
	return execGetBlockTemplate(nil, nil, nil, nil, nil, "test", 0, params)
}
