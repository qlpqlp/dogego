// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

func tryWalletAutoBumpFee(
	chainName string,
	paths *DataPaths,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	blocks *store.RawBlockStore,
	j HeaderJournal,
	oldTx *wire.Tx,
	oldRaw []byte,
	opts map[string]json.RawMessage,
	relayTx func([]byte) error,
	net chain.Network,
) (interface{}, int, string) {
	if paths == nil || rpcWalletAddress(paths) == "" {
		return nil, -8, "bumpfee: options with rawtx required (DogeGo has no wallet fee bump)"
	}
	if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
		return nil, code, msg
	}
	newTx, _, _, code, msg := buildWalletBumpFeeTx(chainName, paths, pool, txIndex, blocks, oldTx, oldRaw, opts, "bumpfee")
	if code != 0 {
		return nil, code, msg
	}
	ser, err := newTx.Serialize()
	if err != nil {
		return nil, -8, "bumpfee: internal error"
	}
	hexUnsigned := hex.EncodeToString(ser)
	unsignedParam, _ := json.Marshal(hexUnsigned)
	signRes, code, msg := execSignRawTransaction(chainName, paths, []json.RawMessage{
		unsignedParam,
		json.RawMessage(`null`),
		json.RawMessage(`null`),
		json.RawMessage(`null`),
	})
	if code != 0 {
		return nil, code, msg
	}
	if complete, _ := signRes["complete"].(bool); !complete {
		return nil, -8, "bumpfee: signing incomplete"
	}
	signedHex, _ := signRes["hex"].(string)
	newRaw, err := hex.DecodeString(strings.TrimPrefix(strings.ToLower(signedHex), "0x"))
	if err != nil || len(newRaw) == 0 {
		return nil, -8, "bumpfee: signed tx decode failed"
	}
	replacement, err := wire.DeserializeTx(newRaw)
	if err != nil {
		return nil, -22, "TX decode failed"
	}
	return submitBumpFeeReplacement(oldTx, oldRaw, replacement, newRaw, pool, txIndex, blocks, j, paths, relayTx, net)
}

// buildWalletBumpFeeTx clones a mempool tx and reduces the wallet change output to raise the fee (unsigned).
func buildWalletBumpFeeTx(
	chainName string,
	paths *DataPaths,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	blocks *store.RawBlockStore,
	oldTx *wire.Tx,
	oldRaw []byte,
	opts map[string]json.RawMessage,
	errPrefix string,
) (*wire.Tx, int64, int64, int, string) {
	if !wire.IsBIP125Replaceable(oldTx) {
		return nil, 0, 0, -8, errPrefix + ": transaction is not BIP125 replaceable in mempool"
	}
	spendScripts := rpcWalletBumpFeeSpendScripts(chainName, paths)
	if len(spendScripts) == 0 {
		return nil, 0, 0, -8, errPrefix + ": wallet not available"
	}
	changeIdx := walletChangeVoutIndexScripts(oldTx, spendScripts)
	if changeIdx < 0 {
		return nil, 0, 0, -8, errPrefix + ": unable to bump fee: no change output"
	}
	view := bumpFeePrevOutView(pool, paths, txIndex, blocks)
	origFee, err := consensus.TxFee(oldTx, view)
	if err != nil || origFee < 0 {
		return nil, 0, 0, -8, errPrefix + ": cannot compute original fee"
	}
	newTx := cloneLegacyTx(oldTx)
	ser, err := newTx.Serialize()
	if err != nil {
		return nil, 0, 0, -8, errPrefix + ": internal error"
	}
	minNewFee := origFee + consensus.FeeForSize(consensus.IncrementalRelayFeePerKB(), len(ser))
	targetFee := minNewFee
	if opts != nil {
		if v, ok := opts["fee_rate"]; ok {
			rate, code, msg := parseFeeRateDOGEPerKB(v, errPrefix)
			if code != 0 {
				return nil, 0, 0, code, msg
			}
			fr := consensus.FeeForSize(uint64(rate*1e8), len(ser))
			if fr > targetFee {
				targetFee = fr
			}
		}
	}
	delta := targetFee - origFee
	estSignedSz := len(ser) + len(newTx.Vin)*150
	incr := consensus.FeeForSize(consensus.IncrementalRelayFeePerKB(), estSignedSz)
	if delta < incr {
		delta = incr
	}
	rbfMin := origFee + incr
	if origFee+delta < rbfMin {
		delta = rbfMin - origFee
	}
	oldSz := len(oldRaw)
	if oldSz <= 0 {
		oldSz = len(oldTx.SerializeForHash())
	}
	if oldSz > 0 && estSignedSz > 0 {
		confRate := origFee * 1000 / int64(oldSz)
		minDelta := (confRate*int64(estSignedSz))/1000 - origFee + 1
		if minDelta > delta {
			delta = minDelta
		}
	}
	if newTx.Vout[changeIdx].Value <= delta {
		return nil, 0, 0, -6, "Insufficient funds"
	}
	newTx.Vout[changeIdx].Value -= delta
	if newTx.Vout[changeIdx].Value > 0 && newTx.Vout[changeIdx].Value < consensus.HardDustLimitKoinu {
		return nil, 0, 0, -6, "Insufficient funds"
	}
	newFee, err := consensus.TxFee(newTx, view)
	if err != nil || newFee < 0 {
		newFee = origFee + delta
	}
	return newTx, origFee, newFee, 0, ""
}

