// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"dogego/chain"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func headerFieldsJSON(h80 []byte, height int64) map[string]any {
	t := int64(binary.LittleEndian.Uint32(h80[68:72]))
	bits := binary.LittleEndian.Uint32(h80[72:76])
	return map[string]any{
		"height":     height,
		"hash":       pow.BlockHashHex(h80),
		"time":       t,
		"bits":       fmt.Sprintf("%08x", bits),
		"nonce":      binary.LittleEndian.Uint32(h80[76:80]),
		"version":    int32(binary.LittleEndian.Uint32(h80[0:4])),
		"merkleroot": pow.LEUint256DisplayHex(h80[36:68]),
	}
}

// LookupHeaderForAPI resolves ?height=N or ?hash=64hex (hash takes precedence if both set).
func LookupHeaderForAPI(j *store.HeaderJournal, heightStr, hashStr string) (map[string]any, int, string) {
	hashStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hashStr), "0x"))
	heightStr = strings.TrimSpace(heightStr)
	if j == nil {
		return nil, 503, "header journal unavailable"
	}
	if hashStr != "" {
		if len(hashStr) != 64 {
			return nil, 400, "hash must be 64 hex characters"
		}
		h, err := j.HeightByDisplayHash(hashStr)
		if err != nil {
			return nil, 404, err.Error()
		}
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return nil, 404, err.Error()
		}
		return headerFieldsJSON(h80, h), 0, ""
	}
	if heightStr == "" {
		return nil, 400, "provide height= or hash="
	}
	hv, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil || hv < 0 {
		return nil, 400, "invalid height"
	}
	h80, err := j.ReadHeaderAt(hv)
	if err != nil {
		return nil, 404, err.Error()
	}
	return headerFieldsJSON(h80, hv), 0, ""
}

func isCoinbaseTx(t *wire.Tx) bool {
	if len(t.Vin) != 1 {
		return false
	}
	if t.Vin[0].PrevIdx != 0xffffffff {
		return false
	}
	var z [32]byte
	return t.Vin[0].PrevHash == z
}

// rawBlockStoreKey looks up a stored .bin under rawblocks/. DogeGo normally names files hex(BlockHashLE(header))
// (wire / P2P block id). Files copied from explorers or some tools are often named with the display-order hash
// (hex byte-reversed), which is what BlockHashHex shows - try both before reporting missing.
func rawBlockStoreKey(raw *store.RawBlockStore, h80 []byte) (key [32]byte, found bool, usedDisplayName bool) {
	if raw == nil {
		return key, false, false
	}
	want := pow.BlockHashLE(h80)
	if raw.Has(want) {
		return want, true, false
	}
	var alt [32]byte
	for i := 0; i < 32; i++ {
		alt[i] = want[31-i]
	}
	if raw.Has(alt) {
		return alt, true, true
	}
	return key, false, false
}

func applyContiguousBlockHints(out map[string]any, hv, contiguousHint int64) {
	if contiguousHint < 0 {
		return
	}
	out["contiguous_body_height"] = contiguousHint
	out["connected_to_chain"] = hv <= contiguousHint
	if hv > contiguousHint {
		out["dogego_sync_note"] = "raw block stored but not yet connected (forward IBD still filling heights below this block)"
	}
}

// LookupBlockForAPI resolves height or hash; includes raw block summary when present on disk.
// contiguousHint is the cached highest height with raw bodies for every height in [0,h] (-1 if unknown).
// Callers must pass contiguousHeightForAPI(cfg) instead of scanning genesis→tip on each lookup.
func LookupBlockForAPI(j *store.HeaderJournal, raw *store.RawBlockStore, addrVer byte, heightStr, hashStr string, contiguousHint int64) (map[string]any, int, string) {
	hashStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hashStr), "0x"))
	heightStr = strings.TrimSpace(heightStr)
	if j == nil {
		return nil, 503, "header journal unavailable"
	}
	var hv int64
	var h80 []byte
	var err error
	if hashStr != "" {
		if len(hashStr) != 64 {
			return nil, 400, "hash must be 64 hex characters"
		}
		hv, err = j.HeightByDisplayHash(hashStr)
		if err != nil {
			return nil, 404, err.Error()
		}
		h80, err = j.ReadHeaderAt(hv)
		if err != nil {
			return nil, 404, err.Error()
		}
	} else if heightStr != "" {
		hv, err = strconv.ParseInt(heightStr, 10, 64)
		if err != nil || hv < 0 {
			return nil, 400, "invalid height"
		}
		h80, err = j.ReadHeaderAt(hv)
		if err != nil {
			return nil, 404, err.Error()
		}
	} else {
		return nil, 400, "provide height= or hash="
	}
	out := headerFieldsJSON(h80, hv)
	want := pow.BlockHashLE(h80)
	out["hash_le_hex"] = fmt.Sprintf("%x", want)
	out["hash_display_hex"] = pow.BlockHashHex(h80)
	if raw == nil {
		out["has_raw_block"] = false
		out["dogego_note"] = "no raw block store (SPV or blocks not enabled)"
		return out, 0, ""
	}
	key, ok, usedDisplay := rawBlockStoreKey(raw, h80)
	if !ok {
		out["has_raw_block"] = false
		out["dogego_note"] = "no rawblocks/*.bin for this header (expected filename hex is hash_le_hex; explorer-style block hash is hash_display_hex)"
		return out, 0, ""
	}
	if usedDisplay {
		out["dogego_raw_bin_filename"] = "display_order_hex"
	}
	payload, err := raw.Get(key)
	if err != nil {
		out["has_raw_block"] = false
		out["raw_error"] = err.Error()
		return out, 0, ""
	}
	out["has_raw_block"] = true
	out["raw_size_bytes"] = len(payload)
	applyContiguousBlockHints(out, hv, contiguousHint)
	nTx, err := wire.BlockTxCount(payload)
	if err != nil {
		out["parse_error"] = err.Error()
		return out, 0, ""
	}
	out["tx_count"] = int(nTx)
	cb, _, err := wire.ReadTxAtIndex(payload, 0)
	if err == nil && isCoinbaseTx(cb) {
		var sumKoinu int64
		var pays []string
		seen := map[string]struct{}{}
		for _, o := range cb.Vout {
			sumKoinu += o.Value
			a := chain.PayToPubKeyHashAddress(o.PkScript, addrVer)
			if a != "" {
				pays = append(pays, a)
				seen[a] = struct{}{}
			}
		}
		out["coinbase_total_koinu"] = sumKoinu
		out["coinbase_payout_addresses"] = pays
		out["coinbase_unique_p2pkh"] = len(seen)
	}
	return out, 0, ""
}

