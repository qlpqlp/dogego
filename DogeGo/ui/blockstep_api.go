// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
	"dogego/wire"
)

const blockstepMaxTimelinePoints = 48

// BlockStepMeta is returned by GET /api/blockstep/meta.
func BlockStepMeta(cfg StartConfig) (map[string]any, int, string) {
	j := cfg.Journal
	if j == nil {
		return nil, 503, "header journal unavailable"
	}
	tip, err := j.TipHeight()
	if err != nil {
		return nil, 503, err.Error()
	}
	genesisTS := int64(1386325540)
	if net, err := networkFromUISlug(cfg.Network); err == nil {
		if p, err := chain.ParamsFor(net); err == nil {
			genesisTS = int64(p.GenesisTime)
		}
	}
	var tipTime int64
	if h80, err := j.ReadHeaderAt(tip); err == nil {
		tipTime = headerTimeUnix(h80)
	}
	contig := contiguousHeightForAPI(cfg)
	chainActive := chainActiveHeightForAPI(cfg, tip)
	navHeight := blockstepNavigableHeight(tip, contig, chainActive, cfg.RawBlocks != nil)
	var navTime int64
	if navHeight >= 0 {
		if h80, err := j.ReadHeaderAt(navHeight); err == nil {
			navTime = headerTimeUnix(h80)
		}
	}
	timelineEnd := time.Now().Unix()
	if navTime > 0 && navHeight < tip {
		timelineEnd = navTime
	} else if tipTime > 0 {
		timelineEnd = tipTime
	}
	out := map[string]any{
		"network":              cfg.Network,
		"header_tip_height":    tip,
		"header_tip_time":      tipTime,
		"chain_active_height":  chainActive,
		"contiguous_bodies":    contig,
		"navigable_height":     navHeight,
		"navigable_time":       navTime,
		"genesis_time":         genesisTS,
		"timeline_start":       genesisTS,
		"timeline_end":         timelineEnd,
		"timeline_start_label": "Dec 2013",
		"has_raw_blocks":       cfg.RawBlocks != nil,
		"has_tx_index":         cfg.TxIndex != nil,
		"syncing":              cfg.RawBlocks != nil && contig >= 0 && contig < tip,
		"indexing_note":        blockstepIndexingNote(cfg.RawBlocks, cfg.TxIndex, contig, tip, navHeight),
	}
	return out, 0, ""
}

func blockstepNavigableHeight(headerTip, contig, chainActive int64, hasRaw bool) int64 {
	if !hasRaw {
		if headerTip >= 0 {
			return headerTip
		}
		return 0
	}
	nav := int64(-1)
	if contig >= 0 {
		nav = contig
	} else if chainActive >= 0 {
		nav = chainActive
	}
	if nav < 0 {
		nav = 0
	}
	if headerTip >= 0 && nav > headerTip {
		nav = headerTip
	}
	return nav
}

func blockstepIndexingNote(raw *store.RawBlockStore, txIx *store.TxIndex, contig, tip, navHeight int64) string {
	if raw == nil {
		return "SPV mode - headers only; block bodies and transaction walks need a full node."
	}
	if contig >= 0 && contig < tip {
		return fmt.Sprintf("Block bodies through height %d are on this node - slide and open blocks in that range while sync continues.", navHeight)
	}
	if txIx == nil {
		return "Transaction index is off - enable tx index in Settings to jump between txs."
	}
	return "Local chain data ready for BlockStep walks."
}

func headerTimeUnix(h80 []byte) int64 {
	if len(h80) < 72 {
		return 0
	}
	return int64(binary.LittleEndian.Uint32(h80[68:72]))
}

