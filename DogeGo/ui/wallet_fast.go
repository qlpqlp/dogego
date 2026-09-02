// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"fmt"
	"sort"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/store"
	"dogego/wallet"
)

// walletScriptIndex maps wallet spend scripts to display addresses (all HD receive/change indices).
type walletScriptIndex struct {
	byScript map[string]string
}

func walletScriptIndexFor(cfg StartConfig) (walletScriptIndex, bool) {
	pub, scriptHash, err := chainVersions(cfg.Network)
	if err != nil {
		return walletScriptIndex{}, false
	}
	idx := walletScriptIndex{byScript: make(map[string]string)}
	if cfg.ActiveWallet() != nil {
		for _, script := range cfg.ActiveWallet().SpendScripts() {
			if len(script) == 0 {
				continue
			}
			if a := chain.ScriptPubKeyAddress(script, pub, scriptHash); a != "" {
				idx.byScript[string(script)] = a
			}
		}
	}
	if len(idx.byScript) == 0 && cfg.ActiveWallet() != nil {
		addr := strings.TrimSpace(walletAddr(cfg.ActiveWallet()))
		if addr != "" {
			if _, payload, err := chain.Base58CheckDecode(addr); err == nil {
				spk := chain.P2PKHScriptFromPubKeyHash(payload)
				idx.byScript[string(spk)] = addr
			}
		}
	}
	if len(idx.byScript) == 0 {
		return walletScriptIndex{}, false
	}
	return idx, true
}

func (idx walletScriptIndex) match(pkScript []byte) (addr string, ok bool) {
	if len(pkScript) == 0 {
		return "", false
	}
	a, ok := idx.byScript[string(pkScript)]
	return a, ok
}

func collectWalletUtxoRowsFromCache(cfg StartConfig) (rows []walletUtxoTxRow, tip, maturity int64, ok bool) {
	utxo := utxoCacheLive(cfg)
	if utxo == nil || utxo.TipHeight() < 0 {
		return nil, 0, 0, false
	}
	idx, okIdx := walletScriptIndexFor(cfg)
	if !okIdx {
		return nil, 0, 0, false
	}
	scriptSet := make(map[string]struct{}, len(idx.byScript))
	for script := range idx.byScript {
		scriptSet[script] = struct{}{}
	}
	tip = utxo.TipHeight()
	maturity = int64(coinbaseMaturityBlocks(cfg.Network, tip))
	rows = make([]walletUtxoTxRow, 0, 64)
	for _, row := range utxo.FilterRowsByScriptSet(scriptSet, 0) {
		addr, match := idx.match(row.PkScript)
		if !match {
			continue
		}
		conf := int64(1)
		if tip >= 0 && row.Height >= 0 {
			conf = tip - row.Height + 1
			if conf < 1 {
				conf = 1
			}
		}
		rows = append(rows, walletUtxoTxRow{
			txid: row.TxID, vout: row.Vout, height: row.Height,
			valueKoinu: row.Value, confirmations: conf, address: addr,
		})
	}
	return rows, tip, maturity, true
}

// walletBalanceFromUtxoCache sums confirmed and immature balances from the in-memory UTXO
// set (same source BlockStep uses). Avoids slow getwalletinfo/listunspent on solo miners.
func walletBalanceFromUtxoCache(cfg StartConfig) (confirmed, immature float64, utxoCount int, ok bool) {
	rows, _, maturity, ok := collectWalletUtxoRowsFromCache(cfg)
	if !ok {
		return 0, 0, 0, false
	}
	if len(rows) == 0 {
		return 0, 0, 0, true
	}
	var confKoinu, immKoinu int64
	for _, r := range rows {
		if r.vout == 0 && r.confirmations > 0 && r.confirmations < maturity {
			immKoinu += r.valueKoinu
		} else if r.confirmations > 0 {
			confKoinu += r.valueKoinu
		}
		utxoCount++
	}
	return float64(confKoinu) / 1e8, float64(immKoinu) / 1e8, utxoCount, true
}

