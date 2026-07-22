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

const defaultSubmitPackageMaxFeeDOGEPerKB = 0.10

type parsedPackageTx struct {
	raw []byte
	tx  *wire.Tx
}

// execSubmitPackage accepts a topologically sorted parent→child tx package (Core submitpackage subset).
func execSubmitPackage(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, j HeaderJournal, paths *DataPaths, params []json.RawMessage, relayTx func([]byte) error, allowUnverified bool, net chain.Network) (interface{}, int, string) {
	if pool == nil {
		return nil, -18, "submitpackage: mempool not available"
	}
	if len(params) < 1 || len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	var hexList []string
	if err := json.Unmarshal(params[0], &hexList); err != nil {
		return nil, -8, "submitpackage: package must be a JSON array of hex strings"
	}
	if len(hexList) == 0 {
		return nil, -8, "submitpackage: empty package"
	}
	maxFeeRate, feeErr := parseSubmitPackageMaxFeeRate(params, 1)
	if feeErr != "" {
		return nil, -8, feeErr
	}
	maxBurnDOGE, burnErr := parseMaxBurnAmountDOGE(params, 2, "submitpackage")
	if burnErr != "" {
		return nil, -8, burnErr
	}
	maxBurnKoinu := int64(maxBurnDOGE * 1e8)

	pkg, code, msg := decodeSubmitPackage(hexList)
	if code != 0 {
		return nil, code, msg
	}
	if err := validateSubmitPackageStructure(pkg); err != nil {
		return nil, -8, "submitpackage: "+err.Error()
	}

	var utxo consensus.UtxoOutpointSource
	if paths != nil {
		utxo = paths.Utxo
	}
	view := consensus.AdmissionPrevOutViewWithUtxo(pool, utxo, txIndex, blocks)
	pkgTxs := make([]*wire.Tx, len(pkg))
	for i := range pkg {
		pkgTxs[i] = pkg[i].tx
	}
	feePerKB := minRelayFeeFromPaths(paths)
	packageMinRelayOK := true
	var packageMinRelayErr error
	if !allowUnverified {
		if err := consensus.CheckMinRelayFeePackageTxs(pkgTxs, view, feePerKB); err != nil {
			packageMinRelayOK = false
			packageMinRelayErr = err
		}
	}

	beforeIDs := map[string]struct{}{}
	if ids, err := pool.RawMemPoolTxIDs(); err == nil {
		for _, id := range ids {
			beforeIDs[id] = struct{}{}
		}
	}
	pkgTxIDs := make(map[string]struct{}, len(pkg))
	for _, ent := range pkg {
		pkgTxIDs[txidToRPC(ent.tx.TxHash())] = struct{}{}
	}

	txResults := make(map[string]interface{})
	allOK := true

	for i, ent := range pkg {
		wtxid := txidToRPC(ent.tx.WTxHash())
		rpcTxid := txidToRPC(ent.tx.TxHash())
		row := map[string]interface{}{
			"txid": rpcTxid,
		}
		vsize := len(ent.raw)
		row["vsize"] = vsize
		row["weight"] = 4 * vsize

		if txConfirmed(txIndex, rpcTxid) {
			row["error"] = "txn-already-known"
			txResults[wtxid] = row
			allOK = false
			continue
		}
		if opReturnBurnKoinu(ent.tx) > maxBurnKoinu {
			row["error"] = "max-burn-exceeded"
			txResults[wtxid] = row
			allOK = false
			continue
		}

		inMempool := pool.ContainsTxID(rpcTxid)
		var submitErr error
		if !inMempool && !allowUnverified {
			if !packageMinRelayOK {
				submitErr = packageMinRelayErr
			} else {
				adm := newMempoolAdmission(pool, txIndex, blocks, j, paths, net)
				adm.SkipMinRelayFee = true
				submitErr = acceptMempoolTxRPC(ent.raw, ent.tx, pool, paths, adm)
			}
		}
		if submitErr != nil {
			row["error"] = consensus.MempoolRejectReason(submitErr)
			txResults[wtxid] = row
			allOK = false
			continue
		}
		if !inMempool {
			if maxFeeRate > 0 {
				if pkgRep, err := consensus.PackageFeeReportForTx(ent.tx, pool, view); err == nil {
					eff := float64(pkgRep.EffectiveRatePerKB) / 1e8
					if eff > maxFeeRate {
						row["error"] = "max-fee-exceeded"
						txResults[wtxid] = row
						allOK = false
						continue
					}
				}
			}
			fees, sizes := consensus.MempoolEvictionMaps(pool, view)
			consensus.AddCandidateEvictionEntry(ent.tx, ent.raw, view, fees, sizes)
			if err := pool.AddWithEviction(ent.raw, fees, sizes); err != nil {
				row["error"] = err.Error()
				txResults[wtxid] = row
				allOK = false
				continue
			}
			TrackMempoolTxFee(paths, ent.tx, ent.raw, pool, txIndex, blocks, j, net)
			if relayTx != nil {
				_ = relayTx(ent.raw)
			}
		} else if relayTx != nil {
			_ = relayTx(ent.raw)
		}

		applyMempoolAcceptFees(row, ent.tx, pool, txIndex, blocks, rpcTxid)
		appendEffectiveIncludes(row, submitPackageEffectiveIncludes(pkg, i))
		txResults[wtxid] = row
	}

	packageMsg := "success"
	if !allOK {
		packageMsg = "transaction failed"
	}
	out := map[string]interface{}{
		"package_msg": packageMsg,
		"tx-results":  txResults,
	}
	if replaced := submitPackageReplacedTxIDs(pool, beforeIDs, pkgTxIDs); len(replaced) > 0 {
		out["replaced-transactions"] = replaced
	}
	return out, 0, ""
}

