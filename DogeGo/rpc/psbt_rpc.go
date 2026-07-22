// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"

	"dogego/chain"
	"dogego/wire"
)

// execDecodePsbt returns Core-shaped JSON for a PSBT (BIP-174; legacy txs only).
func execDecodePsbt(chainName string, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var isWitness bool
		if err := json.Unmarshal(params[1], &isWitness); err != nil {
			return nil, -8, "decodepsbt: iswitness must be boolean"
		}
		if isWitness {
			return nil, -8, "decodepsbt: witness PSBT is not supported in DogeGo"
		}
	}
	var psbtStr string
	if err := json.Unmarshal(params[0], &psbtStr); err != nil {
		return nil, -8, "decodepsbt: psbt must be a string"
	}
	raw, code, msg := decodePsbtBlob(psbtStr)
	if code != 0 {
		return nil, code, msg
	}
	p, err := wire.ParsePSBT(raw)
	if err != nil {
		return nil, -8, "decodepsbt: " + err.Error()
	}
	return psbtToRPCJSON(chainName, p)
}

func decodePsbtBlob(s string) ([]byte, int, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, -8, "decodepsbt: empty psbt"
	}
	lower := strings.ToLower(strings.TrimPrefix(s, "0x"))
	if isHexString(lower) {
		raw, err := hex.DecodeString(lower)
		if err != nil {
			return nil, -8, "decodepsbt: invalid hex"
		}
		return raw, 0, ""
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, -8, "decodepsbt: invalid base64 or hex"
	}
	return raw, 0, ""
}

func isHexString(s string) bool {
	if len(s) == 0 || len(s)%2 != 0 {
		return false
	}
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

func psbtToRPCJSON(chainName string, p *wire.Psbt) (interface{}, int, string) {
	txJSON, err := txToRPCJSONChain(p.UnsignedTx, chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	out := map[string]interface{}{
		"tx":     txJSON,
		"inputs": psbtInputMapsJSON(chainName, p),
		"outputs": psbtOutputMapsJSON(p),
	}
	if p.Version != 0 {
		out["psbt_version"] = p.Version
	}
	return out, 0, ""
}

func psbtInputMapsJSON(chainName string, p *wire.Psbt) []interface{} {
	rows := make([]interface{}, len(p.Inputs))
	for i, m := range p.Inputs {
		rows[i] = psbtInMapJSON(chainName, m)
	}
	return rows
}

func psbtOutputMapsJSON(p *wire.Psbt) []interface{} {
	rows := make([]interface{}, len(p.Outputs))
	for i, m := range p.Outputs {
		rows[i] = psbtOutMapJSON(m)
	}
	return rows
}

func psbtInMapJSON(chainName string, m []wire.PsbtKeyValue) map[string]interface{} {
	row := make(map[string]interface{})
	partials := make(map[string]interface{})
	for _, kv := range m {
		switch kv.Type {
		case wire.PsbtInNonWitnessUtxo:
			if tx, err := wire.DeserializeTx(kv.Value); err == nil {
				if j, err := txToRPCJSONChain(tx, chainName); err == nil {
					row["non_witness_utxo"] = j
				} else {
					row["non_witness_utxo"] = hex.EncodeToString(kv.Value)
				}
			} else {
				row["non_witness_utxo"] = hex.EncodeToString(kv.Value)
			}
		case wire.PsbtInWitnessUtxo:
			row["witness_utxo"] = psbtWitnessUtxoJSON(chainName, kv.Value)
		case wire.PsbtInPartialSig:
			partials[hex.EncodeToString(kv.Subkey)] = hex.EncodeToString(kv.Value)
		case wire.PsbtInSighash:
			if len(kv.Value) == 4 {
				row["sighash"] = hex.EncodeToString(kv.Value)
			}
		case wire.PsbtInRedeemScript:
			row["redeem_script"] = hex.EncodeToString(kv.Value)
		case wire.PsbtInWitnessScript:
			row["witness_script"] = hex.EncodeToString(kv.Value)
		case wire.PsbtInFinalScriptSig:
			row["final_scriptsig"] = hex.EncodeToString(kv.Value)
		case wire.PsbtInFinalScriptWit:
			row["final_scriptwitness"] = psbtFinalWitnessJSON(kv.Value)
		case wire.PsbtInBIP32Derivation:
			row["bip32_derivs"] = appendBIP32Deriv(row["bip32_derivs"], kv)
		default:
			row[psbtUnknownKey("in", kv.Type)] = hex.EncodeToString(kv.Value)
		}
	}
	if len(partials) > 0 {
		row["partial_signatures"] = partials
	}
	return row
}

func psbtOutMapJSON(m []wire.PsbtKeyValue) map[string]interface{} {
	row := make(map[string]interface{})
	for _, kv := range m {
		switch kv.Type {
		case wire.PsbtOutRedeemScript:
			row["redeem_script"] = hex.EncodeToString(kv.Value)
		case wire.PsbtOutWitnessScript:
			row["witness_script"] = hex.EncodeToString(kv.Value)
		case wire.PsbtOutBIP32Derivation:
			row["bip32_derivs"] = appendBIP32Deriv(row["bip32_derivs"], kv)
		default:
			row[psbtUnknownKey("out", kv.Type)] = hex.EncodeToString(kv.Value)
		}
	}
	return row
}

func psbtWitnessUtxoJSON(chainName string, val []byte) interface{} {
	if len(val) < 9 {
		return hex.EncodeToString(val)
	}
	amount := int64(uint64(val[0]) | uint64(val[1])<<8 | uint64(val[2])<<16 | uint64(val[3])<<24 |
		uint64(val[4])<<32 | uint64(val[5])<<40 | uint64(val[6])<<48 | uint64(val[7])<<56)
	script := val[8:]
	spk := map[string]interface{}{"hex": hex.EncodeToString(script)}
	if net, err := networkFromRPCChainName(chainName); err == nil {
		if p, err := chain.ParamsFor(net); err == nil {
			spk = scriptPubKeyRPC(script, p)
		}
	}
	return map[string]interface{}{
		"amount":       float64(amount) / 1e8,
		"scriptPubKey": spk,
	}
}

func psbtFinalWitnessJSON(val []byte) interface{} {
	return hex.EncodeToString(val)
}

func appendBIP32Deriv(cur interface{}, kv wire.PsbtKeyValue) []interface{} {
	var list []interface{}
	if cur != nil {
		if l, ok := cur.([]interface{}); ok {
			list = l
		}
	}
	entry := map[string]interface{}{
		"pubkey": hex.EncodeToString(kv.Subkey),
	}
	if len(kv.Value) >= 4 {
		entry["master_fingerprint"] = hex.EncodeToString(kv.Value[:4])
		var path []interface{}
		for i := 4; i+4 <= len(kv.Value); i += 4 {
			idx := uint32(kv.Value[i]) | uint32(kv.Value[i+1])<<8 | uint32(kv.Value[i+2])<<16 | uint32(kv.Value[i+3])<<24
			path = append(path, int(idx))
		}
		if len(path) > 0 {
			entry["path"] = path
		}
	}
	list = append(list, entry)
	return list
}

func psbtUnknownKey(prefix string, typ byte) string {
	return prefix + "_unknown_" + hex.EncodeToString([]byte{typ})
}