// HeightAtUnix returns the greatest header height with time <= ts.
func HeightAtUnix(j *store.HeaderJournal, ts int64) (int64, error) {
	if j == nil {
		return 0, fmt.Errorf("no header journal")
	}
	tip, err := j.TipHeight()
	if err != nil {
		return 0, err
	}
	lo, hi := int64(0), tip
	for lo < hi {
		mid := (lo + hi + 1) / 2
		h80, err := j.ReadHeaderAt(mid)
		if err != nil {
			return 0, err
		}
		if headerTimeUnix(h80) <= ts {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo, nil
}

// BlockStepTimeline returns sampled blocks between two unix timestamps.
func BlockStepTimeline(j *store.HeaderJournal, raw *store.RawBlockStore, fromTS, toTS int64, points int) (map[string]any, int, string) {
	if j == nil {
		return nil, 503, "header journal unavailable"
	}
	if points <= 0 || points > blockstepMaxTimelinePoints {
		points = 24
	}
	if toTS <= 0 {
		toTS = time.Now().Unix()
	}
	if fromTS <= 0 || fromTS > toTS {
		fromTS = toTS - 86400*30
	}
	h0, err := HeightAtUnix(j, fromTS)
	if err != nil {
		return nil, 500, err.Error()
	}
	h1, err := HeightAtUnix(j, toTS)
	if err != nil {
		return nil, 500, err.Error()
	}
	if h1 < h0 {
		h0, h1 = h1, h0
	}
	var samples []map[string]any
	step := int64(1)
	if h1 > h0 {
		step = (h1 - h0) / int64(points-1)
		if step < 1 {
			step = 1
		}
	}
	for h := h0; h <= h1; h += step {
		if s, code, msg := blockstepHeaderSample(j, raw, h); code == 0 {
			samples = append(samples, s)
		} else if msg != "" && len(samples) == 0 {
			return nil, code, msg
		}
		if len(samples) >= points {
			break
		}
	}
	if len(samples) == 0 || samples[len(samples)-1]["height"] != h1 {
		if s, _, _ := blockstepHeaderSample(j, raw, h1); s != nil {
			samples = append(samples, s)
		}
	}
	return map[string]any{
		"from_time":    fromTS,
		"to_time":      toTS,
		"height_start": h0,
		"height_end":   h1,
		"points":       samples,
	}, 0, ""
}

func blockstepHeaderSample(j *store.HeaderJournal, raw *store.RawBlockStore, h int64) (map[string]any, int, string) {
	h80, err := j.ReadHeaderAt(h)
	if err != nil {
		return nil, 404, err.Error()
	}
	t := headerTimeUnix(h80)
	hasRaw := false
	if raw != nil {
		if key, ok, _ := rawBlockStoreKey(raw, h80); ok {
			hasRaw = raw.Has(key)
		}
	}
	return map[string]any{
		"height":       h,
		"time":         t,
		"hash":         pow.BlockHashHex(h80),
		"has_raw_body": hasRaw,
	}, 0, ""
}

// BlockStepBlockDetail returns block header + transaction list for BlockStep UI.
func BlockStepBlockDetail(j *store.HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, networkSlug string, heightStr string, txOffset, txLimit int, contiguousHint int64) (map[string]any, int, string) {
	heightStr = strings.TrimSpace(heightStr)
	if heightStr == "" {
		return nil, 400, "missing height"
	}
	hv, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil || hv < 0 {
		return nil, 400, "invalid height"
	}
	pubVer, _ := blockstepAddrVersions(networkSlug)
	base, code, msg := LookupBlockForAPI(j, raw, pubVer, heightStr, "", contiguousHint)
	if code != 0 {
		return nil, code, msg
	}
	out := map[string]any{"block": base}
	status := blockstepAvailability(base, txIx)
	out["availability"] = status

	hasRaw, _ := base["has_raw_block"].(bool)
	if !hasRaw {
		out["transactions"] = []any{}
		out["dogego_blockstep_note"] = "Block body not on this node yet - header is here, transactions arrive with sync."
		return out, 0, ""
	}
	txs, txTotal, truncated, errMsg := ListBlockTransactionsPage(j, raw, txIx, networkSlug, hv, txOffset, txLimit)
	out["transactions"] = txs
	out["transaction_count"] = txTotal
	out["transaction_offset"] = txOffset
	out["transaction_limit"] = txLimit
	if truncated {
		out["truncated"] = true
		out["has_more_tx"] = txOffset+len(txs) < txTotal
	}
	if errMsg != "" && len(txs) == 0 {
		out["raw_error"] = errMsg
	}
	return out, 0, ""
}

func blockstepAddrVersions(networkSlug string) (byte, byte) {
	return addrVersionsForNetwork(networkSlug)
}

func blockstepAvailability(block map[string]any, txIx *store.TxIndex) map[string]any {
	hasRaw, _ := block["has_raw_block"].(bool)
	return map[string]any{
		"header":       true,
		"body":         hasRaw,
		"tx_index":     txIx != nil,
		"can_walk_tx":  hasRaw && txIx != nil,
		"status":       blockstepStatusLabel(hasRaw, txIx != nil),
		"icon":         blockstepStatusIcon(hasRaw, txIx != nil),
	}
}

func blockstepStatusLabel(hasRaw, hasIdx bool) string {
	if hasRaw && hasIdx {
		return "ready"
	}
	if hasRaw {
		return "partial"
	}
	return "headers_only"
}

func blockstepStatusIcon(hasRaw, hasIdx bool) string {
	if hasRaw && hasIdx {
		return "verified"
	}
	if hasRaw {
		return "layers"
	}
	return "hdr_weak"
}

const blockstepMaxSpendVouts = 128

// BlockStepTxDetail returns navigable transaction view for BlockStep.
func BlockStepTxDetail(networkSlug string, j *store.HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, addrIx *store.AddrIndex, pool *mempool.Pool, txid string) (map[string]any, int, string) {
	txid = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(txid), "0x"))
	if txid == "" {
		return nil, 400, "missing txid"
	}
	jm, rawTx, src, err := rpc.LookupTxExplorer(txIx, raw, pool, txid)
	if err != nil {
		return map[string]any{
			"found":        false,
			"txid":         txid,
			"availability": map[string]any{"status": "missing", "icon": "search_off", "message": "Transaction not found on this node yet. It may be unindexed, unsynced, or only on the wider network."},
			"dogego_tip":   "Try again after sync progresses, or search a more recent block.",
		}, 0, ""
	}
	if rawTx != nil {
		if tx, derr := wire.DeserializeTx(rawTx); derr == nil {
			if chainName, cerr := blockstepRPCChainName(networkSlug); cerr == nil {
				if enriched, jerr := rpc.TxToRPCJSONChain(tx, chainName); jerr == nil {
					if src == "chain" {
						if bh, ok := jm["blockhash"].(string); ok {
							enriched["blockhash"] = bh
						}
						if ti, ok := jm["tx_index"]; ok {
							enriched["tx_index"] = ti
						}
					}
					if src == "mempool" {
						enriched["confirmations"] = int64(0)
						enriched["dogego_note"] = "unconfirmed (local mempool)"
					}
					jm = enriched
				}
			}
		}
	}
	pubVer, scriptVer := blockstepAddrVersions(networkSlug)
	inputs := blockstepVinNav(jm, txIx, raw, txIx != nil && raw != nil)
	outputs := blockstepVoutNav(jm, pubVer, scriptVer)
	if txIx != nil && raw != nil && j != nil && src == "chain" && addrIx != nil {
		vouts := make([]int, 0, len(outputs))
		for i := range outputs {
			if i >= blockstepMaxSpendVouts {
				break
			}
			if vout, ok := outputs[i]["index"].(int); ok {
				vouts = append(vouts, vout)
			}
		}
		spends := FindSpendsFromIndexOnly(addrIx, txid, vouts)
		for i := range outputs {
			vout, _ := outputs[i]["index"].(int)
			if hit, ok := spends[vout]; ok && hit.Found {
				outputs[i]["spend_txid"] = hit.SpendTxid
				outputs[i]["spend_vin"] = hit.SpendVin
				outputs[i]["spend_height"] = hit.Height
				outputs[i]["can_jump_spend"] = true
				outputs[i]["hint"] = fmt.Sprintf("Spent in block #%d · tap to open spending tx (vin %d).", hit.Height, hit.SpendVin)
			}
		}
		if len(outputs) > blockstepMaxSpendVouts {
			outSpendNote := fmt.Sprintf("Spend links omitted for vout %d+ (max %d per view).", blockstepMaxSpendVouts, blockstepMaxSpendVouts)
			for i := blockstepMaxSpendVouts; i < len(outputs); i++ {
				if outputs[i]["hint"] == nil {
					outputs[i]["hint"] = outSpendNote
				}
			}
		}
	}
	var inSum, outSum float64
	for _, inp := range inputs {
		if v, ok := inp["doge"].(float64); ok {
			inSum += v
		}
	}
	for _, out := range outputs {
		if v, ok := out["doge"].(float64); ok {
			outSum += v
		}
	}
	out := map[string]any{
		"found":   true,
		"txid":    txid,
		"source":  src,
		"tx":      jm,
		"inputs":  inputs,
		"outputs": outputs,
	}
	if inSum > 0 && outSum > 0 && inSum >= outSum {
		out["fee_doge"] = inSum - outSum
	}
	if pq := blockstepTxPQSummary(outputs); pq != nil {
		out["pq_commitment"] = pq
	}
	if carrier := blockstepPQCarrierView(rawTx, inputs, outputs, txid); carrier != nil {
		out["pq_carrier"] = carrier
	}
	out["availability"] = map[string]any{
		"status":  "ready",
		"icon":    "receipt_long",
		"message": "Transaction loaded from your local node.",
	}
	if src == "mempool" {
		out["availability"] = map[string]any{
			"status":  "mempool",
			"icon":    "pending_actions",
			"message": "Unconfirmed - still in the mempool; inputs may not have a mined history on-chain yet.",
		}
	}
	_ = rawTx
	return out, 0, ""
}