func submitPackageReplacedTxIDs(pool *mempool.Pool, before map[string]struct{}, pkgTxIDs map[string]struct{}) []string {
	if pool == nil || len(before) == 0 {
		return nil
	}
	after := map[string]struct{}{}
	if ids, err := pool.RawMemPoolTxIDs(); err == nil {
		for _, id := range ids {
			after[id] = struct{}{}
		}
	}
	var out []string
	for id := range before {
		if _, still := after[id]; still {
			continue
		}
		if _, inPkg := pkgTxIDs[id]; inPkg {
			continue
		}
		out = append(out, id)
	}
	return out
}

func submitPackageEffectiveIncludes(pkg []parsedPackageTx, idx int) []string {
	if idx < 0 || idx >= len(pkg) {
		return nil
	}
	includes := []string{txidToRPC(pkg[idx].tx.WTxHash())}
	// Include earlier package txs spent (directly) by this tx - Core effective-includes for CPFP.
	for j := 0; j < idx; j++ {
		if packageTxSpends(pkg[idx].tx, pkg[j].tx) {
			includes = append(includes, txidToRPC(pkg[j].tx.WTxHash()))
		}
	}
	return includes
}

func appendEffectiveIncludes(row map[string]interface{}, includes []string) {
	if row == nil || len(includes) == 0 {
		return
	}
	fees, ok := row["fees"].(map[string]interface{})
	if !ok {
		return
	}
	fees["effective-includes"] = includes
}

func decodeSubmitPackage(hexList []string) ([]parsedPackageTx, int, string) {
	out := make([]parsedPackageTx, 0, len(hexList))
	for i, hexStr := range hexList {
		hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
		raw, err := hex.DecodeString(hexStr)
		if err != nil || len(raw) == 0 {
			return nil, -8, fmt.Sprintf("submitpackage: invalid hex at index %d", i)
		}
		if len(raw) > maxRawTxBytes {
			return nil, -8, fmt.Sprintf("submitpackage: transaction too large at index %d", i)
		}
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			return nil, -22, fmt.Sprintf("submitpackage: TX decode failed at index %d", i)
		}
		if tx.HasWitness() {
			return nil, -8, fmt.Sprintf("submitpackage: witness transactions are not supported at index %d", i)
		}
		out = append(out, parsedPackageTx{raw: raw, tx: tx})
	}
	return out, 0, ""
}

func validateSubmitPackageStructure(pkg []parsedPackageTx) error {
	n := len(pkg)
	if n == 0 {
		return fmt.Errorf("empty package")
	}
	pos := make(map[string]int, n)
	for i, ent := range pkg {
		pos[txidToRPC(ent.tx.TxHash())] = i
	}
	for i, ent := range pkg {
		for _, in := range ent.tx.Vin {
			if isCoinbaseWireIn(&in) {
				continue
			}
			pid := txidToRPC(in.PrevHash)
			j, inPkg := pos[pid]
			if !inPkg {
				continue
			}
			if j >= i {
				return fmt.Errorf("package not topologically sorted")
			}
		}
	}
	if n >= 2 {
		for i := 0; i < n-1; i++ {
			for j := 0; j < n-1; j++ {
				if i == j {
					continue
				}
				if packageTxSpends(pkg[j].tx, pkg[i].tx) {
					return fmt.Errorf("package contains dependent parents")
				}
			}
		}
	}
	return nil
}

func packageTxSpends(spender, parent *wire.Tx) bool {
	if spender == nil || parent == nil {
		return false
	}
	ph := parent.TxHash()
	for _, in := range spender.Vin {
		if in.PrevHash == ph {
			return true
		}
	}
	return false
}

func opReturnBurnKoinu(tx *wire.Tx) int64 {
	if tx == nil {
		return 0
	}
	var sum int64
	for _, o := range tx.Vout {
		if len(o.PkScript) > 0 && o.PkScript[0] == 0x6a {
			sum += o.Value
		}
	}
	return sum
}

func parseSubmitPackageMaxFeeRate(params []json.RawMessage, idx int) (float64, string) {
	if len(params) <= idx || strings.TrimSpace(string(params[idx])) == "null" {
		return defaultSubmitPackageMaxFeeDOGEPerKB, ""
	}
	rate, errMsg := parseMaxFeeRateDOGEPerKB(params, idx, "submitpackage")
	if errMsg != "" {
		return 0, errMsg
	}
	if rate == 0 {
		return 0, ""
	}
	return rate, ""
}

func parseMaxBurnAmountDOGE(params []json.RawMessage, idx int, method string) (float64, string) {
	if len(params) <= idx || strings.TrimSpace(string(params[idx])) == "null" {
		return 0, ""
	}
	var amt float64
	if err := json.Unmarshal(params[idx], &amt); err != nil || amt < 0 {
		return 0, method + ": invalid maxburnamount"
	}
	return amt, ""
}
