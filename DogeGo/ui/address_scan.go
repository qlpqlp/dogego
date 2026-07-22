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

	"dogego/chain"
	"dogego/mempool"
	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
	"dogego/wire"
)

const addrScanMaxRawBlocks = 8000
const addrScanMaxHits = 120

// AddrTxHit is one vout in a stored raw block that pays to the given P2PKH address.
type AddrTxHit struct {
	Height      int64  `json:"height"`
	TxIndex     int    `json:"tx_index"`
	TxID        string `json:"txid"`
	Vout        int    `json:"vout"`
	ValueKoinu  int64  `json:"value_satoshi"`
	SpendTxID   string `json:"spend_txid,omitempty"`
	SpendVin    int    `json:"spend_vin,omitempty"`
	SpendHeight int64  `json:"spend_height,omitempty"`
}

// AddrSpendHit is one vin in a stored raw block that spends a prevout paying `address`.
type AddrSpendHit struct {
	Height       int64  `json:"height"`
	TxIndex      int    `json:"tx_index"`
	TxID         string `json:"txid"`
	Vin          int    `json:"vin"`
	ValueKoinu   int64  `json:"value_satoshi"`
	PrevTxID     string `json:"prev_txid"`
	PrevVout     int    `json:"prev_vout"`
}

// ScanAddressInRawWindow scans stored raw blocks (and optional mempool) for outputs
// paying `address`, plus spends when tx index is available. Unspent UTXOs from the
// in-memory cache are merged so balances match even when receives are outside the scan window.
func ScanAddressInRawWindow(j *store.HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, pubkeyVer, scriptHashVer byte, address string, pool *mempool.Pool, hintTxid string, hintVout int, utxoFn func() *store.UtxoCache) (map[string]any, error) {
	if j == nil {
		return nil, fmt.Errorf("no header journal")
	}
	if raw == nil {
		return nil, fmt.Errorf("no raw block store")
	}
	address = normalizeScanAddress(address, pubkeyVer, scriptHashVer)
	if address == "" {
		return nil, fmt.Errorf("empty address")
	}
	headerTip, err := j.TipHeight()
	if err != nil {
		return nil, err
	}
	scanTip, minH := addressScanHeightRange(j, raw, headerTip)
	var heights []int64
	for h := scanTip; h >= minH; h-- {
		heights = append(heights, h)
	}
	var hits []AddrTxHit
	var spends []AddrSpendHit
	var rawHits, rawMiss, parseErrs int
	var totalKoinu, totalSpentKoinu int64
	mergeUtxoOutputsForAddress(&hits, utxoFn, address, pubkeyVer, scriptHashVer, &totalKoinu)
	skipRawScan := len(hits) >= addrScanMaxHits
	if !skipRawScan {
		for i := 0; i < len(heights) && (len(hits) < addrScanMaxHits || len(spends) < addrScanMaxHits); i++ {
			h := heights[i]
			h80, err := j.ReadHeaderAt(h)
			if err != nil {
				continue
			}
			id := pow.BlockHashLE(h80)
			if !raw.Has(id) {
				rawMiss++
				continue
			}
			payload, err := raw.Get(id)
			if err != nil {
				rawMiss++
				continue
			}
			rawHits++
			scanErr := wire.ForEachBlockTx(payload, func(ti uint32, tx *wire.Tx) error {
				txid := mempool.TxIDDisplayHex(tx.TxHash())
				if len(hits) < addrScanMaxHits {
					for vi, o := range tx.Vout {
						if !scriptPubKeyMatchesAddress(o.PkScript, address, pubkeyVer, scriptHashVer) {
							continue
						}
						hits = append(hits, AddrTxHit{
							Height:     h,
							TxIndex:    int(ti),
							TxID:       txid,
							Vout:       vi,
							ValueKoinu: o.Value,
						})
						totalKoinu += o.Value
						if len(hits) >= addrScanMaxHits {
							break
						}
					}
				}
				if len(spends) < addrScanMaxHits && (txIx != nil || raw != nil) {
					for vi, in := range tx.Vin {
						if isCoinbaseVin(in) {
							continue
						}
						prevTxid := mempool.TxIDDisplayHex(in.PrevHash)
						val, spk, ok := resolvePrevoutVout(txIx, raw, prevTxid, in.PrevIdx)
						if !ok {
							continue
						}
						a := chain.ScriptPubKeyAddress(spk, pubkeyVer, scriptHashVer)
						if a == "" || !strings.EqualFold(strings.TrimSpace(a), address) {
							if !scriptPubKeyMatchesAddress(spk, address, pubkeyVer, scriptHashVer) {
								continue
							}
						}
						spends = append(spends, AddrSpendHit{
							Height:     h,
							TxIndex:    int(ti),
							TxID:       txid,
							Vin:        vi,
							ValueKoinu: val,
							PrevTxID:   prevTxid,
							PrevVout:   int(in.PrevIdx),
						})
						totalSpentKoinu += val
						if len(spends) >= addrScanMaxHits {
							break
						}
					}
				}
				return nil
			})
			if scanErr != nil {
				parseErrs++
			}
		}
	}
	scanMempoolOutputsForAddress(&hits, pool, address, pubkeyVer, scriptHashVer, &totalKoinu)
	mergeHintOutpoint(&hits, j, raw, txIx, pool, hintTxid, hintVout, address, pubkeyVer, scriptHashVer, &totalKoinu)
	linkOutputSpends(hits, spends)
	chainActive := rpc.ActiveChainBlockHeight(j, raw)
	out := map[string]any{
		"address":                     address,
		"scan_from_height":            scanTip,
		"scan_to_height":              minH,
		"chain_active_height":         chainActive,
		"heights_considered":          len(heights),
		"raw_blocks_scanned":          rawHits,
		"raw_blocks_missing":          rawMiss,
		"raw_parse_errors":            parseErrs,
		"matching_outputs":            hits,
		"matching_output_count":       len(hits),
		"matching_spends":             spends,
		"matching_spend_count":      len(spends),
		"total_received_koinu_window": totalKoinu,
		"total_received_doge_window":  float64(totalKoinu) / 1e8,
		"total_spent_koinu_window":    totalSpentKoinu,
		"total_spent_doge_window":     float64(totalSpentKoinu) / 1e8,
		"dogego_note":                 "Received outputs from the last " + fmt.Sprintf("%d", addrScanMaxRawBlocks) + " stored heights plus unspent UTXOs at chain tip. Spends need the tx index for input values.",
	}
	if len(hits) == 0 && len(spends) == 0 {
		out["dogego_empty_reason"] = fmt.Sprintf("No matching outputs in heights %d-%d (%d blocks on disk, %d missing). Parent txs for mempool orphans are not searchable until those blocks sync.", minH, scanTip, rawHits, rawMiss)
	}
	if len(hits) >= addrScanMaxHits || len(spends) >= addrScanMaxHits {
		out["truncated"] = true
	}
	if skipRawScan {
		out["utxo_fast_path"] = true
		out["dogego_note"] = "Unspent outputs from UTXO cache (solo wallet fast path); recent block scan skipped."
	}
	return out, nil
}

