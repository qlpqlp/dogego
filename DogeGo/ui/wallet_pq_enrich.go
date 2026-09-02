// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/hex"
	"strings"

	"dogego/consensus"
	"dogego/wire"
)

// walletPQMetaFromHex classifies a tx for History (OP_RETURN commitment or carrier scriptSig).
// kind is sent_pq or received_pq; pqTag is FLC1/DIL2/RCG4 when detected.
func walletPQMetaFromHex(hexStr string, category string) (kind, pqTag string) {
	raw, err := hex.DecodeString(strings.TrimSpace(hexStr))
	if err != nil {
		return "", ""
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return "", ""
	}
	tag, source := walletPQDetectTag(tx)
	if tag == "" {
		return "", ""
	}
	_ = source
	category = strings.ToLower(strings.TrimSpace(category))
	switch category {
	case "send":
		return "sent_pq", tag
	case "receive":
		return "received_pq", tag
	default:
		if source == "carrier_scriptsig" {
			return "sent_pq", tag
		}
		return "received_pq", tag
	}
}

func walletPQDetectTag(tx *wire.Tx) (tag, source string) {
	for _, o := range tx.Vout {
		if c, ok := consensus.DetectPQCommitment(o.PkScript); ok {
			return c.Tag, "commitment_only"
		}
	}
	for _, in := range tx.Vin {
		if part, err := consensus.ParsePQCarrierPartScriptSig(in.Script); err == nil {
			if algo, ok := consensus.PQCarrierAlgoForCarrierTag(part.CarrierTag8); ok {
				return algo.OPReturnTag, "carrier_scriptsig"
			}
		}
	}
	return "", ""
}

// walletSendPQMetaFromHex classifies a send tx for History (OP_RETURN commitment or carrier scriptSig).
func walletSendPQMetaFromHex(hexStr string) (kind, pqTag string) {
	return walletPQMetaFromHex(hexStr, "send")
}

func enrichWalletPQUIEntry(entry map[string]interface{}, hexStr string) {
	if entry == nil || strings.TrimSpace(hexStr) == "" {
		return
	}
	category := strings.ToLower(strings.TrimSpace(strFromAny(entry["category"])))
	if category == "" {
		category = "receive"
	}
	if kind, tag := walletPQMetaFromHex(hexStr, category); kind != "" {
		entry["tx_kind"] = kind
		if tag != "" {
			entry["pq_tag"] = tag
		}
	}
}

func enrichWalletSendUIEntry(entry map[string]interface{}, hexStr string) {
	if entry != nil {
		entry["category"] = "send"
	}
	enrichWalletPQUIEntry(entry, hexStr)
}

func enrichWalletReceiveUIEntry(cfg StartConfig, entry map[string]interface{}) {
	if entry == nil || cfg.ActiveWallet() == nil {
		return
	}
	txid := strings.ToLower(strings.TrimSpace(strFromAny(entry["txid"])))
	if txid == "" {
		return
	}
	height := int64(-1)
	if h, ok := entry["blockheight"].(float64); ok {
		height = int64(h)
	} else if h, ok := entry["blockheight"].(int64); ok {
		height = h
	}
	if hx := walletTxHexForUI(cfg, txid, height); hx != "" {
		enrichWalletPQUIEntry(entry, hx)
	}
}

func walletHistoryEntryMatchesKind(entry map[string]interface{}, kind string) bool {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" || kind == "all" {
		return true
	}
	kindVal := strings.ToLower(strings.TrimSpace(strFromAny(entry["tx_kind"])))
	if kindVal == "" {
		kindVal = strings.ToLower(strings.TrimSpace(strFromAny(entry["category"])))
	}
	pqTag := strings.TrimSpace(strFromAny(entry["pq_tag"]))
	switch kind {
	case "sent":
		return kindVal == "sent" || kindVal == "send" || kindVal == "sent_pq"
	case "received":
		return kindVal == "received" || kindVal == "receive" || kindVal == "received_pq"
	case "mining":
		return kindVal == "mining" || kindVal == "mining_immature" || kindVal == "generate" || kindVal == "immature"
	case "quantum":
		return kindVal == "sent_pq" || kindVal == "received_pq" || pqTag != ""
	}
	return true
}