// walletListUnspentFromUtxoCache returns listunspent-shaped rows for Send coin control (fast solo-miner path).
func walletListUnspentFromUtxoCache(cfg StartConfig) ([]interface{}, bool) {
	rows, _, maturity, ok := collectWalletUtxoRowsFromCache(cfg)
	if !ok {
		return nil, false
	}
	if len(rows) == 0 {
		return []interface{}{}, true
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].height != rows[j].height {
			return rows[i].height > rows[j].height
		}
		return rows[i].txid > rows[j].txid
	})
	out := make([]interface{}, len(rows))
	for i, r := range rows {
		spendable := true
		if r.vout == 0 && r.confirmations > 0 && r.confirmations < maturity {
			spendable = false
		}
		if r.confirmations < 1 {
			spendable = false
		}
		addr := r.address
		if addr == "" {
			addr = walletAddr(cfg.ActiveWallet())
		}
		out[i] = map[string]interface{}{
			"txid":          r.txid,
			"vout":          r.vout,
			"address":       addr,
			"amount":        float64(r.valueKoinu) / 1e8,
			"confirmations": r.confirmations,
			"spendable":     spendable,
			"solvable":      spendable,
			"safe":          spendable,
		}
	}
	return out, true
}

type walletUtxoTxRow struct {
	txid          string
	vout          uint32
	height        int64
	valueKoinu    int64
	confirmations int64
	address       string
}

// walletTxPageFromUtxoCache lists wallet receive rows from the UTXO cache (fast solo-miner path).
func walletTxPageFromUtxoCache(cfg StartConfig, offset, limit int, q, kind string) (total int, items []interface{}, ok bool) {
	rows, _, maturity, ok := collectWalletUtxoRowsFromCache(cfg)
	if !ok {
		return 0, nil, false
	}
	if len(rows) == 0 {
		return 0, []interface{}{}, true
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].height != rows[j].height {
			return rows[i].height > rows[j].height
		}
		return rows[i].txid > rows[j].txid
	})
	filtered := filterWalletUtxoRows(rows, q, kind, maturity)
	entries := make([]map[string]interface{}, 0, len(filtered))
	for _, r := range filtered {
		addr := r.address
		if addr == "" {
			addr = walletAddr(cfg.ActiveWallet())
		}
		entries = append(entries, walletUtxoRowToEntryWithPQ(cfg, r, addr, maturity, cfg.ActiveJournal()))
	}
	entries = walletCollapseUIEntries(entries)
	total = len(entries)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := total
	if limit > 0 {
		end = offset + limit
		if end > total {
			end = total
		}
	}
	items = make([]interface{}, end-offset)
	for i, e := range entries[offset:end] {
		items[i] = e
	}
	return total, items, true
}

// walletTxHistoryUsesScannedSendFastPath reports whether /api/wallet/txs may list sends from wallet.db scan index.
func walletTxHistoryUsesScannedSendFastPath(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "sent", "send", "quantum":
		return true
	default:
		return false
	}
}

// walletTxPageFromScannedSend lists confirmed wallet sends from wallet.db (no UTXO-cache walk).
func walletTxPageFromScannedSend(cfg StartConfig, offset, limit int, q, kind string) (total int, items []interface{}, ok bool) {
	if cfg.ActiveWallet() == nil {
		return 0, nil, false
	}
	scanned := cfg.ActiveWallet().ListScannedTx()
	if len(scanned) == 0 {
		return 0, nil, false
	}
	tip := int64(-1)
	if utxo := utxoCacheLive(cfg); utxo != nil {
		tip = utxo.TipHeight()
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	q = strings.ToLower(strings.TrimSpace(q))
	filtered := make([]wallet.ScannedTx, 0, len(scanned))
	for _, r := range scanned {
		if r.Category != "send" {
			continue
		}
		if q != "" {
			blob := strings.ToLower(strings.Join([]string{
				r.TxID, r.Address, "send", "sent",
				fmt.Sprint(float64(r.AmountKoinu) / 1e8),
			}, " "))
			if !strings.Contains(blob, q) {
				continue
			}
		}
		entry := scannedSendToUIEntry(cfg, r, tip, kind)
		if !walletHistoryEntryMatchesKind(entry, kind) {
			continue
		}
		filtered = append(filtered, r)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].BlockHeight != filtered[j].BlockHeight {
			return filtered[i].BlockHeight > filtered[j].BlockHeight
		}
		return filtered[i].TxID > filtered[j].TxID
	})
	total = len(filtered)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := total
	if limit > 0 {
		end = offset + limit
		if end > total {
			end = total
		}
	}
	pageRows := filtered[offset:end]
	entries := make([]map[string]interface{}, 0, len(pageRows))
	for _, r := range pageRows {
		entry := scannedSendToUIEntry(cfg, r, tip, kind)
		entries = append(entries, entry)
	}
	items = make([]interface{}, len(entries))
	for i := range entries {
		items[i] = entries[i]
	}
	return total, items, true
}

func scannedSendToUIEntry(cfg StartConfig, r wallet.ScannedTx, tip int64, kind ...string) map[string]interface{} {
	return scannedTxToUIEntry(cfg, r, tip, kind...)
}