const maxBlockTxListAPI = 200
const blockTxDefaultPage = 40

// ListBlockTransactionsPage returns a slice of transaction summaries from a stored block.
func ListBlockTransactionsPage(j *store.HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, networkSlug string, height int64, offset, limit int) (txs []map[string]any, total int, truncated bool, errMsg string) {
	if limit <= 0 {
		limit = blockTxDefaultPage
	}
	if limit > maxBlockTxListAPI {
		limit = maxBlockTxListAPI
	}
	if offset < 0 {
		offset = 0
	}
	if j == nil || raw == nil {
		return nil, 0, false, "no raw block store"
	}
	h80, err := j.ReadHeaderAt(height)
	if err != nil {
		return nil, 0, false, err.Error()
	}
	key, ok, _ := rawBlockStoreKey(raw, h80)
	if !ok {
		return nil, 0, false, "block body not on disk"
	}
	payload, err := raw.Get(key)
	if err != nil {
		return nil, 0, false, err.Error()
	}
	nTx, err := wire.BlockTxCount(payload)
	if err != nil {
		return nil, 0, false, err.Error()
	}
	total = int(nTx)
	pubVer, scriptVer := addrVersionsForNetwork(networkSlug)
	var skipped int
	_ = wire.ForEachBlockTx(payload, func(ti uint32, tx *wire.Tx) error {
		if int(skipped) < offset {
			skipped++
			return nil
		}
		if len(txs) >= limit {
			truncated = total > offset+len(txs)
			return nil
		}
		txid := mempool.TxIDDisplayHex(tx.TxHash())
		var sum int64
		var addrs []string
		for _, o := range tx.Vout {
			sum += o.Value
			if a := chain.ScriptPubKeyAddress(o.PkScript, pubVer, scriptVer); a != "" {
				addrs = append(addrs, a)
			}
		}
		entry := map[string]any{
			"index":       ti,
			"txid":        txid,
			"is_coinbase": isCoinbaseTx(tx),
			"vout_count":  len(tx.Vout),
			"vin_count":   len(tx.Vin),
			"total_doge":  float64(sum) / 1e8,
		}
		if len(addrs) > 0 {
			entry["sample_addresses"] = addrs
		}
		if txIx != nil {
			_, err := store.LoadIndexedTx(txIx, raw, txid)
			entry["indexed"] = err == nil
		} else {
			entry["indexed"] = false
		}
		txs = append(txs, entry)
		return nil
	})
	if txs == nil {
		txs = []map[string]any{}
	}
	if offset+len(txs) < total {
		truncated = true
	}
	return txs, total, truncated, ""
}

// ListBlockTransactions decodes transaction summaries from a stored raw block at height.
func ListBlockTransactions(j *store.HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, networkSlug string, height int64) ([]map[string]any, bool, string) {
	txs, _, truncated, errMsg := ListBlockTransactionsPage(j, raw, txIx, networkSlug, height, 0, maxBlockTxListAPI)
	return txs, truncated, errMsg
}

func addrVersionsForNetwork(networkSlug string) (byte, byte) {
	net, err := networkFromUISlug(networkSlug)
	if err != nil {
		return 30, 22
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return 30, 22
	}
	return p.PubkeyHashAddrID, p.ScriptHashAddrID
}

func attachBlockTransactions(out map[string]any, j *store.HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, networkSlug string, height int64) {
	blk, _ := out["block"].(map[string]any)
	if blk == nil {
		return
	}
	hasRaw, _ := blk["has_raw_block"].(bool)
	if !hasRaw {
		out["transactions"] = []any{}
		out["transactions_note"] = "Block header is known but the body is not stored on this node yet - sync more blocks to list transactions."
		return
	}
	txs, truncated, errMsg := ListBlockTransactions(j, raw, txIx, networkSlug, height)
	out["transactions"] = txs
	if truncated {
		out["transactions_truncated"] = true
	}
	if errMsg != "" && len(txs) == 0 {
		out["transactions_note"] = errMsg
	}
}