func blockstepPQCarrierView(rawTx []byte, inputs, outputs []map[string]any, txid string) map[string]any {
	if len(rawTx) == 0 {
		return blockstepPQCarrierViewFromNav(inputs, outputs, txid)
	}
	tx, err := wire.DeserializeTx(rawTx)
	if err != nil {
		return blockstepPQCarrierViewFromNav(inputs, outputs, txid)
	}
	if _, _, ok := consensus.DetectPQCommitmentInTx(tx); ok {
		view := map[string]any{"role": "tx_c", "txc_txid": txid}
		for _, o := range outputs {
			if kind, _ := o["output_kind"].(string); kind == "pq_carrier" {
				if spend, _ := o["spend_txid"].(string); spend != "" {
					view["txr_txid"] = spend
					view["reveal_status"] = "confirmed"
					break
				}
			}
		}
		if view["txr_txid"] == nil {
			view["reveal_status"] = "pending"
			view["note"] = "TX_R reveal not seen on this node yet - carrier outputs unspent."
		}
		return view
	}
	parts, err := consensus.ParsePQCarrierTXR(tx)
	if err == nil && len(parts) > 0 {
		if algo, ok := consensus.PQCarrierAlgoForCarrierTag(parts[0].CarrierTag8); ok {
			view := map[string]any{
				"role":          "tx_r",
				"txr_txid":      txid,
				"pq_tag":        algo.OPReturnTag,
				"carrier_tag":   parts[0].CarrierTag8,
				"reveal_status": "confirmed",
			}
			if len(inputs) > 0 {
				if prev, _ := inputs[0]["prev_txid"].(string); prev != "" {
					view["txc_txid"] = prev
				}
			}
			return view
		}
	}
	return blockstepPQCarrierViewFromNav(inputs, outputs, txid)
}