func isCoinbaseVin(in wire.TxIn) bool {
	var z [32]byte
	return in.PrevHash == z && in.PrevIdx == 0xffffffff
}

func linkOutputSpends(hits []AddrTxHit, spends []AddrSpendHit) {
	if len(hits) == 0 || len(spends) == 0 {
		return
	}
	byOut := make(map[string]AddrSpendHit, len(spends))
	for _, s := range spends {
		k := strings.ToLower(strings.TrimSpace(s.PrevTxID)) + ":" + fmt.Sprintf("%d", s.PrevVout)
		byOut[k] = s
	}
	for i := range hits {
		k := strings.ToLower(strings.TrimSpace(hits[i].TxID)) + ":" + fmt.Sprintf("%d", hits[i].Vout)
		if s, ok := byOut[k]; ok {
			hits[i].SpendTxID = s.TxID
			hits[i].SpendVin = s.Vin
			hits[i].SpendHeight = s.Height
		}
	}
}


// spendScanHeightRange returns [minH, scanTip] for searching spends after txBlockHeight.
// contiguousHint caps scanTip to stored bodies without scanning headers (avoids O(window) per lookup).
func spendScanHeightRange(j *store.HeaderJournal, raw *store.RawBlockStore, contiguousHint, txBlockHeight int64) (scanTip, minH int64) {
	if j == nil || raw == nil {
		return -1, 0
	}
	headerTip, err := j.TipHeight()
	if err != nil {
		return -1, 0
	}
	if contiguousHint >= 0 {
		scanTip = contiguousHint
	} else {
		scanTip, _ = addressScanHeightRange(j, raw, headerTip)
	}
	if txBlockHeight >= 0 {
		minH = txBlockHeight + 1
		if minH > scanTip {
			return scanTip, scanTip + 1 // empty range
		}
		return scanTip, minH
	}
	_, minH = addressScanHeightRange(j, raw, headerTip)
	if contiguousHint >= 0 && scanTip > contiguousHint {
		scanTip = contiguousHint
	}
	return scanTip, minH
}

