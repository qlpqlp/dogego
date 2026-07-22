// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"dogego/mempool"
	"dogego/store"
)

var walletTxRowCache struct {
	mu   sync.Mutex
	key  string
	at   time.Time
	rows []walletTxRow
}

const walletTxRowCacheTTL = 20 * time.Second

func walletTxRowCacheKey(chainName string, paths *DataPaths) string {
	if paths == nil || paths.Utxo == nil {
		return chainName + ":0"
	}
	return fmt.Sprintf("%s:%d", chainName, paths.Utxo.TipHeight())
}

func walletUIRowsCached(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool) []walletTxRow {
	key := walletTxRowCacheKey(chainName, paths)
	now := time.Now()
	walletTxRowCache.mu.Lock()
	if walletTxRowCache.key == key && now.Sub(walletTxRowCache.at) < walletTxRowCacheTTL && walletTxRowCache.rows != nil {
		out := walletTxRowCache.rows
		walletTxRowCache.mu.Unlock()
		return out
	}
	walletTxRowCache.mu.Unlock()

	rows, code, _ := walletCollectTransactionsUI(chainName, paths, j, raw, pool, 1)
	if code != 0 {
		return []walletTxRow{}
	}
	walletTxRowCache.mu.Lock()
	walletTxRowCache.key = key
	walletTxRowCache.at = now
	walletTxRowCache.rows = rows
	walletTxRowCache.mu.Unlock()
	return rows
}

// WalletTxListPage is a paginated slice of wallet history rows for the web UI.
type WalletTxListPage struct {
	Total  int
	Offset int
	Limit  int
	Items  []interface{}
}

// WalletListTransactionsPage returns filtered wallet rows (newest first) with offset/limit paging.
func WalletListTransactionsPage(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex, offset, limit int, q, kind string) WalletTxListPage {
	maturity := walletCoinbaseMaturity(chainName, j, raw, paths)
	rows := walletUIRowsCached(chainName, paths, j, raw, pool)
	filtered := filterWalletTxRows(rows, q, kind, maturity, chainName, paths, j, raw, pool, ix)
	total := len(filtered)
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
	page := filtered[offset:end]
	items := make([]interface{}, len(page))
	for i := range page {
		walletTxRowFillHeaders(j, &page[i])
		addr := page[i].address
		items[i] = walletTxRowToUIListEntry(chainName, paths, j, raw, pool, ix, addr, page[i], maturity)
	}
	return WalletTxListPage{
		Total:  total,
		Offset: offset,
		Limit:  limit,
		Items:  items,
	}
}

func filterWalletTxRows(rows []walletTxRow, q, kind string, maturity int64, chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex) []walletTxRow {
	q = strings.ToLower(strings.TrimSpace(q))
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "all"
	}
	out := make([]walletTxRow, 0, len(rows))
	for _, r := range rows {
		kindVal := walletTxKindHeuristic(r, maturity)
		pqTag := ""
		if kind == "quantum" {
			if ek, pq := walletEnrichTxKindList(paths, pool, r); ek != "" {
				kindVal = ek
				pqTag = pq
			}
		}
		if kind != "all" && !walletTxRowMatchesKind(kindVal, r.category, kind, pqTag) {
			continue
		}
		if q != "" && !walletTxRowMatchesSearch(r, kindVal, q) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func walletTxRowMatchesKind(kindVal, category, kind, pqTag string) bool {
	switch kind {
	case "sent":
		return kindVal == "sent" || kindVal == "send" || kindVal == "sent_pq" || category == "send"
	case "received":
		return kindVal == "received" || kindVal == "receive" || kindVal == "received_pq" || category == "receive"
	case "mining":
		return kindVal == "mining" || kindVal == "mining_immature" || kindVal == "generate" || kindVal == "immature"
	case "quantum":
		return kindVal == "sent_pq" || kindVal == "received_pq" || strings.TrimSpace(pqTag) != ""
	}
	return true
}

func walletTxRowMatchesSearch(r walletTxRow, kindVal, q string) bool {
	blob := strings.ToLower(strings.Join([]string{
		r.txid,
		r.address,
		r.category,
		kindVal,
		fmt.Sprint(float64(r.amountKoinu) / 1e8),
	}, " "))
	return strings.Contains(blob, q)
}

func filterWalletTxEntries(all []interface{}, q, kind string) []interface{} {
	q = strings.ToLower(strings.TrimSpace(q))
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "all"
	}
	out := make([]interface{}, 0, len(all))
	for _, it := range all {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		if kind != "all" && !walletTxEntryMatchesKind(m, kind) {
			continue
		}
		if q != "" && !walletTxEntryMatchesSearch(m, q) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func walletTxEntryMatchesKind(m map[string]interface{}, kind string) bool {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" || kind == "all" {
		return true
	}
	kindVal := strings.ToLower(fmt.Sprint(m["tx_kind"]))
	if kindVal == "" {
		kindVal = strings.ToLower(fmt.Sprint(m["category"]))
	}
	if kind == "quantum" {
		if kindVal == "sent_pq" {
			return true
		}
		return strings.TrimSpace(fmt.Sprint(m["pq_tag"])) != ""
	}
	return walletTxRowMatchesKind(kindVal, strings.ToLower(fmt.Sprint(m["category"])), kind, strings.TrimSpace(fmt.Sprint(m["pq_tag"])))
}

func walletTxEntryMatchesSearch(m map[string]interface{}, q string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return true
	}
	blob := strings.ToLower(strings.Join([]string{
		fmt.Sprint(m["txid"]),
		fmt.Sprint(m["address"]),
		fmt.Sprint(m["label"]),
		fmt.Sprint(m["category"]),
		fmt.Sprint(m["tx_kind"]),
		fmt.Sprint(m["pq_tag"]),
		fmt.Sprint(m["amount"]),
	}, " "))
	return strings.Contains(blob, q)
}
