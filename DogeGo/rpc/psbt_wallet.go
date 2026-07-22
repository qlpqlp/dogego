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
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// execWalletCreateFundedPsbt builds, funds, and returns a PSBT (Core walletcreatefundedpsbt subset).
func execWalletCreateFundedPsbt(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, ix *store.TxIndex, pool *mempool.Pool, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 2 || len(params) > 6 {
		return nil, -32602, "Wrong number of arguments"
	}
	if rpcWalletDefaultAddress(paths) == "" {
		return nil, -1, "walletcreatefundedpsbt: built-in wallet is not available"
	}
	if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
		return nil, code, msg
	}
	if paths == nil || paths.Utxo == nil {
		return nil, -1, "walletcreatefundedpsbt: UTXO cache not available"
	}

	var rawInputs []json.RawMessage
	inputsSpecified := false
	if len(params) > 0 && strings.TrimSpace(string(params[0])) != "null" {
		if err := json.Unmarshal(params[0], &rawInputs); err != nil {
			return nil, -8, "walletcreatefundedpsbt: inputs must be a JSON array"
		}
		inputsSpecified = len(rawInputs) > 0
	}

	outputs, code, msg := parseCreateOutputsParam(params[1])
	if code != 0 {
		return nil, code, "walletcreatefundedpsbt: " + msg
	}

	lockTime := uint32(0)
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var lt float64
		if err := json.Unmarshal(params[2], &lt); err != nil {
			return nil, -8, "walletcreatefundedpsbt: bad locktime"
		}
		if lt < 0 || lt != float64(uint32(lt)) {
			return nil, -8, "walletcreatefundedpsbt: locktime must be uint32"
		}
		lockTime = uint32(lt)
	}

	fundOpts := defaultFundRawTxOptions()
	if inputsSpecified {
		fundOpts.addInputs = false
	}
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		var opts map[string]json.RawMessage
		if err := json.Unmarshal(params[3], &opts); err != nil {
			return nil, -8, "walletcreatefundedpsbt: options must be a JSON object"
		}
		fundOpts, code, msg = parseFundRawTxOptions(paths, opts)
		if code != 0 {
			if !strings.HasPrefix(msg, "walletcreatefundedpsbt:") && !strings.HasPrefix(msg, "fundrawtransaction:") {
				msg = "walletcreatefundedpsbt: " + msg
			}
			return nil, code, msg
		}
		if !inputsSpecified {
			fundOpts.addInputs = true
		}
	} else if !inputsSpecified {
		fundOpts.addInputs = true
	}

	version := int32(2)
	if len(params) > 5 && strings.TrimSpace(string(params[5])) != "null" {
		var vf float64
		if err := json.Unmarshal(params[5], &vf); err != nil {
			return nil, -8, "walletcreatefundedpsbt: bad version"
		}
		if vf != float64(int32(vf)) {
			return nil, -8, "walletcreatefundedpsbt: version must be integer"
		}
		version = int32(vf)
	}

	replaceable := true
	if fundOpts.inputSequence != 0xffffffff {
		replaceable = false
	}
	tx, code, msg := buildUnsignedTxFromRPC(chainName, rawInputs, outputs, lockTime, version, &replaceable)
	if code != 0 {
		if !strings.HasPrefix(msg, "walletcreatefundedpsbt:") {
			msg = "walletcreatefundedpsbt: " + msg
		}
		return nil, code, msg
	}
	ser, err := tx.Serialize()
	if err != nil {
		return nil, -8, "walletcreatefundedpsbt: "+err.Error()
	}
	optsRaw, err := json.Marshal(fundRawTxOptionsToMap(fundOpts))
	if err != nil {
		return nil, -8, "walletcreatefundedpsbt: "+err.Error()
	}
	fundRes, code, msg := execFundRawTransaction(chainName, paths, j, raw, ix, []json.RawMessage{
		json.RawMessage(`"` + hex.EncodeToString(ser) + `"`),
		optsRaw,
	})
	if code != 0 {
		if !strings.HasPrefix(msg, "walletcreatefundedpsbt:") && !strings.HasPrefix(msg, "fundrawtransaction:") {
			msg = "walletcreatefundedpsbt: " + msg
		}
		return nil, code, msg
	}
	fm, ok := fundRes.(map[string]interface{})
	if !ok {
		return nil, -8, "walletcreatefundedpsbt: internal fund error"
	}
	fundedHex, _ := fm["hex"].(string)
	fundedHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(fundedHex), "0x"))
	fundedRaw, err := hex.DecodeString(fundedHex)
	if err != nil {
		return nil, -8, "walletcreatefundedpsbt: funded TX decode failed"
	}
	fundedTx, err := wire.DeserializeTx(fundedRaw)
	if err != nil {
		return nil, -8, "walletcreatefundedpsbt: funded TX decode failed"
	}
	psbt, err := wire.NewPsbtFromTx(fundedTx)
	if err != nil {
		return nil, -8, "walletcreatefundedpsbt: " + err.Error()
	}
	fillPsbtPrevouts(psbt, ix, raw, pool)
	attachWalletPSBTDerivations(chainName, paths, psbt)
	b64, code, msg := encodePSBTBase64(psbt)
	if code != 0 {
		return nil, code, "walletcreatefundedpsbt: " + msg
	}
	out := map[string]interface{}{
		"psbt":      b64,
		"fee":       fm["fee"],
		"changepos": fm["changepos"],
	}
	return out, 0, ""
}