func scannedTxToUIEntry(cfg StartConfig, r wallet.ScannedTx, tip int64, kind ...string) map[string]interface{} {
	category := strings.TrimSpace(r.Category)
	if category == "" {
		category = "receive"
	}
	conf := int64(0)
	if r.BlockHeight < 0 {
		conf = 0
	} else if tip >= 0 && r.BlockHeight >= 0 {
		conf = tip - r.BlockHeight + 1
		if conf < 1 {
			conf = 1
		}
	} else {
		conf = 1
	}
	rowTime := r.BlockHeight
	if cfg.ActiveJournal() != nil && r.BlockHeight >= 0 {
		if h80, err := cfg.ActiveJournal().ReadHeaderAt(r.BlockHeight); err == nil {
			rowTime = headerTimeUnix(h80)
		}
	}
	txKind := "received"
	if category == "send" {
		txKind = "sent"
	}
	amountKoinu := r.AmountKoinu
	if category == "send" && amountKoinu > 0 {
		amountKoinu = -amountKoinu
	}
	entry := map[string]interface{}{
		"account":            "",
		"label":              "",
		"address":            r.Address,
		"category":           category,
		"amount":             float64(amountKoinu) / 1e8,
		"confirmations":      conf,
		"txid":               r.TxID,
		"time":               rowTime,
		"timereceived":       rowTime,
		"bip125-replaceable": "no",
		"walletconflicts":    []interface{}{},
		"tx_kind":            txKind,
		"vout":               r.Vout,
		"trusted":            conf > 0,
	}
	if r.BlockHeight >= 0 {
		entry["blockheight"] = r.BlockHeight
		entry["blockindex"] = r.Vout
		if cfg.ActiveJournal() != nil {
			if h80, err := cfg.ActiveJournal().ReadHeaderAt(r.BlockHeight); err == nil {
				entry["blocktime"] = headerTimeUnix(h80)
			}
		}
	}
	if category == "receive" {
		enrichWalletReceiveUIEntry(cfg, entry)
	}
	if r.FeeKoinu > 0 {
		entry["fee"] = float64(r.FeeKoinu) / 1e8
	}
	if hx := walletTxHexForUI(cfg, r.TxID, r.BlockHeight); hx != "" {
		entry["hex"] = hx
		kindFilter := ""
		if len(kind) > 0 {
			kindFilter = kind[0]
		}
		if category == "send" {
			walletRefreshSendEntryFromHex(cfg, entry, hx, kindFilter)
		}
	}
	return entry
}

// walletTxPageFromScannedHistory lists receive+send rows from wallet.db (no UTXO-cache walk).
func walletTxPageFromScannedHistory(cfg StartConfig, offset, limit int, q, kind string) (total int, items []interface{}, ok bool) {
	if cfg.ActiveWallet() == nil {
		return 0, nil, false
	}
	scanned := cfg.ActiveWallet().ListScannedTx()
	if len(scanned) == 0 {
		return 0, nil, false
	}
	tip := int64(-1)
	if utxo := utxoCacheLive(cfg); utxo != nil {
		tip = utxo.TipHeight()
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	q = strings.ToLower(strings.TrimSpace(q))
	filtered := make([]wallet.ScannedTx, 0, len(scanned))
	for _, r := range scanned {
		entry := scannedTxToUIEntry(cfg, r, tip, kind)
		if !walletHistoryEntryMatchesKind(entry, kind) {
			continue
		}
		if q != "" && !walletHistoryEntryMatchesSearch(entry, q) {
			continue
		}
		filtered = append(filtered, r)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].BlockHeight != filtered[j].BlockHeight {
			return filtered[i].BlockHeight > filtered[j].BlockHeight
		}
		return filtered[i].TxID > filtered[j].TxID
	})
	total = len(filtered)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := total
	if limit > 0 {
		end = offset + limit
		if end > total {
			end = total
		}
	}
	pageRows := filtered[offset:end]
	entries := make([]map[string]interface{}, 0, len(pageRows))
	for _, r := range pageRows {
		entries = append(entries, scannedTxToUIEntry(cfg, r, tip, kind))
	}
	entries = walletCollapseUIEntries(entries)
	items = make([]interface{}, len(entries))
	for i := range entries {
		items[i] = entries[i]
	}
	return total, items, true
}

func walletScanHasReceiveRows(w *wallet.Disk) bool {
	if w == nil {
		return false
	}
	for _, r := range w.ListScannedTx() {
		if r.Category == "receive" {
			return true
		}
	}
	return false
}