// walletChangeVoutIndexScripts finds the largest wallet change output (any HD spend script).
func walletChangeVoutIndexScripts(tx *wire.Tx, spendScripts [][]byte) int {
	if tx == nil || len(spendScripts) == 0 {
		return -1
	}
	spendSet := walletScriptSet(spendScripts)
	best := -1
	var bestVal int64
	for i, out := range tx.Vout {
		if _, ok := spendSet[hex.EncodeToString(out.PkScript)]; !ok {
			continue
		}
		if out.Value > bestVal {
			bestVal = out.Value
			best = i
		}
	}
	if best < 0 || bestVal == 0 {
		return -1
	}
	return best
}

func cloneLegacyTx(src *wire.Tx) *wire.Tx {
	if src == nil {
		return nil
	}
	dst := &wire.Tx{Version: src.Version, LockTime: src.LockTime}
	for _, in := range src.Vin {
		dst.Vin = append(dst.Vin, wire.TxIn{
			PrevHash: in.PrevHash,
			PrevIdx:  in.PrevIdx,
			Script:   append([]byte(nil), in.Script...),
			Sequence: in.Sequence,
		})
	}
	for _, out := range src.Vout {
		dst.Vout = append(dst.Vout, wire.TxOut{
			Value:    out.Value,
			PkScript: append([]byte(nil), out.PkScript...),
		})
	}
	return dst
}

func submitBumpFeeReplacement(
	oldTx *wire.Tx,
	oldRaw []byte,
	newTx *wire.Tx,
	newRaw []byte,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	blocks *store.RawBlockStore,
	j HeaderJournal,
	paths *DataPaths,
	relayTx func([]byte) error,
	net chain.Network,
) (interface{}, int, string) {
	if !txDoubleSpendsSameInputs(oldTx, newTx) {
		return nil, -8, "bumpfee: replacement does not spend the same inputs as the original transaction"
	}
	view := bumpFeePrevOutView(pool, paths, txIndex, blocks)
	origFee, err := consensus.TxFee(oldTx, view)
	if err != nil || origFee < 0 {
		return nil, -8, "bumpfee: cannot compute original fee"
	}
	if err := consensus.TryResolveMempoolRBFConflicts(newTx, pool, view, fullRBFFromPaths(paths)); err != nil {
		if strings.Contains(err.Error(), "not BIP125") {
			return nil, -8, "bumpfee: transaction is not BIP125 replaceable in mempool"
		}
		return nil, -8, "bumpfee: " + err.Error()
	}
	adm := newMempoolAdmission(pool, txIndex, blocks, j, paths, net)
	if err := consensus.AcceptMempoolTxAdmission(newTx, adm); err != nil {
		return nil, -26, "mandatory-script-verify-flag-failed (DogeGo): " + err.Error()
	}
	fees, sizes := consensus.MempoolEvictionMaps(pool, view)
	consensus.AddCandidateEvictionEntry(newTx, newRaw, view, fees, sizes)
	if err := pool.AddWithEviction(newRaw, fees, sizes); err != nil {
		return nil, -4, "bumpfee: " + err.Error()
	}
	if relayTx != nil {
		_ = relayTx(newRaw)
	}
	if paths != nil && paths.WalletRecordTxReplacement != nil {
		_ = paths.WalletRecordTxReplacement(txidToRPC(oldTx.TxHash()), txidToRPC(newTx.TxHash()))
	}
	newID := txidToRPC(newTx.TxHash())
	walletRecordTxHex(paths, newID, hex.EncodeToString(newRaw))
	newFee, err := consensus.TxFee(newTx, view)
	if err != nil || newFee < 0 {
		newFee = 0
	}
	return map[string]interface{}{
		"txid":    txidToRPC(newTx.TxHash()),
		"origfee": float64(origFee) / 1e8,
		"fee":     float64(newFee) / 1e8,
		"errors":  []interface{}{},
	}, 0, ""
}