func fundRawTxOptionsToMap(o fundRawTxOptions) map[string]interface{} {
	m := map[string]interface{}{
		"add_inputs":     o.addInputs,
		"lockUnspents":   o.lockUnspents,
		"includeWatching": o.includeWatching,
	}
	if o.changeAddr != "" {
		m["changeAddress"] = o.changeAddr
	}
	if o.changePos >= 0 {
		m["changePosition"] = o.changePos
	}
	if o.feePerKB > 0 {
		m["feeRate"] = float64(o.feePerKB) / 1e8
	}
	if len(o.subtractFeeFrom) > 0 {
		arr := make([]int, len(o.subtractFeeFrom))
		copy(arr, o.subtractFeeFrom)
		m["subtractFeeFromOutputs"] = arr
	}
	if o.minimumTotalFeeKoinu > 0 {
		m["minimumTotalFee"] = float64(o.minimumTotalFeeKoinu) / 1e8
	}
	if o.inputSequence == 0xffffffff {
		m["replaceable"] = false
	}
	return m
}

// execDescriptorProcessPsbt is a Core alias for walletprocesspsbt (descriptor wallet signing).
func execDescriptorProcessPsbt(chainName string, paths *DataPaths, ix *store.TxIndex, raw *store.RawBlockStore, pool *mempool.Pool, params []json.RawMessage) (interface{}, int, string) {
	return execWalletProcessPsbt(chainName, paths, ix, raw, pool, params)
}

// execWalletProcessPsbt fills UTXO data and signs with the built-in wallet (Core walletprocesspsbt subset).
func execWalletProcessPsbt(chainName string, paths *DataPaths, ix *store.TxIndex, raw *store.RawBlockStore, pool *mempool.Pool, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 5 {
		return nil, -32602, "Wrong number of arguments"
	}
	if rpcWalletDefaultAddress(paths) == "" {
		return nil, -1, "walletprocesspsbt: built-in wallet is not available"
	}
	sign := true
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if err := json.Unmarshal(params[1], &sign); err != nil {
			return nil, -8, "walletprocesspsbt: sign must be boolean"
		}
	}
	sighashStr := "ALL"
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		if err := json.Unmarshal(params[2], &sighashStr); err != nil {
			return nil, -8, "walletprocesspsbt: bad sighashtype"
		}
	}
	finalize := true
	if len(params) > 4 && strings.TrimSpace(string(params[4])) != "null" {
		if err := json.Unmarshal(params[4], &finalize); err != nil {
			return nil, -8, "walletprocesspsbt: finalize must be boolean"
		}
	}

	p, code, msg := loadPSBTParam(params)
	if code != 0 {
		if !strings.HasPrefix(msg, "walletprocesspsbt:") {
			msg = "walletprocesspsbt: " + msg
		}
		return nil, code, msg
	}
	fillPsbtPrevouts(p, ix, raw, pool)
	attachWalletPSBTDerivations(chainName, paths, p)

	if sign {
		if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
			return nil, code, msg
		}
		hashType, err := parseSigHashType(sighashStr)
		if err != nil {
			return nil, -8, "walletprocesspsbt: " + err.Error()
		}
		applyWalletSpendTimelocks(p.UnsignedTx, paths)
		signPsbtWithWallet(chainName, paths, p, hashType)
		if err := signPsbtWithExternalSigner(paths, p); err != nil {
			return nil, -1, "walletprocesspsbt: " + err.Error()
		}
	}

	_, complete := p.ExtractedTx()
	out := map[string]interface{}{
		"complete": complete,
	}
	if complete && finalize {
		tx, ok := p.ExtractedTx()
		if ok {
			ser, err := tx.Serialize()
			if err != nil {
				return nil, -8, "walletprocesspsbt: " + err.Error()
			}
			out["hex"] = hex.EncodeToString(ser)
		}
	}
	b64, code, msg := encodePSBTBase64(p)
	if code != 0 {
		return nil, code, "walletprocesspsbt: " + msg
	}
	out["psbt"] = b64
	return out, 0, ""
}