// OutpointSpendResult is the spend of one prevout (when found).
type OutpointSpendResult struct {
	SpendTxid string
	SpendVin  int
	Height    int64
	Found     bool
}

// FindSpendsFromIndexOnly resolves spends via the outspend index only (no block scan).
// BlockStep uses this on the hot path so tx pages stay instant during IBD.
func FindSpendsFromIndexOnly(addrIx *store.AddrIndex, prevTxid string, vouts []int) map[int]OutpointSpendResult {
	results := make(map[int]OutpointSpendResult, len(vouts))
	if addrIx == nil || len(vouts) == 0 {
		return results
	}
	prevTxid = strings.ToLower(strings.TrimSpace(prevTxid))
	if prevTxid == "" {
		return results
	}
	for _, v := range vouts {
		if v < 0 {
			continue
		}
		if hit, ok := addrIx.LookupOutpointSpend(prevTxid, v); ok {
			results[v] = OutpointSpendResult{
				SpendTxid: hit.TxID,
				SpendVin:  int(hit.Vin),
				Height:    hit.Height,
				Found:     true,
			}
		}
	}
	return results
}

// FindSpendsForOutpoints resolves spends for many vouts of prevTxid using the outspend index
// and at most one backward block scan (not one scan per vout).
func FindSpendsForOutpoints(addrIx *store.AddrIndex, j *store.HeaderJournal, raw *store.RawBlockStore, prevTxid string, vouts []int, contiguousHint, txBlockHeight int64) map[int]OutpointSpendResult {
	results := make(map[int]OutpointSpendResult, len(vouts))
	prevTxid = strings.ToLower(strings.TrimSpace(prevTxid))
	if prevTxid == "" || j == nil || raw == nil || len(vouts) == 0 {
		return results
	}
	want := map[int]struct{}{}
	for _, v := range vouts {
		if v < 0 {
			continue
		}
		if addrIx != nil {
			if hit, ok := addrIx.LookupOutpointSpend(prevTxid, v); ok {
				results[v] = OutpointSpendResult{SpendTxid: hit.TxID, SpendVin: int(hit.Vin), Height: hit.Height, Found: true}
				continue
			}
		}
		want[v] = struct{}{}
	}
	if len(want) == 0 {
		return results
	}
	scanTip, minH := spendScanHeightRange(j, raw, contiguousHint, txBlockHeight)
	if scanTip < minH {
		return results
	}
	for h := scanTip; h >= minH && len(want) > 0; h-- {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			continue
		}
		id := pow.BlockHashLE(h80)
		if !raw.Has(id) {
			continue
		}
		payload, err := raw.Get(id)
		if err != nil {
			continue
		}
		_ = wire.ForEachBlockTx(payload, func(_ uint32, tx *wire.Tx) error {
			if len(want) == 0 {
				return nil
			}
			for vi, in := range tx.Vin {
				if isCoinbaseVin(in) {
					continue
				}
				if mempool.TxIDDisplayHex(in.PrevHash) != prevTxid {
					continue
				}
				v := int(in.PrevIdx)
				if _, ok := want[v]; !ok {
					continue
				}
				results[v] = OutpointSpendResult{
					SpendTxid: mempool.TxIDDisplayHex(tx.TxHash()),
					SpendVin:  vi,
					Height:    h,
					Found:     true,
				}
				delete(want, v)
			}
			return nil
		})
	}
	return results
}

// FindSpendForOutpoint scans stored blocks for a vin spending prevTxid:prevVout.
func FindSpendForOutpoint(j *store.HeaderJournal, raw *store.RawBlockStore, prevTxid string, prevVout int) (spendTxid string, spendVin int, height int64, ok bool) {
	res := FindSpendsForOutpoints(nil, j, raw, prevTxid, []int{prevVout}, -1, -1)
	if hit, ok := res[prevVout]; ok && hit.Found {
		return hit.SpendTxid, hit.SpendVin, hit.Height, true
	}
	return "", 0, -1, false
}

