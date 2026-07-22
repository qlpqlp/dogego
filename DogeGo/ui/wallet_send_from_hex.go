// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/hex"
	"sort"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func walletScriptSetFromTracked(tracked [][]byte, cfg StartConfig) (map[string][]byte, map[string]string) {
	scriptSet := make(map[string][]byte, len(tracked))
	addrByScript := make(map[string]string, len(tracked))
	if cfg.Wallet == nil {
		return scriptSet, addrByScript
	}
	pkh, sh, err := chainVersions(cfg.Network)
	if err != nil {
		return scriptSet, addrByScript
	}
	for _, pk := range tracked {
		if len(pk) == 0 {
			continue
		}
		k := string(pk)
		scriptSet[k] = pk
		addrByScript[k] = chain.ScriptPubKeyAddress(pk, pkh, sh)
	}
	return scriptSet, addrByScript
}

func walletLookupPrevOutput(cfg StartConfig, prevHash [32]byte, vout uint32) (value int64, script []byte, ok bool) {
	if cfg.UtxoCache != nil {
		if e, found := cfg.UtxoCache().LookupOutpoint(prevHash, vout); found {
			return e.Value, e.PkScript, true
		}
	}
	if cfg.TxIndex != nil && cfg.RawBlocks != nil {
		prevTx, err := store.LoadIndexedTx(cfg.TxIndex, cfg.RawBlocks, mempool.TxIDDisplayHex(prevHash))
		if err != nil {
			return 0, nil, false
		}
		if int(vout) >= len(prevTx.Vout) {
			return 0, nil, false
		}
		o := prevTx.Vout[vout]
		return o.Value, o.PkScript, true
	}
	return 0, nil, false
}

func walletSendSpendFromHex(cfg StartConfig, hx string) (sendAddr string, sendAmt int64, feeKoinu int64, ok bool) {
	if cfg.Wallet == nil {
		return "", 0, 0, false
	}
	raw, err := hex.DecodeString(strings.TrimSpace(hx))
	if err != nil {
		return "", 0, 0, false
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return "", 0, 0, false
	}
	tracked := cfg.Wallet.TrackedScripts()
	scriptSet, addrByScript := walletScriptSetFromTracked(tracked, cfg)
	if len(scriptSet) == 0 {
		return "", 0, 0, false
	}
	recvByAddr := make(map[string]int64)
	var spentTotal int64
	for _, in := range tx.Vin {
		if consensus.IsNullOutpoint(&in) {
			continue
		}
		val, pk, found := walletLookupPrevOutput(cfg, in.PrevHash, in.PrevIdx)
		if !found {
			continue
		}
		if _, tracked := scriptSet[string(pk)]; !tracked {
			continue
		}
		spentTotal += val
		if sendAddr == "" {
			sendAddr = addrByScript[string(pk)]
		}
	}
	if spentTotal <= 0 {
		return "", 0, 0, false
	}
	for _, o := range tx.Vout {
		if len(o.PkScript) == 0 {
			continue
		}
		if _, tracked := scriptSet[string(o.PkScript)]; !tracked {
			continue
		}
		addr := addrByScript[string(o.PkScript)]
		if addr == "" {
			continue
		}
		recvByAddr[addr] += o.Value
	}
	if sendAddr == "" {
		for _, addr := range addrByScript {
			if addr != "" {
				sendAddr = addr
				break
			}
		}
	}
	var totalOut int64
	for _, o := range tx.Vout {
		totalOut += o.Value
	}
	feeKoinu = spentTotal - totalOut
	if feeKoinu < 0 {
		feeKoinu = 0
	}
	sendAmt = wallet.SendDisplayKoinu(spentTotal, recvByAddr, sendAddr, tx.Vout, scriptSet)
	if sendAmt <= 0 {
		return "", 0, 0, false
	}
	return sendAddr, sendAmt, feeKoinu, true
}

func walletSendEntryNeedsHexRefresh(entry map[string]interface{}, filterKind string) bool {
	if entry == nil {
		return false
	}
	filterKind = strings.ToLower(strings.TrimSpace(filterKind))
	kindVal := strings.ToLower(strings.TrimSpace(strFromAny(entry["tx_kind"])))
	pqTag := strings.TrimSpace(strFromAny(entry["pq_tag"]))
	if filterKind == "quantum" && kindVal != "sent_pq" && pqTag == "" {
		return true
	}
	var amtKoinu int64
	feeKoinu := int64(0)
	if f, ok := entry["fee"].(float64); ok && f > 0 {
		feeKoinu = int64(f * 1e8)
	}
	if f, ok := entry["amount"].(float64); ok {
		if f < 0 {
			amtKoinu = int64(-f * 1e8)
		} else {
			amtKoinu = int64(f * 1e8)
		}
	}
	// Old scan rows stored internal transfers as fee-only sends.
	if feeKoinu > 0 && amtKoinu > 0 && amtKoinu <= feeKoinu*2 {
		return true
	}
	if kindVal != "sent_pq" && pqTag == "" {
		return filterKind == "quantum"
	}
	return false
}

