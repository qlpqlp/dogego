// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/store"
	"dogego/wire"
)

var errSubtractFeeInsufficient = errors.New("insufficient funds for fee")

func applySubtractFeeFromOutputs(tx *wire.Tx, indices []int, fee int64) error {
	if tx == nil || fee <= 0 || len(indices) == 0 {
		return nil
	}
	var weight int64
	for _, i := range indices {
		if i < 0 || i >= len(tx.Vout) {
			return fmt.Errorf("invalid vout index")
		}
		if tx.Vout[i].Value <= 0 {
			return fmt.Errorf("invalid vout index")
		}
		weight += tx.Vout[i].Value
	}
	if weight < fee {
		return errSubtractFeeInsufficient
	}
	remaining := fee
	for j, i := range indices {
		var part int64
		if j == len(indices)-1 {
			part = remaining
		} else {
			part = fee * tx.Vout[i].Value / weight
			remaining -= part
		}
		tx.Vout[i].Value -= part
		if tx.Vout[i].Value < consensus.HardDustLimitKoinu && tx.Vout[i].Value > 0 {
			return errSubtractFeeInsufficient
		}
		if tx.Vout[i].Value < 0 {
			return errSubtractFeeInsufficient
		}
	}
	return nil
}

// execFundRawTransaction adds P2PKH inputs from the node UTXO cache until outputs are funded (no wallet).
func execFundRawTransaction(chainName string, paths *DataPaths, j HeaderJournal, rbStore *store.RawBlockStore, ix *store.TxIndex, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if paths == nil || paths.Utxo == nil {
		return nil, -1, "fundrawtransaction: UTXO set not available (sync chain with tx index / UTXO cache)"
	}
	if rpcWalletDefaultAddress(paths) != "" {
		if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
			return nil, code, msg
		}
	}
	var hexStr string
	if err := json.Unmarshal(params[0], &hexStr); err != nil {
		return nil, -8, "fundrawtransaction: hexstring must be a string"
	}
	hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	txRaw, err := hex.DecodeString(hexStr)
	if err != nil || len(txRaw) == 0 {
		return nil, -8, "fundrawtransaction: TX decode failed"
	}
	tx, err := wire.DeserializeTx(txRaw)
	if err != nil {
		return nil, -8, "fundrawtransaction: TX decode failed"
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}

	fundOpts := defaultFundRawTxOptions()
	if len(params) == 2 && strings.TrimSpace(string(params[1])) != "null" {
		var legacy bool
		if err := json.Unmarshal(params[1], &legacy); err == nil {
			_ = legacy
		} else {
			var opts map[string]json.RawMessage
			if err := json.Unmarshal(params[1], &opts); err != nil {
				return nil, -8, "fundrawtransaction: options must be a JSON object"
			}
			var code int
			var msg string
			fundOpts, code, msg = parseFundRawTxOptions(paths, opts)
			if code != 0 {
				return nil, code, msg
			}
		}
	}
	changeAddr := fundOpts.changeAddr
	if changeAddr != "" {
		vis, _, _ := ValidateAddressString(chainName, changeAddr)
		if ok, _ := vis["isvalid"].(bool); !ok {
			return nil, -8, "fundrawtransaction: changeAddress must be a valid dogecoin address"
		}
	}
	changePos := fundOpts.changePos
	feePerKB := fundOpts.feePerKB
	subtractFeeFrom := fundOpts.subtractFeeFrom
	inputSequence := fundOpts.inputSequence
	if feePerKB == 0 {
		feePerKB = rpcWalletPayTxFeeKoinuPerKB(paths)
	}
	if feePerKB == 0 {
		feePerKB = minRelayFeeFromPaths(paths)
		if feePerKB == 0 {
			feePerKB = consensus.MinRelayTxFeePerKB()
		}
	}

	outSum := int64(0)
	for _, o := range tx.Vout {
		outSum += o.Value
	}
	inSum := int64(0)
	for _, in := range tx.Vin {
		if consensus.IsNullOutpoint(&in) {
			continue
		}
		id := txidToRPC(in.PrevHash)
		e, ok := walletLookupOutpoint(paths, id, in.PrevIdx)
		if !ok {
			return nil, -8, "fundrawtransaction: input spends unknown outpoint (not in UTXO set)"
		}
		inSum += e.Value
	}

	used := make(map[string]struct{})
	for _, in := range tx.Vin {
		if consensus.IsNullOutpoint(&in) {
			continue
		}
		used[txidToRPC(in.PrevHash)+fmt.Sprintf(":%d", in.PrevIdx)] = struct{}{}
	}

	scripts := rpcWalletTrackedScripts(paths)
	rows, err := walletConfirmedUTXORows(paths, scripts, 0)
	if err != nil {
		return nil, -1, "fundrawtransaction: "+err.Error()
	}
	fundSets := buildFundScriptSets(paths)
	var candidates []fundPick
	for _, row := range rows {
		key := row.TxID + fmt.Sprintf(":%d", row.Vout)
		if _, ok := used[key]; ok {
			continue
		}
		if !utxoRowFundableSets(chainName, paths, fundSets, row.PkScript, p, fundOpts.includeWatching) {
			continue
		}
		if fundOpts.lockUnspents && utxoOutpointLocked(paths, row) {
			continue
		}
		if !utxoRowSpendableForFund(paths, j, rbStore, ix, row, chainName) {
			continue
		}
		candidates = append(candidates, fundPick{row: row})
	}
	if fundOpts.addInputs && len(candidates) == 0 && inSum < outSum {
		return nil, -6, "Insufficient funds"
	}
	if !fundOpts.addInputs && inSum < outSum {
		return nil, -6, "Insufficient funds"
	}

	addedIn := int64(0)
	if fundOpts.addInputs {
		sortFundCandidates(paths, candidates)
	}
	baseSer, err := tx.Serialize()
	if err != nil {
		return nil, -8, err.Error()
	}
	estSize := len(baseSer)
	const fundInputVBytes = 148
	for _, c := range candidates {
		if !fundOpts.addInputs {
			break
		}
		prev, err := decodeRPCPrevHashHex(c.row.TxID)
		if err != nil {
			continue
		}
		estSize += fundInputVBytes
		feeNeeded := int64(consensus.FeeForSize(feePerKB, estSize))
		tx.Vin = append(tx.Vin, wire.TxIn{
			PrevHash: prev,
			PrevIdx:  c.row.Vout,
			Sequence: inputSequence,
		})
		addedIn += c.row.Value
		if inSum+addedIn >= outSum+feeNeeded {
			break
		}
	}

	ser, err := tx.Serialize()
	if err != nil {
		return nil, -8, err.Error()
	}
	fee := int64(consensus.FeeForSize(feePerKB, len(ser)))
	if len(subtractFeeFrom) > 0 {
		if err := applySubtractFeeFromOutputs(tx, subtractFeeFrom, fee); err != nil {
			if err == errSubtractFeeInsufficient {
				return nil, -6, "Insufficient funds"
			}
			return nil, -8, "fundrawtransaction: "+err.Error()
		}
		ser, err = tx.Serialize()
		if err != nil {
			return nil, -8, err.Error()
		}
		fee = int64(consensus.FeeForSize(feePerKB, len(ser)))
		outSum = int64(0)
		for _, o := range tx.Vout {
			outSum += o.Value
		}
		change := inSum + addedIn - outSum
		if change < 0 {
			return nil, -6, "Insufficient funds"
		}
		if change > 0 {
			if changeAddr == "" {
				changeAddr = rpcWalletDefaultChangeAddress(paths)
			}
			if changeAddr == "" {
				return nil, -8, "fundrawtransaction: changeAddress required when change is non-zero"
			}
			pk, err := changeScriptPubKey(changeAddr, p)
			if err != nil {
				return nil, -8, "fundrawtransaction: invalid changeAddress"
			}
			if change < consensus.HardDustLimitKoinu {
				return nil, -6, "Insufficient funds"
			}
			tx.Vout = append(tx.Vout, wire.TxOut{Value: change, PkScript: pk})
			changePos = len(tx.Vout) - 1
			ser, err = tx.Serialize()
			if err != nil {
				return nil, -8, err.Error()
			}
			fee = int64(consensus.FeeForSize(feePerKB, len(ser)))
			walletCommitChangeAddress(paths, changeAddr)
		} else {
			changePos = -1
		}
		if !enforceMinimumTotalFee(tx, &fee, fundOpts.minimumTotalFeeKoinu) {
			return nil, -4, "fundrawtransaction: fee is below minimumTotalFee"
		}
		ser, err = tx.Serialize()
		if err != nil {
			return nil, -8, err.Error()
		}
		return map[string]interface{}{
			"hex":       hex.EncodeToString(ser),
			"fee":       float64(fee) / 1e8,
			"changepos": changePos,
		}, 0, ""
	}

	change := inSum + addedIn - outSum - fee
	if change < 0 {
		return nil, -6, "Insufficient funds"
	}

	if change > 0 && changeAddr == "" {
		changeAddr = rpcWalletDefaultChangeAddress(paths)
	}

	if change > 0 {
		if changeAddr == "" {
			return nil, -8, "fundrawtransaction: changeAddress required when change is non-zero"
		}
		pk, err := changeScriptPubKey(changeAddr, p)
		if err != nil {
			return nil, -8, "fundrawtransaction: invalid changeAddress"
		}
		if change < consensus.HardDustLimitKoinu {
			fee += change
			change = 0
		} else {
			out := wire.TxOut{Value: change, PkScript: pk}
			if changePos >= 0 && changePos <= len(tx.Vout) {
				before := append([]wire.TxOut(nil), tx.Vout[:changePos]...)
				after := append([]wire.TxOut(nil), tx.Vout[changePos:]...)
				tx.Vout = append(before, out)
				tx.Vout = append(tx.Vout, after...)
			} else {
				changePos = len(tx.Vout)
				tx.Vout = append(tx.Vout, out)
			}
			walletCommitChangeAddress(paths, changeAddr)
			ser, err = tx.Serialize()
			if err != nil {
				return nil, -8, err.Error()
			}
			fee = int64(consensus.FeeForSize(feePerKB, len(ser)))
			change = inSum + addedIn - outSum - fee
			if change < 0 {
				return nil, -6, "Insufficient funds"
			}
			if change > 0 && change < consensus.HardDustLimitKoinu {
				tx.Vout = tx.Vout[:len(tx.Vout)-1]
				changePos = -1
				fee += change
				change = 0
				ser, err = tx.Serialize()
				if err != nil {
					return nil, -8, err.Error()
				}
			}
		}
	}

	if changePos < 0 {
		changePos = -1
	}
	if !enforceMinimumTotalFee(tx, &fee, fundOpts.minimumTotalFeeKoinu) {
		return nil, -4, "fundrawtransaction: fee is below minimumTotalFee"
	}
	ser, err = tx.Serialize()
	if err != nil {
		return nil, -8, err.Error()
	}
	return map[string]interface{}{
		"hex":       hex.EncodeToString(ser),
		"fee":       float64(fee) / 1e8,
		"changepos": changePos,
	}, 0, ""
}

