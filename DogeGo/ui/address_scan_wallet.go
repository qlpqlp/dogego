// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"strings"

	"dogego/wallet"
)

// ScanAddressWalletFast returns wallet-owned address history from UTXO cache + wallet.db (no raw block walk).
func ScanAddressWalletFast(cfg StartConfig, address string, pubVer, scriptVer byte, recvOffset, recvLimit, spendOffset, spendLimit int) (map[string]any, bool) {
	if cfg.Wallet == nil || !cfg.Wallet.ContainsAddress(address) {
		return nil, false
	}
	utxo := utxoCacheLive(cfg)
	if utxo == nil || utxo.TipHeight() < 0 {
		return nil, false
	}
	want := strings.ToLower(strings.TrimSpace(address))
	scriptSet := addressPkScriptSet(address, pubVer, scriptVer)
	if len(scriptSet) == 0 {
		return nil, false
	}
	seen := make(map[string]struct{})
	var hits []AddrTxHit
	var totalKoinu int64
	for _, row := range utxo.FilterRowsByScriptSet(scriptSet, 0) {
		k := outpointKey(row.TxID, int(row.Vout))
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		hits = append(hits, AddrTxHit{
			Height: row.Height, TxID: row.TxID, Vout: int(row.Vout), ValueKoinu: row.Value,
		})
		totalKoinu += row.Value
	}
	appendWalletScannedReceives(cfg.Wallet, want, &hits, seen, &totalKoinu)
	var spendHits []AddrSpendHit
	var totalSpent int64
	appendWalletScannedSends(cfg.Wallet, want, &spendHits, &totalSpent)
	if len(hits) == 0 && len(spendHits) == 0 {
		return nil, false
	}
	recvTotal := len(hits)
	spendTotal := len(spendHits)
	out := map[string]any{
		"address":                     address,
		"wallet_fast":                   true,
		"indexed":                     false,
		"matching_outputs":            sliceAddrHits(hits, recvOffset, recvLimit),
		"matching_output_count":       recvTotal,
		"matching_output_offset":      recvOffset,
		"matching_output_limit":       recvLimit,
		"matching_spends":             sliceAddrSpends(spendHits, spendOffset, spendLimit),
		"matching_spend_count":        spendTotal,
		"matching_spend_offset":       spendOffset,
		"matching_spend_limit":        spendLimit,
		"total_received_koinu_window": totalKoinu,
		"total_received_doge_window":  float64(totalKoinu) / 1e8,
		"total_spent_koinu_window":    totalSpent,
		"total_spent_doge_window":     float64(totalSpent) / 1e8,
		"chain_active_height":         utxo.TipHeight(),
		"dogego_note":                 "Wallet fast path: UTXO cache + wallet.db scan index (no raw block walk).",
	}
	if recvOffset+recvLimit < recvTotal || spendOffset+spendLimit < spendTotal {
		out["has_more"] = true
	}
	if recvTotal >= addrScanMaxHits || spendTotal >= addrScanMaxHits {
		out["truncated"] = true
	}
	return out, true
}

func appendWalletScannedReceives(w *wallet.Disk, want string, hits *[]AddrTxHit, seen map[string]struct{}, total *int64) {
	if w == nil {
		return
	}
	for _, r := range w.ListScannedTx() {
		if r.Category != "receive" || !strings.EqualFold(strings.TrimSpace(r.Address), want) {
			continue
		}
		k := outpointKey(r.TxID, int(r.Vout))
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		*hits = append(*hits, AddrTxHit{
			Height: r.BlockHeight, TxID: r.TxID, Vout: int(r.Vout), ValueKoinu: r.AmountKoinu,
		})
		*total += r.AmountKoinu
		if len(*hits) >= addrScanMaxHits {
			return
		}
	}
}

func appendWalletScannedSends(w *wallet.Disk, want string, spends *[]AddrSpendHit, total *int64) {
	if w == nil {
		return
	}
	for _, r := range w.ListScannedTx() {
		if r.Category != "send" || !strings.EqualFold(strings.TrimSpace(r.Address), want) {
			continue
		}
		amt := r.AmountKoinu
		if amt < 0 {
			amt = -amt
		}
		*spends = append(*spends, AddrSpendHit{
			Height: r.BlockHeight, TxID: r.TxID, Vin: 0, ValueKoinu: amt,
		})
		*total += amt
		if len(*spends) >= addrScanMaxHits {
			return
		}
	}
}
