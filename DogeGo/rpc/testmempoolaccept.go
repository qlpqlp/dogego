// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// execTestMempoolAccept dry-runs mempool admission for one or more raw transactions (Core testmempoolaccept).
// Does not add to the mempool or relay. params[0] is a JSON array of hex strings, or a single hex string.
func execTestMempoolAccept(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, j HeaderJournal, paths *DataPaths, params []json.RawMessage, allowUnverified bool, net chain.Network) (result interface{}, errCode int, errMsg string) {
	if pool == nil {
		return nil, -18, "testmempoolaccept: mempool not available"
	}
	if len(params) < 1 {
		return nil, -8, "testmempoolaccept: array of raw transactions required"
	}
	var hexList []string
	if err := json.Unmarshal(params[0], &hexList); err != nil {
		var one string
		if err2 := json.Unmarshal(params[0], &one); err2 != nil {
			return nil, -8, "testmempoolaccept: expected JSON array of hex strings or a single hex string"
		}
		hexList = []string{one}
	}
	if len(hexList) == 0 {
		return nil, -8, "testmempoolaccept: empty array"
	}
	maxFeeRateDOGEPerKB, feeErr := parseMaxFeeRateDOGEPerKB(params, 1, "testmempoolaccept")
	if feeErr != "" {
		return nil, -8, feeErr
	}
	out := make([]map[string]interface{}, 0, len(hexList))
	for _, hexStr := range hexList {
		out = append(out, testOneMempoolAccept(pool, txIndex, blocks, j, paths, hexStr, allowUnverified, net, maxFeeRateDOGEPerKB))
	}
	return out, 0, ""
}

func testOneMempoolAccept(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, j HeaderJournal, paths *DataPaths, hexStr string, allowUnverified bool, net chain.Network, maxFeeRateDOGEPerKB float64) map[string]interface{} {
	hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	res := map[string]interface{}{
		"allowed": false,
	}
	raw, err := hex.DecodeString(hexStr)
	if err != nil || len(raw) == 0 {
		res["reject-reason"] = "TX decode failed"
		return res
	}
	if len(raw) > maxRawTxBytes {
		res["reject-reason"] = fmt.Sprintf("transaction too large (%d bytes)", len(raw))
		return res
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		res["reject-reason"] = "TX decode failed"
		return res
	}
	txid := txidToRPC(tx.TxHash())
	res["txid"] = txid
	res["wtxid"] = txidToRPC(tx.WTxHash())
	vsize := len(raw)
	res["vsize"] = vsize
	res["weight"] = 4 * vsize
	res["fees"] = map[string]interface{}{
		"base":              0.0,
		"modified":          0.0,
		"ancestor":          0.0,
		"descendant":        0.0,
		"effective-feerate": 0.0,
	}
	res["bip125-replaceable"] = wire.IsBIP125Replaceable(tx)
	effDOGE := applyMempoolAcceptFees(res, tx, pool, txIndex, blocks, txid)
	if txConfirmed(txIndex, txid) {
		res["reject-reason"] = "txn-already-known"
		return res
	}
	if pool.IsFull() {
		res["reject-reason"] = "mempool full"
		return res
	}
	if pool.ContainsTxID(txid) {
		res["reject-reason"] = "txn-already-in-mempool"
		return res
	}
	if !allowUnverified {
		adm := newMempoolAdmission(pool, txIndex, blocks, j, paths, net)
		if err := acceptMempoolTxRPC(raw, tx, pool, paths, adm); err != nil {
			res["reject-reason"] = consensus.MempoolRejectReason(err)
			return res
		}
	}
	if maxFeeRateDOGEPerKB > 0 && effDOGE > maxFeeRateDOGEPerKB {
		res["reject-reason"] = "max-fee-exceeded"
		return res
	}
	res["allowed"] = true
	return res
}

// applyMempoolAcceptFees fills res["fees"] when prevouts resolve (Core-shaped; includes prioritisetransaction delta).
// Returns effective feerate in DOGE/kB when known, else 0.
func applyMempoolAcceptFees(res map[string]interface{}, tx *wire.Tx, pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, txid string) float64 {
	view := mempoolAdmissionView(pool, txIndex, blocks)
	if view == nil || tx == nil {
		return 0
	}
	pkg, err := consensus.PackageFeeReportForTx(tx, pool, view)
	if err != nil {
		return 0
	}
	delta := int64(0)
	if pool != nil {
		delta = pool.FeeDeltaKoinu(txid)
	}
	base := float64(pkg.BaseFeeKoinu) / 1e8
	mod := base + float64(delta)/1e8
	anc := float64(pkg.AncestorFeeKoinu) / 1e8
	desc := float64(pkg.DescendantFeeKoinu) / 1e8
	eff := float64(pkg.EffectiveRatePerKB) / 1e8
	res["fees"] = map[string]interface{}{
		"base":              base,
		"modified":          mod,
		"ancestor":          anc,
		"descendant":        desc,
		"effective-feerate": eff,
	}
	return eff
}
