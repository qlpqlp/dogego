// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// execCreatePsbt builds a PSBT from inputs/outputs (Core createpsbt; legacy txs only).
func execCreatePsbt(chainName string, ix *store.TxIndex, raw *store.RawBlockStore, pool *mempool.Pool, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var rawInputs []json.RawMessage
	if err := json.Unmarshal(params[0], &rawInputs); err != nil {
		return nil, -8, "createpsbt: inputs must be a JSON array"
	}
	outputs, code, msg := parseCreateOutputsParam(params[1])
	if code != 0 {
		return nil, code, "createpsbt: " + msg
	}
	lockTime := uint32(0)
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var lt float64
		if err := json.Unmarshal(params[2], &lt); err != nil {
			return nil, -8, "createpsbt: bad locktime"
		}
		if lt < 0 || lt != float64(uint32(lt)) {
			return nil, -8, "createpsbt: locktime must be uint32"
		}
		lockTime = uint32(lt)
	}
	replaceable := true
	if len(params) > 3 && strings.TrimSpace(string(params[3])) != "null" {
		if err := json.Unmarshal(params[3], &replaceable); err != nil {
			return nil, -8, "createpsbt: replaceable must be boolean"
		}
	}
	version := int32(2)
	if len(params) > 4 && strings.TrimSpace(string(params[4])) != "null" {
		var vf float64
		if err := json.Unmarshal(params[4], &vf); err != nil {
			return nil, -8, "createpsbt: bad version"
		}
		if vf != float64(int32(vf)) {
			return nil, -8, "createpsbt: version must be integer"
		}
		version = int32(vf)
	}

	tx, code, msg := buildUnsignedTxFromRPC(chainName, rawInputs, outputs, lockTime, version, &replaceable)
	if code != 0 {
		if !strings.HasPrefix(msg, "createpsbt:") {
			msg = "createpsbt: " + msg
		}
		return nil, code, msg
	}
	p, err := wire.NewPsbtFromTx(tx)
	if err != nil {
		return nil, -8, "createpsbt: " + err.Error()
	}
	fillPsbtPrevouts(p, ix, raw, pool)
	b64, code, msg := encodePSBTBase64(p)
	if code != 0 {
		return nil, code, "createpsbt: " + msg
	}
	return b64, 0, ""
}

// execConvertToPsbt wraps a hex-encoded legacy transaction as PSBT (Core converttopsbt).
func execConvertToPsbt(params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	var hexStr string
	if err := json.Unmarshal(params[0], &hexStr); err != nil {
		return nil, -8, "converttopsbt: hex must be a string"
	}
	hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	raw, err := hex.DecodeString(hexStr)
	if err != nil || len(raw) == 0 {
		return nil, -8, "converttopsbt: invalid hex"
	}
	permitsigdata := false
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if err := json.Unmarshal(params[1], &permitsigdata); err != nil {
			return nil, -8, "converttopsbt: permitsigdata must be boolean"
		}
	}
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var isWitness bool
		if err := json.Unmarshal(params[2], &isWitness); err != nil {
			return nil, -8, "converttopsbt: iswitness must be boolean"
		}
		if isWitness {
			return nil, -8, "converttopsbt: witness transactions are not supported in DogeGo"
		}
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return nil, -8, "converttopsbt: " + err.Error()
	}
	if tx.HasWitness() {
		return nil, -8, "converttopsbt: witness transactions are not supported in DogeGo"
	}
	if !permitsigdata {
		for i, in := range tx.Vin {
			if len(in.Script) > 0 {
				return nil, -8, "converttopsbt: input "+strconv.Itoa(i)+" has scriptSig; set permitsigdata true to strip"
			}
		}
	} else {
		for i := range tx.Vin {
			tx.Vin[i].Script = nil
			tx.Vin[i].Witness = nil
		}
	}
	p, err := wire.NewPsbtFromTx(tx)
	if err != nil {
		return nil, -8, "converttopsbt: " + err.Error()
	}
	b64, code, msg := encodePSBTBase64(p)
	if code != 0 {
		return nil, code, "converttopsbt: " + msg
	}
	return b64, 0, ""
}