func walletRefreshSendEntryFromHex(cfg StartConfig, entry map[string]interface{}, hx string, filterKind string) {
	if entry == nil || strings.TrimSpace(hx) == "" {
		return
	}
	sendAddr, sendAmt, feeKoinu, ok := walletSendSpendFromHex(cfg, hx)
	if ok {
		if sendAddr != "" {
			entry["address"] = sendAddr
		}
		entry["amount"] = -float64(sendAmt) / 1e8
		if feeKoinu > 0 {
			entry["fee"] = float64(feeKoinu) / 1e8
		}
	} else if walletSendEntryNeedsHexRefresh(entry, filterKind) {
		// Keep stored row when hex cannot be parsed; PQ tag may still apply below.
	}
	enrichWalletSendUIEntry(entry, hx)
}

func walletSendEntryFromHex(cfg StartConfig, txid, hx string, blockHeight, tip int64, filterKind string) map[string]interface{} {
	sendAddr, sendAmt, feeKoinu, ok := walletSendSpendFromHex(cfg, hx)
	if !ok {
		return nil
	}
	r := wallet.ScannedTx{
		TxID: txid, Category: "send", Address: sendAddr,
		AmountKoinu: -sendAmt, FeeKoinu: feeKoinu, BlockHeight: blockHeight,
	}
	entry := scannedSendToUIEntry(cfg, r, tip, filterKind)
	if entry == nil {
		return nil
	}
	return entry
}

const walletSupplementMaxCandidates = 48

func walletSupplementMissingSends(cfg StartConfig, entries []map[string]interface{}, scanned []wallet.ScannedTx, tip int64, kind string) []map[string]interface{} {
	if cfg.Wallet == nil || len(scanned) == 0 {
		return entries
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "all" && kind != "sent" && kind != "send" && kind != "quantum" {
		return entries
	}
	sendTxids := make(map[string]struct{})
	for _, r := range scanned {
		if r.Category != "send" {
			continue
		}
		if id := strings.ToLower(strings.TrimSpace(r.TxID)); id != "" {
			sendTxids[id] = struct{}{}
		}
	}
	type recvCandidate struct {
		txid   string
		height int64
	}
	recvHeights := make(map[string]int64)
	for _, r := range scanned {
		if r.Category != "receive" {
			continue
		}
		id := strings.ToLower(strings.TrimSpace(r.TxID))
		if id == "" {
			continue
		}
		if _, ok := sendTxids[id]; ok {
			continue
		}
		if h, ok := recvHeights[id]; !ok || r.BlockHeight > h {
			recvHeights[id] = r.BlockHeight
		}
	}
	candidates := make([]recvCandidate, 0, len(recvHeights))
	for id, height := range recvHeights {
		candidates = append(candidates, recvCandidate{txid: id, height: height})
	}
	if len(candidates) == 0 {
		return entries
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].height != candidates[j].height {
			return candidates[i].height > candidates[j].height
		}
		return candidates[i].txid > candidates[j].txid
	})
	seen := make(map[string]struct{}, walletSupplementMaxCandidates)
	for _, c := range candidates {
		if len(seen) >= walletSupplementMaxCandidates {
			break
		}
		if _, ok := seen[c.txid]; ok {
			continue
		}
		seen[c.txid] = struct{}{}
		hx := walletTxHexForUI(cfg, c.txid, c.height)
		if hx == "" {
			continue
		}
		entry := walletSendEntryFromHex(cfg, c.txid, hx, c.height, tip, kind)
		if entry == nil {
			continue
		}
		if !walletHistoryEntryMatchesKind(entry, kind) {
			continue
		}
		entries = append(entries, entry)
		sendTxids[c.txid] = struct{}{}
	}
	if len(entries) > 1 {
		sort.Slice(entries, func(i, j int) bool {
			return walletHistoryEntrySortKey(entries[i]) > walletHistoryEntrySortKey(entries[j])
		})
	}
	return entries
}