func walletHistoryEntryMatchesSearch(entry map[string]interface{}, q string) bool {
	if entry == nil || q == "" {
		return true
	}
	q = strings.ToLower(strings.TrimSpace(q))
	blob := strings.ToLower(strings.Join([]string{
		strFromAny(entry["txid"]),
		strFromAny(entry["address"]),
		strFromAny(entry["category"]),
		strFromAny(entry["tx_kind"]),
		fmt.Sprint(entry["amount"]),
	}, " "))
	return strings.Contains(blob, q)
}

// walletTxHistoryUsesUtxoFastPath reports whether /api/wallet/txs may use the UTXO-cache
// fast path. That path only lists receive/mining rows (no sends, no PQ enrichment).
func walletTxHistoryUsesUtxoFastPath(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "all", "received", "mining", "mining_immature", "immature", "generate":
		return true
	default:
		return false
	}
}

func walletUtxoRowMatchesKind(kindVal, filter string) bool {
	switch filter {
	case "sent":
		return kindVal == "sent" || kindVal == "send" || kindVal == "sent_pq"
	case "received":
		return kindVal == "received" || kindVal == "receive" || kindVal == "received_pq"
	case "mining":
		return kindVal == "mining" || kindVal == "mining_immature" || kindVal == "generate" || kindVal == "immature"
	case "quantum":
		return kindVal == "sent_pq" || kindVal == "received_pq"
	}
	return true
}