func blockstepPQCarrierViewFromNav(inputs, outputs []map[string]any, txid string) map[string]any {
	for _, o := range outputs {
		if tag, _ := o["pq_tag"].(string); tag != "" {
			view := map[string]any{"role": "tx_c", "txc_txid": txid, "pq_tag": tag}
			for _, out := range outputs {
				if spend, _ := out["spend_txid"].(string); spend != "" {
					if kind, _ := out["output_kind"].(string); kind == "pq_carrier" {
						view["txr_txid"] = spend
						view["reveal_status"] = "confirmed"
						return view
					}
				}
			}
			view["reveal_status"] = "pending"
			return view
		}
	}
	for _, inp := range inputs {
		if tag, _ := inp["pq_carrier_tag"].(string); tag != "" {
			view := map[string]any{
				"role":          "tx_r",
				"txr_txid":      txid,
				"carrier_tag":   tag,
				"reveal_status": "confirmed",
			}
			if prev, _ := inp["prev_txid"].(string); prev != "" {
				view["txc_txid"] = prev
			}
			if algo, ok := consensus.PQCarrierAlgoForCarrierTag(tag); ok {
				view["pq_tag"] = algo.OPReturnTag
			}
			return view
		}
	}
	return nil
}

func blockstepRPCChainName(networkSlug string) (string, error) {
	net, err := networkFromUISlug(networkSlug)
	if err != nil {
		return "", err
	}
	switch net {
	case chain.MainnetDogecoin:
		return "main", nil
	case chain.RebootTestnet:
		return "test", nil
	default:
		return "", fmt.Errorf("unknown network")
	}
}