func signPsbtWithWallet(chainName string, paths *DataPaths, p *wire.Psbt, hashType uint32) {
	if p == nil || paths == nil {
		return
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return
	}
	cp, err := chain.ParamsFor(net)
	if err != nil {
		return
	}
	prevMap, err := buildPrevOutMapFromPSBT(p, paths)
	if err != nil || len(prevMap) == 0 {
		return
	}
	keys, err := decodeWIFPrivKeys(rpcWalletWIFs(paths), cp.PrivKeyWIFVersion)
	if err != nil || len(keys) == 0 {
		return
	}
	tx := p.UnsignedTx
	for idx := range tx.Vin {
		if p.InputHasFinalScriptSig(idx) {
			continue
		}
		in := &tx.Vin[idx]
		if isCoinbaseWireIn(in) {
			continue
		}
		key := prevMapKey(in.PrevHash, in.PrevIdx)
		ent, ok := prevMap[key]
		if !ok {
			continue
		}
		spk, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(ent.ScriptPubKey), "0x"))
		if err != nil || len(spk) == 0 {
			continue
		}
		var redeem, innerRedeem []byte
		if ent.RedeemScript != "" {
			redeem, _ = hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(ent.RedeemScript), "0x"))
		}
		if ent.InnerRedeemScript != "" {
			innerRedeem, _ = hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(ent.InnerRedeemScript), "0x"))
		}
		scriptSig, signErr := signInputScript(tx, idx, spk, redeem, innerRedeem, keys, hashType)
		if signErr != nil {
			continue
		}
		p.SetInputFinalScriptSig(idx, scriptSig)
	}
}

func buildPrevOutMapFromPSBT(p *wire.Psbt, paths *DataPaths) (map[string]prevOutEnt, error) {
	if p == nil || p.UnsignedTx == nil {
		return nil, nil
	}
	m := make(map[string]prevOutEnt)
	for i, in := range p.UnsignedTx.Vin {
		if isCoinbaseWireIn(&in) {
			continue
		}
		key := prevMapKey(in.PrevHash, in.PrevIdx)
		if _, ok := m[key]; ok {
			continue
		}
		var spk []byte
		for _, kv := range p.Inputs[i] {
			if kv.Type == wire.PsbtInNonWitnessUtxo {
				parent, err := wire.DeserializeTx(kv.Value)
				if err == nil && int(in.PrevIdx) < len(parent.Vout) {
					spk = append([]byte(nil), parent.Vout[in.PrevIdx].PkScript...)
				}
			}
		}
		if len(spk) == 0 && paths != nil && paths.Utxo != nil {
			if e, ok := paths.Utxo.Lookup(txidToRPC(in.PrevHash), in.PrevIdx); ok {
				spk = append([]byte(nil), e.PkScript...)
			}
		}
		if len(spk) == 0 {
			continue
		}
		ent := prevOutEnt{ScriptPubKey: hex.EncodeToString(spk)}
		for _, kv := range p.Inputs[i] {
			if kv.Type == wire.PsbtInRedeemScript && len(kv.Value) > 0 {
				ent.RedeemScript = hex.EncodeToString(kv.Value)
			}
		}
		if ent.RedeemScript == "" && paths != nil && paths.WalletWatchRedeemScript != nil {
			if redeem := paths.WalletWatchRedeemScript(spk); len(redeem) > 0 {
				ent.RedeemScript = hex.EncodeToString(redeem)
			}
		}
		m[key] = ent
	}
	if len(m) == 0 {
		return nil, nil
	}
	return m, nil
}