// addressScanHeightRange returns [minH, scanTip] covering chainActive and stored bodies ahead during IBD.
func addressScanHeightRange(j *store.HeaderJournal, raw *store.RawBlockStore, headerTip int64) (scanTip, minH int64) {
	chainActive := rpc.ActiveChainBlockHeight(j, raw)
	scanBase := chainActive
	if scanBase < 0 {
		scanBase = headerTip
	}
	scanTip = scanBase
	if raw != nil && j != nil && scanBase >= 0 {
		maxUp := scanBase + addrScanMaxRawBlocks
		if maxUp > headerTip {
			maxUp = headerTip
		}
		for h := scanBase + 1; h <= maxUp; h++ {
			h80, err := j.ReadHeaderAt(h)
			if err != nil {
				break
			}
			if raw.Has(pow.BlockHashLE(h80)) {
				scanTip = h
			}
		}
	}
	window := int64(addrScanMaxRawBlocks)
	if chainActive >= 0 && chainActive+1 < window {
		window = chainActive + 1
	}
	minH = 0
	if scanTip >= window {
		minH = scanTip - window + 1
	}
	return scanTip, minH
}

func normalizeScanAddress(address string, pubVer, scriptVer byte) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	payload, err := chain.DecodeBase58CheckBytes(address)
	if err == nil && len(payload) == 21 {
		ver := payload[0]
		if ver == pubVer || ver == scriptVer {
			var h20 [20]byte
			copy(h20[:], payload[1:])
			return chain.Base58CheckEncode(ver, h20[:])
		}
	}
	return address
}

func resolvePrevoutVout(txIx *store.TxIndex, raw *store.RawBlockStore, prevTxid string, prevVout uint32) (value int64, pkScript []byte, ok bool) {
	if txIx != nil && raw != nil {
		if val, spk, ok := store.LoadIndexedTxVout(txIx, raw, prevTxid, prevVout); ok {
			return val, spk, true
		}
		if tx, err := store.LoadIndexedTx(txIx, raw, prevTxid); err == nil && int(prevVout) < len(tx.Vout) {
			o := tx.Vout[prevVout]
			return o.Value, append([]byte(nil), o.PkScript...), true
		}
	}
	return 0, nil, false
}

func mergeUtxoOutputsForAddress(hits *[]AddrTxHit, utxoFn func() *store.UtxoCache, address string, pubVer, scriptVer byte, totalKoinu *int64) {
	if utxoFn == nil || hits == nil {
		return
	}
	utxo := utxoFn()
	if utxo == nil {
		return
	}
	scriptSet := addressPkScriptSet(address, pubVer, scriptVer)
	if len(scriptSet) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(*hits))
	for _, h := range *hits {
		seen[outpointKey(h.TxID, h.Vout)] = struct{}{}
	}
	rows := utxo.FilterRowsByScriptSet(scriptSet, addrScanMaxHits)
	for _, row := range rows {
		k := outpointKey(row.TxID, int(row.Vout))
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		*hits = append(*hits, AddrTxHit{
			Height:     row.Height,
			TxID:       row.TxID,
			Vout:       int(row.Vout),
			ValueKoinu: row.Value,
		})
		*totalKoinu += row.Value
		if len(*hits) >= addrScanMaxHits {
			break
		}
	}
}

func addressPkScriptSet(address string, pubVer, scriptVer byte) map[string]struct{} {
	payload, err := chain.DecodeBase58CheckBytes(strings.TrimSpace(address))
	if err != nil || len(payload) != 21 {
		if h, ok := store.Hash160FromAddress(address); ok {
			var h20 [20]byte
			copy(h20[:], h[:])
			return map[string]struct{}{string(chain.P2PKHScriptFromPubKeyHash(h20)): {}}
		}
		return nil
	}
	var h20 [20]byte
	copy(h20[:], payload[1:])
	set := make(map[string]struct{}, 2)
	switch payload[0] {
	case pubVer:
		set[string(chain.P2PKHScriptFromPubKeyHash(h20))] = struct{}{}
	case scriptVer:
		set[string(chain.P2SHScriptFromScriptHash(h20))] = struct{}{}
	default:
		set[string(chain.P2PKHScriptFromPubKeyHash(h20))] = struct{}{}
	}
	return set
}

func outpointKey(txid string, vout int) string {
	return strings.ToLower(strings.TrimSpace(txid)) + ":" + strconv.Itoa(vout)
}