func changeScriptPubKey(changeAddr string, p chain.Params) ([]byte, error) {
	ver, h160, err := chain.Base58CheckDecode(changeAddr)
	if err != nil {
		return nil, err
	}
	switch ver {
	case p.PubkeyHashAddrID:
		return chain.P2PKHScriptFromPubKeyHash(h160), nil
	case p.ScriptHashAddrID:
		return chain.P2SHScriptFromScriptHash(h160), nil
	default:
		return nil, fmt.Errorf("unsupported address version")
	}
}

func isFundableP2PKH(pkScript []byte, p2pkhVer byte) bool {
	_ = p2pkhVer
	return len(pkScript) == 25 && pkScript[0] == 0x76 && pkScript[1] == 0xa9 && pkScript[2] == 0x14 &&
		pkScript[23] == 0x88 && pkScript[24] == 0xac
}

func parseFeeRateDOGEPerKB(raw json.RawMessage, method string) (float64, int, string) {
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		if f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, -8, method + ": invalid feeRate"
		}
		return f, 0, ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, -8, method + ": feeRate must be a number"
	}
	f2, err := json.Number(strings.TrimSpace(s)).Float64()
	if err != nil || f2 < 0 || math.IsNaN(f2) || math.IsInf(f2, 0) {
		return 0, -8, method + ": invalid feeRate"
	}
	return f2, 0, ""
}