func blockstepVinNav(jm map[string]interface{}, txIx *store.TxIndex, raw *store.RawBlockStore, canJump bool) []map[string]any {
	vin, _ := jm["vin"].([]interface{})
	var out []map[string]any
	for i, v := range vin {
		m, _ := v.(map[string]interface{})
		if m == nil {
			continue
		}
		entry := map[string]any{"index": i}
		if cb, ok := m["coinbase"].(string); ok && cb != "" {
			entry["type"] = "coinbase"
			entry["label"] = "New coins from mining"
			entry["can_jump"] = false
			entry["icon"] = "generating_tokens"
			out = append(out, entry)
			continue
		}
		prevTx, _ := m["txid"].(string)
		prevVout, _ := m["vout"].(uint32)
		if vf, ok := m["vout"].(float64); ok {
			prevVout = uint32(vf)
		}
		entry["type"] = "input"
		entry["prev_txid"] = prevTx
		entry["prev_vout"] = prevVout
		entry["can_jump"] = canJump && prevTx != ""
		if ss, ok := m["scriptSig"].(map[string]interface{}); ok {
			if hx, ok := ss["hex"].(string); ok && hx != "" {
				entry["script_sig_hex"] = hx
				if script, err := hex.DecodeString(hx); err == nil {
					if part, err := consensus.ParsePQCarrierPartScriptSig(script); err == nil {
						entry["pq_carrier_reveal"] = true
						entry["pq_carrier_tag"] = part.CarrierTag8
						if algo, ok := consensus.PQCarrierAlgoForCarrierTag(part.CarrierTag8); ok {
							entry["pq_tag"] = algo.OPReturnTag
						}
						entry["icon"] = "verified_user"
						entry["hint"] = "PQ carrier reveal (TX_R) - full verification material in scriptSig."
					}
				}
			}
		}
		if canJump && prevTx != "" && txIx != nil && raw != nil {
			if pm, _, lerr := rpc.LookupTxFromIndex(txIx, raw, prevTx); lerr == nil {
				if vouts, ok := pm["vout"].([]interface{}); ok && int(prevVout) < len(vouts) {
					if om, ok := vouts[int(prevVout)].(map[string]interface{}); ok {
						if val, ok := om["value"].(float64); ok {
							entry["doge"] = val
							entry["value"] = val
						}
					}
				}
			}
		}
		if entry["doge"] == nil && !entry["can_jump"].(bool) {
			entry["hint"] = "Value unknown - ancestor not indexed yet."
		}
		if !entry["can_jump"].(bool) {
			entry["icon"] = "help_outline"
			if entry["hint"] == nil || entry["hint"] == "Value unknown - ancestor not indexed yet." {
				entry["hint"] = "Ancestor not indexed yet - sync more blocks to follow this coin backward."
			}
		} else {
			entry["icon"] = "arrow_back"
			if v, ok := entry["doge"].(float64); ok {
				entry["hint"] = fmt.Sprintf("%.8f DOGE - tap to follow backward.", v)
			} else {
				entry["hint"] = "Tap to see where these coins came from."
			}
		}
		out = append(out, entry)
	}
	return out
}