func scriptPubKeyMatchesAddress(pkScript []byte, want string, pubkeyVer, scriptVer byte) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, a := range scriptPubKeyAddresses(pkScript, pubkeyVer, scriptVer) {
		if a == want {
			return true
		}
	}
	return false
}

func scriptPubKeyAddresses(pkScript []byte, pubkeyVer, scriptVer byte) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(a string) {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			return
		}
		if _, ok := seen[a]; ok {
			return
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	add(chain.ScriptPubKeyAddress(pkScript, pubkeyVer, scriptVer))
	if len(pkScript) >= 3 && pkScript[len(pkScript)-1] == 0xac {
		var pub []byte
		switch pkScript[0] {
		case 0x21:
			if len(pkScript) == 35 {
				pub = pkScript[1:33]
			}
		case 0x41:
			if len(pkScript) == 67 {
				pub = pkScript[1:65]
			}
		}
		if len(pub) > 0 {
			h := chain.Hash160(pub)
			var h20 [20]byte
			copy(h20[:], h)
			add(chain.Base58CheckEncode(pubkeyVer, h20[:]))
		}
	}
	return out
}

func scanMempoolOutputsForAddress(hits *[]AddrTxHit, pool *mempool.Pool, address string, pubVer, scriptVer byte, totalKoinu *int64) {
	if pool == nil {
		return
	}
	entries, err := pool.SortedMemPoolVerbose()
	if err != nil {
		return
	}
	for _, e := range entries {
		rawTx, err := pool.GetRawByTxID(e.TxID)
		if err != nil || len(rawTx) == 0 {
			continue
		}
		tx, err := wire.DeserializeTx(rawTx)
		if err != nil {
			continue
		}
		txid := mempool.TxIDDisplayHex(tx.TxHash())
		for vi, o := range tx.Vout {
			if !scriptPubKeyMatchesAddress(o.PkScript, address, pubVer, scriptVer) {
				continue
			}
			*hits = append(*hits, AddrTxHit{
				Height:     -1,
				TxIndex:    -1,
				TxID:       txid,
				Vout:       vi,
				ValueKoinu: o.Value,
			})
			*totalKoinu += o.Value
			if len(*hits) >= addrScanMaxHits {
				return
			}
		}
	}
}

func mergeHintOutpoint(hits *[]AddrTxHit, j *store.HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, pool *mempool.Pool, hintTxid string, hintVout int, address string, pubVer, scriptVer byte, totalKoinu *int64) {
	hintTxid = strings.ToLower(strings.TrimSpace(hintTxid))
	if hintTxid == "" || hintVout < 0 {
		return
	}
	for _, h := range *hits {
		if strings.EqualFold(h.TxID, hintTxid) && h.Vout == hintVout {
			return
		}
	}
	jm, rawTx, _, err := rpc.LookupTxExplorer(txIx, raw, pool, hintTxid)
	if err != nil || jm == nil {
		return
	}
	var tx *wire.Tx
	if len(rawTx) > 0 {
		tx, _ = wire.DeserializeTx(rawTx)
	}
	if tx == nil {
		return
	}
	if hintVout >= len(tx.Vout) {
		return
	}
	o := tx.Vout[hintVout]
	if !scriptPubKeyMatchesAddress(o.PkScript, address, pubVer, scriptVer) {
		return
	}
	height := int64(-1)
	if h, ok := jm["height"].(float64); ok {
		height = int64(h)
	} else if h, ok := jm["height"].(int64); ok {
		height = h
	} else if bh, ok := jm["blockhash"].(string); ok && j != nil && raw != nil {
		height = blockHeightForHash(j, raw, bh)
	}
	*hits = append([]AddrTxHit{{
		Height:     height,
		TxID:       hintTxid,
		Vout:       hintVout,
		ValueKoinu: o.Value,
	}}, *hits...)
	*totalKoinu += o.Value
}

func blockHeightForHash(j *store.HeaderJournal, raw *store.RawBlockStore, blockHash string) int64 {
	if j == nil {
		return -1
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return -1
	}
	want := strings.ToLower(strings.TrimSpace(blockHash))
	lo, hi := int64(0), tip
	for lo <= hi {
		mid := (lo + hi) / 2
		h80, err := j.ReadHeaderAt(mid)
		if err != nil {
			return -1
		}
		got := strings.ToLower(pow.BlockHashHex(h80))
		if got == want {
			return mid
		}
		if got < want {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return -1
}