func filterWalletUtxoRows(rows []walletUtxoTxRow, q, kind string, maturity int64) []walletUtxoTxRow {
	q = strings.ToLower(strings.TrimSpace(q))
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "all"
	}
	out := make([]walletUtxoTxRow, 0, len(rows))
	for _, r := range rows {
		kindVal := walletUtxoTxKind(r, maturity)
		if kind != "all" && !walletUtxoRowMatchesKind(kindVal, kind) {
			continue
		}
		if q != "" {
			blob := strings.ToLower(strings.Join([]string{
				r.txid, kindVal, fmt.Sprint(float64(r.valueKoinu) / 1e8),
			}, " "))
			if !strings.Contains(blob, q) {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

func walletUtxoTxKind(r walletUtxoTxRow, maturity int64) string {
	if r.vout == 0 && r.confirmations > 0 {
		if maturity > 0 && r.confirmations < maturity {
			return "mining_immature"
		}
		return "mining"
	}
	return "received"
}

func walletUtxoRowToEntry(r walletUtxoTxRow, address string, maturity int64, j *store.HeaderJournal) map[string]interface{} {
	kind := walletUtxoTxKind(r, maturity)
	rowTime := r.height
	if j != nil && r.height >= 0 {
		if h80, err := j.ReadHeaderAt(r.height); err == nil {
			rowTime = headerTimeUnix(h80)
		}
	}
	entry := map[string]interface{}{
		"account":            "",
		"label":              "",
		"address":            address,
		"category":           "receive",
		"amount":             float64(r.valueKoinu) / 1e8,
		"confirmations":      r.confirmations,
		"txid":               r.txid,
		"time":               rowTime,
		"timereceived":       rowTime,
		"bip125-replaceable": "no",
		"walletconflicts":    []interface{}{},
		"tx_kind":            kind,
		"vout":               r.vout,
		"trusted":            r.confirmations > 0,
	}
	if r.height >= 0 {
		entry["blockheight"] = r.height
		entry["blockindex"] = r.vout
		if j != nil {
			if h80, err := j.ReadHeaderAt(r.height); err == nil {
				entry["blocktime"] = headerTimeUnix(h80)
			}
		}
	}
	return entry
}

func walletUtxoRowToEntryWithPQ(cfg StartConfig, r walletUtxoTxRow, address string, maturity int64, j *store.HeaderJournal) map[string]interface{} {
	entry := walletUtxoRowToEntry(r, address, maturity, j)
	enrichWalletReceiveUIEntry(cfg, entry)
	return entry
}

func walletHasScannedIndex(w *wallet.Disk) bool {
	return w != nil && len(w.ListScannedTx()) > 0
}

// walletTxPageMergedAll combines wallet.db history when indexed, else UTXO receives + scan sends.
func walletTxPageMergedAll(cfg StartConfig, offset, limit int, q string) (total int, items []interface{}, ok bool) {
	if cfg.ActiveWallet() == nil || !walletHasScannedIndex(cfg.ActiveWallet()) {
		return 0, nil, false
	}
	if walletScanHasReceiveRows(cfg.ActiveWallet()) {
		return walletTxPageFromScannedHistory(cfg, offset, limit, q, "all")
	}
	_, recvItems, recvOK := walletTxPageFromUtxoCache(cfg, 0, 0, q, "all")
	if !recvOK {
		return 0, nil, false
	}
	_, sendItems, sendOK := walletTxPageFromScannedSend(cfg, 0, 0, q, "all")
	if !sendOK {
		return 0, nil, false
	}
	merged := append(sendItems, recvItems...)
	mergedMaps := make([]map[string]interface{}, 0, len(merged))
	for _, item := range merged {
		if m, ok := item.(map[string]interface{}); ok && m != nil {
			mergedMaps = append(mergedMaps, m)
		}
	}
	mergedMaps = walletCollapseUIEntries(mergedMaps)
	sort.Slice(mergedMaps, func(i, j int) bool {
		return walletHistoryEntrySortKey(mergedMaps[i]) > walletHistoryEntrySortKey(mergedMaps[j])
	})
	total = len(mergedMaps)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := total
	if limit > 0 {
		end = offset + limit
		if end > total {
			end = total
		}
	}
	page := mergedMaps[offset:end]
	items = make([]interface{}, len(page))
	for i, e := range page {
		items[i] = e
	}
	return total, items, true
}

func walletHistoryEntrySortKey(item interface{}) int64 {
	m, ok := item.(map[string]interface{})
	if !ok || m == nil {
		return 0
	}
	if h, ok := historyEntryInt64(m["blockheight"]); ok {
		return h<<32 + historyEntryInt64Or(m["vout"], 0)
	}
	if t, ok := historyEntryInt64(m["time"]); ok {
		return t
	}
	return 0
}

func historyEntryInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

func historyEntryInt64Or(v interface{}, def int64) int64 {
	if n, ok := historyEntryInt64(v); ok {
		return n
	}
	return def
}

// walletCollapseUIEntries dedupes UI history rows: one send per txid, one receive per txid+address.
func walletCollapseUIEntries(entries []map[string]interface{}) []map[string]interface{} {
	if len(entries) <= 1 {
		return entries
	}
	sendByTx := make(map[string]int)
	recvByKey := make(map[string]int)
	out := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		if e == nil {
			continue
		}
		cat, _ := e["category"].(string)
		txid, _ := e["txid"].(string)
		addr, _ := e["address"].(string)
		switch cat {
		case "send":
			id := strings.ToLower(strings.TrimSpace(txid))
			if id == "" {
				out = append(out, e)
				continue
			}
			amt := historyEntryAmountKoinu(e)
			if i, ok := sendByTx[id]; ok {
				if absHistoryKoinu(amt) > absHistoryKoinu(historyEntryAmountKoinu(out[i])) {
					out[i] = e
				}
				continue
			}
			sendByTx[id] = len(out)
			out = append(out, e)
		case "receive":
			key := strings.ToLower(strings.TrimSpace(txid)) + ":" + strings.TrimSpace(addr)
			if i, ok := recvByKey[key]; ok {
				out[i]["amount"] = historyEntryFloat(out[i]["amount"]) + historyEntryFloat(e["amount"])
				bhNew, _ := historyEntryInt64(e["blockheight"])
				bhOld, _ := historyEntryInt64(out[i]["blockheight"])
				if bhNew > bhOld {
					out[i]["blockheight"] = e["blockheight"]
					out[i]["confirmations"] = e["confirmations"]
					out[i]["vout"] = e["vout"]
					out[i]["time"] = e["time"]
				}
				continue
			}
			recvByKey[key] = len(out)
			out = append(out, e)
		default:
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return walletHistoryEntrySortKey(out[i]) > walletHistoryEntrySortKey(out[j])
	})
	return out
}

func historyEntryFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	default:
		return 0
	}
}

func historyEntryAmountKoinu(e map[string]interface{}) int64 {
	return int64(historyEntryFloat(e["amount"]) * 1e8)
}

func absHistoryKoinu(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func utxoCacheLive(cfg StartConfig) *store.UtxoCache {
	if cfg.UtxoCache == nil {
		return nil
	}
	return cfg.UtxoCache()
}

func chainVersions(network string) (pubkeyVer, scriptHashVer byte, err error) {
	net, err := networkFromUISlug(network)
	if err != nil {
		return 0, 0, err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return 0, 0, err
	}
	return p.PubkeyHashAddrID, p.ScriptHashAddrID, nil
}

func coinbaseMaturityBlocks(network string, height int64) int {
	net, err := networkFromUISlug(network)
	if err != nil {
		return 30
	}
	if height < 0 {
		height = 0
	}
	return consensus.LookupConsensus(net, height).CoinbaseMaturity
}