func blockstepVoutNav(jm map[string]interface{}, pubVer, scriptVer byte) []map[string]any {
	vout, _ := jm["vout"].([]interface{})
	var out []map[string]any
	for _, v := range vout {
		m, _ := v.(map[string]interface{})
		if m == nil {
			continue
		}
		idx, _ := m["n"].(float64)
		val, _ := m["value"].(float64)
		entry := map[string]any{
			"index": int(idx),
			"value": val,
			"doge":  val,
			"icon": "call_made",
			"hint":  "Tap address to explore payments from this output.",
		}
		if spk, ok := m["scriptPubKey"].(map[string]interface{}); ok {
			if addr, ok := spk["address"].(string); ok && addr != "" {
				entry["address"] = addr
				entry["can_jump"] = true
			} else if addrs, ok := spk["addresses"].([]interface{}); ok && len(addrs) > 0 {
				if a, ok := addrs[0].(string); ok {
					entry["address"] = a
					entry["can_jump"] = true
				}
			}
			if entry["address"] == nil {
				if hx, ok := spk["hex"].(string); ok && hx != "" {
					entry["script_hex"] = hx
					if script, err := hex.DecodeString(hx); err == nil {
						if c, ok := consensus.DetectPQCommitment(script); ok {
							entry["output_kind"] = "pq_commitment"
							entry["pq_tag"] = c.Tag
							entry["pq_scheme"] = c.Scheme
							entry["pq_commitment"] = c.Commitment
							entry["icon"] = "verified_user"
							entry["hint"] = "Post-quantum OP_RETURN commitment (" + c.Tag + ", Phase-1 recognition only)."
							if asm, ok := spk["asm"].(string); ok && asm != "" {
								entry["asm"] = asm
							}
						} else if consensus.IsPQCarrierScriptPubKey(script) {
							entry["output_kind"] = "pq_carrier"
							entry["icon"] = "verified_user"
							entry["hint"] = "PQ carrier P2SH output (TX_C) - revealed in companion TX_R."
						} else {
							addr, kind, hint := blockstepScriptDisplay(script, pubVer, scriptVer)
							if addr != "" {
								entry["address"] = addr
								entry["can_jump"] = true
								entry["output_kind"] = kind
							} else if kind == "op_return" {
								entry["output_kind"] = kind
								entry["hint"] = "OP_RETURN data output (not spendable)."
								if asm, ok := spk["asm"].(string); ok && asm != "" {
									entry["asm"] = asm
								}
								if payload := blockstepOpReturnPayloadHex(script); payload != "" {
									entry["op_return_payload"] = payload
								}
							} else if hint != "" {
								entry["hint"] = hint
							}
						}
					}
				}
			}
		}
		if entry["address"] == nil && entry["hint"] == nil {
			entry["can_jump"] = false
			entry["icon"] = "description"
			entry["hint"] = "No standard address - may be multisig or raw script."
		} else if entry["can_jump"] == nil {
			entry["can_jump"] = false
		}
		if entry["can_jump"] == true {
			entry["icon"] = "account_balance_wallet"
			if entry["hint"] == nil {
				entry["hint"] = "Tap address to explore payments from this output."
			}
		}
		out = append(out, entry)
	}
	return out
}

func blockstepTxPQSummary(outputs []map[string]any) map[string]any {
	for _, o := range outputs {
		tag, _ := o["pq_tag"].(string)
		if tag == "" {
			continue
		}
		summary := map[string]any{
			"tag":  tag,
			"icon": "verified_user",
		}
		if scheme, ok := o["pq_scheme"].(string); ok && scheme != "" {
			summary["scheme"] = scheme
		}
		if commit, ok := o["pq_commitment"].(string); ok && commit != "" {
			summary["commitment"] = commit
		}
		if idx, ok := o["index"].(int); ok {
			summary["vout"] = idx
		}
		summary["note"] = "Phase-1 OP_RETURN post-quantum commitment (recognition only)."
		return summary
	}
	return nil
}

func blockstepOpReturnPayloadHex(script []byte) string {
	if len(script) < 2 || script[0] != 0x6a {
		return ""
	}
	if script[1] == 0x00 {
		return ""
	}
	pushLen := int(script[1])
	if pushLen <= 0 || 2+pushLen > len(script) {
		return ""
	}
	return hex.EncodeToString(script[2 : 2+pushLen])
}

