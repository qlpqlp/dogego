// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"fmt"
	"strconv"
	"strings"

	"dogego/store"
)

const (
	addrIndexDefaultPage = 40
	addrIndexMaxPage     = 200
)

// ScanAddressFromIndex returns paginated address history from indexes/addr (sub-second when indexed).
func ScanAddressFromIndex(addrIx *store.AddrIndex, address string, recvOffset, recvLimit, spendOffset, spendLimit int) (map[string]any, error) {
	if addrIx == nil {
		return nil, fmt.Errorf("no address index")
	}
	h160, ok := store.Hash160FromAddress(address)
	if !ok {
		return nil, fmt.Errorf("invalid address")
	}
	if recvLimit <= 0 {
		recvLimit = addrIndexDefaultPage
	}
	if spendLimit <= 0 {
		spendLimit = addrIndexDefaultPage
	}
	if recvLimit > addrIndexMaxPage {
		recvLimit = addrIndexMaxPage
	}
	if spendLimit > addrIndexMaxPage {
		spendLimit = addrIndexMaxPage
	}
	recvs, recvTotal, err := addrIx.LookupReceives(h160, recvOffset, recvLimit)
	if err != nil {
		return nil, err
	}
	spends, spendTotal, err := addrIx.LookupSpends(h160, spendOffset, spendLimit)
	if err != nil {
		return nil, err
	}
	hits := make([]AddrTxHit, 0, len(recvs))
	var totalKoinu int64
	for _, r := range recvs {
		hits = append(hits, AddrTxHit{
			Height:     r.Height,
			TxIndex:    int(r.TxIndex),
			TxID:       r.TxID,
			Vout:       int(r.Vout),
			ValueKoinu: r.Value,
		})
		totalKoinu += r.Value
	}
	spendHits := make([]AddrSpendHit, 0, len(spends))
	var totalSpent int64
	for _, s := range spends {
		spendHits = append(spendHits, AddrSpendHit{
			Height:     s.Height,
			TxIndex:    int(s.TxIndex),
			TxID:       s.TxID,
			Vin:        int(s.Vin),
			ValueKoinu: s.Value,
			PrevTxID:   s.PrevTxID,
			PrevVout:   int(s.PrevVout),
		})
		totalSpent += s.Value
	}
	linkOutputSpendsFromIndex(hits, addrIx)
	out := map[string]any{
		"address":                     strings.TrimSpace(address),
		"indexed":                     true,
		"matching_outputs":            hits,
		"matching_output_count":       recvTotal,
		"matching_output_offset":      recvOffset,
		"matching_output_limit":       recvLimit,
		"matching_spends":             spendHits,
		"matching_spend_count":        spendTotal,
		"matching_spend_offset":       spendOffset,
		"matching_spend_limit":        spendLimit,
		"total_received_koinu_window": totalKoinu,
		"total_received_doge_window":  float64(totalKoinu) / 1e8,
		"total_spent_koinu_window":    totalSpent,
		"total_spent_doge_window":     float64(totalSpent) / 1e8,
		"dogego_note":                 "Address history from indexes/addr (rebuild with reindextx clear=true after upgrade).",
	}
	if recvOffset+len(hits) < recvTotal || spendOffset+len(spendHits) < spendTotal {
		out["has_more"] = true
	}
	return out, nil
}

func linkOutputSpendsFromIndex(hits []AddrTxHit, addrIx *store.AddrIndex) {
	if addrIx == nil {
		return
	}
	for i := range hits {
		if spend, ok := addrIx.LookupOutpointSpend(hits[i].TxID, hits[i].Vout); ok {
			hits[i].SpendTxID = spend.TxID
			hits[i].SpendVin = int(spend.Vin)
			hits[i].SpendHeight = spend.Height
		}
	}
}

// FindSpendForOutpointIndexed tries the outspend index before falling back to block scan.
func FindSpendForOutpointIndexed(addrIx *store.AddrIndex, j *store.HeaderJournal, raw *store.RawBlockStore, prevTxid string, prevVout int) (spendTxid string, spendVin int, height int64, ok bool) {
	res := FindSpendsForOutpoints(addrIx, j, raw, prevTxid, []int{prevVout}, -1, -1)
	if hit, found := res[prevVout]; found && hit.Found {
		return hit.SpendTxid, hit.SpendVin, hit.Height, true
	}
	return "", 0, -1, false
}

func addrIndexPageFromQuery(recvOff, recvLim, spendOff, spendLim string) (ro, rl, so, sl int) {
	ro, rl, so, sl = 0, addrIndexDefaultPage, 0, addrIndexDefaultPage
	if n, err := parseNonNegInt(recvOff); err == nil {
		ro = n
	}
	if n, err := parseNonNegInt(spendOff); err == nil {
		so = n
	}
	if n, err := parseNonNegInt(recvLim); err == nil && n > 0 {
		rl = n
	}
	if n, err := parseNonNegInt(spendLim); err == nil && n > 0 {
		sl = n
	}
	if rl > addrIndexMaxPage {
		rl = addrIndexMaxPage
	}
	if sl > addrIndexMaxPage {
		sl = addrIndexMaxPage
	}
	return ro, rl, so, sl
}

// paginateAddrScanResult slices legacy scan hits/spends for API paging (avoids shipping hundreds at once).
func paginateAddrScanResult(scan map[string]any, recvOffset, recvLimit, spendOffset, spendLimit int) {
	if scan == nil {
		return
	}
	recvTotal := countAddrHits(scan, "matching_outputs")
	spendTotal := countAddrHits(scan, "matching_spends")
	scan["matching_output_count"] = recvTotal
	scan["matching_spend_count"] = spendTotal
	if hits, ok := scan["matching_outputs"].([]AddrTxHit); ok {
		scan["matching_outputs"] = sliceAddrHits(hits, recvOffset, recvLimit)
	}
	if spends, ok := scan["matching_spends"].([]AddrSpendHit); ok {
		scan["matching_spends"] = sliceAddrSpends(spends, spendOffset, spendLimit)
	}
	if recvOffset+recvLimit < recvTotal || spendOffset+spendLimit < spendTotal {
		scan["has_more"] = true
	}
}

func countAddrHits(scan map[string]any, key string) int {
	if n, ok := scan[key+"_count"].(int); ok {
		return n
	}
	switch v := scan[key].(type) {
	case []AddrTxHit:
		return len(v)
	case []AddrSpendHit:
		return len(v)
	default:
		if n, ok := scan["matching_output_count"].(int); ok && key == "matching_outputs" {
			return n
		}
		if n, ok := scan["matching_spend_count"].(int); ok && key == "matching_spends" {
			return n
		}
	}
	return 0
}

func sliceAddrHits(hits []AddrTxHit, offset, limit int) []AddrTxHit {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(hits) {
		return []AddrTxHit{}
	}
	end := len(hits)
	if limit > 0 {
		end = offset + limit
		if end > len(hits) {
			end = len(hits)
		}
	}
	return hits[offset:end]
}

func sliceAddrSpends(spends []AddrSpendHit, offset, limit int) []AddrSpendHit {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(spends) {
		return []AddrSpendHit{}
	}
	end := len(spends)
	if limit > 0 {
		end = offset + limit
		if end > len(spends) {
			end = len(spends)
		}
	}
	return spends[offset:end]
}

func parseNonNegInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid")
	}
	return n, nil
}