func blockstepScriptDisplay(script []byte, pubVer, scriptVer byte) (addr, kind, hint string) {
	if a := chain.ScriptPubKeyAddress(script, pubVer, scriptVer); a != "" {
		return a, "address", ""
	}
	if len(script) >= 3 && script[len(script)-1] == 0xac {
		var pub []byte
		switch script[0] {
		case 0x21:
			if len(script) == 35 {
				pub = script[1:33]
			}
		case 0x41:
			if len(script) == 67 {
				pub = script[1:65]
			}
		}
		if len(pub) > 0 {
			h := chain.Hash160(pub)
			var h20 [20]byte
			copy(h20[:], h)
			return chain.Base58CheckEncode(pubVer, h20[:]), "pubkey", "Legacy pay-to-pubkey - tap to explore."
		}
	}
	if len(script) >= 1 && script[0] == 0x6a {
		return "", "op_return", ""
	}
	return "", "other", ""
}

// BlockStepAddressDetail wraps address scan for BlockStep UI.
func BlockStepAddressDetail(cfg StartConfig, address, hintTxid string, hintVout int, recvOffset, recvLimit, spendOffset, spendLimit int) (map[string]any, int, string) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, 400, "missing address"
	}
	net, err := networkFromUISlug(cfg.Network)
	if err != nil {
		return nil, 400, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, 500, err.Error()
	}
	address = normalizeScanAddress(address, p.PubkeyHashAddrID, p.ScriptHashAddrID)
	var scan map[string]any
	if cfg.AddrIndex != nil && cfg.AddrIndex.HasAny() {
		if indexed, err := ScanAddressFromIndex(cfg.AddrIndex, address, recvOffset, recvLimit, spendOffset, spendLimit); err == nil {
			scan = indexed
			if recvOffset == 0 && strings.TrimSpace(hintTxid) != "" {
				mergeHintOnlyForAddressIndexPage(scan, cfg, address, p.PubkeyHashAddrID, p.ScriptHashAddrID, hintTxid, hintVout)
			}
		}
	}
	if scan == nil && cfg.Wallet != nil {
		if fast, ok := ScanAddressWalletFast(cfg, address, p.PubkeyHashAddrID, p.ScriptHashAddrID, recvOffset, recvLimit, spendOffset, spendLimit); ok {
			scan = fast
		}
	}
	if scan == nil {
		legacy, err := ScanAddressInRawWindow(cfg.Journal, cfg.RawBlocks, cfg.TxIndex, p.PubkeyHashAddrID, p.ScriptHashAddrID, address, cfg.Pool, strings.TrimSpace(hintTxid), hintVout, cfg.UtxoCache)
		if err != nil {
			return map[string]any{
				"found":   false,
				"address": address,
				"availability": map[string]any{
					"status":  "unavailable",
					"icon":    "cloud_off",
					"message": err.Error(),
				},
				"dogego_tip": "Full node with synced blocks needed to explore addresses locally.",
			}, 0, ""
		}
		scan = legacy
		paginateAddrScanResult(scan, recvOffset, recvLimit, spendOffset, spendLimit)
	}
	scan["found"] = true
	scan["availability"] = map[string]any{
		"status":  "ready",
		"icon":    "wallet",
		"message": "Address history from local index.",
	}
	if _, indexed := scan["indexed"].(bool); !indexed {
		if fast, _ := scan["wallet_fast"].(bool); fast {
			scan["availability"] = map[string]any{
				"status":  "ready",
				"icon":    "account_balance_wallet",
				"message": "Wallet-owned address from UTXO cache and wallet.db.",
			}
		} else {
			scan["availability"] = map[string]any{
				"status":  "window",
				"icon":    "wallet",
				"message": "Indexed outputs and spends from stored blocks in the recent window.",
			}
		}
	}
	attachAddressUTXOBalance(scan, cfg.RPCInvoke, cfg.UtxoCache, p.PubkeyHashAddrID, p.ScriptHashAddrID, address)
	return scan, 0, ""
}

func mergeHintOnlyForAddressIndexPage(scan map[string]any, cfg StartConfig, address string, pubVer, scriptVer byte, hintTxid string, hintVout int) {
	hits, ok := scan["matching_outputs"].([]AddrTxHit)
	if !ok {
		return
	}
	var totalKoinu int64
	if n, ok := scan["total_received_koinu_window"].(int64); ok {
		totalKoinu = n
	}
	mergeHintOutpoint(&hits, cfg.Journal, cfg.RawBlocks, cfg.TxIndex, cfg.Pool, hintTxid, hintVout, address, pubVer, scriptVer, &totalKoinu)
	linkOutputSpendsFromIndex(hits, cfg.AddrIndex)
	scan["matching_outputs"] = hits
}

func hintVoutFromQuery(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return -1
	}
	return v
}

func registerBlockStepRoutes(mux *http.ServeMux, cfg StartConfig) {
	mux.HandleFunc("/api/blockstep/meta", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		m, code, msg := BlockStepMeta(cfg)
		writeBlockStepJSON(w, m, code, msg)
	})
	mux.HandleFunc("/api/blockstep/at-time", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ts, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("ts")), 10, 64)
		if ts <= 0 {
			writeBlockStepJSON(w, nil, 400, "invalid ts")
			return
		}
		h, err := HeightAtUnix(cfg.Journal, ts)
		if err != nil {
			writeBlockStepJSON(w, nil, 500, err.Error())
			return
		}
		h80, _ := cfg.Journal.ReadHeaderAt(h)
		writeBlockStepJSON(w, map[string]any{
			"height": h,
			"time":   headerTimeUnix(h80),
			"hash":   pow.BlockHashHex(h80),
		}, 0, "")
	})
	mux.HandleFunc("/api/blockstep/timeline", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		fromTS, _ := strconv.ParseInt(q.Get("from"), 10, 64)
		toTS, _ := strconv.ParseInt(q.Get("to"), 10, 64)
		pts, _ := strconv.Atoi(q.Get("points"))
		m, code, msg := BlockStepTimeline(cfg.Journal, cfg.RawBlocks, fromTS, toTS, pts)
		writeBlockStepJSON(w, m, code, msg)
	})
	mux.HandleFunc("/api/blockstep/block", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		m, code, msg := BlockStepBlockDetail(cfg.Journal, cfg.RawBlocks, cfg.TxIndex, cfg.Network, r.URL.Query().Get("height"), blockTxOffsetFromQuery(r.URL.Query()), blockTxLimitFromQuery(r.URL.Query()), contiguousHeightForAPI(cfg))
		writeBlockStepJSON(w, m, code, msg)
	})
	mux.HandleFunc("/api/blockstep/tx", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		m, code, msg := BlockStepTxDetail(cfg.Network, cfg.Journal, cfg.RawBlocks, cfg.TxIndex, cfg.AddrIndex, cfg.Pool, r.URL.Query().Get("txid"))
		writeBlockStepJSON(w, m, code, msg)
	})
	mux.HandleFunc("/api/blockstep/address", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		ro, rl, so, sl := addrIndexPageFromQuery(q.Get("recv_offset"), q.Get("recv_limit"), q.Get("spend_offset"), q.Get("spend_limit"))
		m, code, msg := BlockStepAddressDetail(cfg, q.Get("address"), strings.TrimSpace(q.Get("hint_txid")), hintVoutFromQuery(q.Get("hint_vout")), ro, rl, so, sl)
		writeBlockStepJSON(w, m, code, msg)
	})
}

func blockTxOffsetFromQuery(q url.Values) int {
	if n, err := parseNonNegInt(q.Get("tx_offset")); err == nil {
		return n
	}
	return 0
}

func blockTxLimitFromQuery(q url.Values) int {
	if n, err := parseNonNegInt(q.Get("tx_limit")); err == nil && n > 0 {
		if n > maxBlockTxListAPI {
			return maxBlockTxListAPI
		}
		return n
	}
	return blockTxDefaultPage
}

func writeBlockStepJSON(w http.ResponseWriter, m map[string]any, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if code != 0 {
		status := http.StatusInternalServerError
		switch code {
		case 400:
			status = http.StatusBadRequest
		case 404:
			status = http.StatusNotFound
		case 503:
			status = http.StatusServiceUnavailable
		}
		w.WriteHeader(status)
		if m == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
			return
		}
	}
	if m != nil && msg != "" && code != 0 {
		m["error"] = msg
	}
	_ = json.NewEncoder(w).Encode(m)
}
