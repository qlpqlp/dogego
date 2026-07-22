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
	"math"
	"strconv"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/wire"
)

const sequenceFinal = 0xffffffff
const sequenceLockTimeEnabled = 0xfffffffe

// buildUnsignedTxFromRPC constructs an unsigned legacy tx (shared by createrawtransaction / createpsbt).
func buildUnsignedTxFromRPC(chainName string, rawInputs []json.RawMessage, rawOutputs map[string]json.RawMessage, lockTime uint32, version int32, replaceable *bool) (*wire.Tx, int, string) {
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}
	if len(rawOutputs) == 0 {
		return nil, -8, "at least one output required"
	}
	if version < 1 || version > 2 {
		return nil, -8, "version must be 1 or 2"
	}

	tx := &wire.Tx{Version: version, LockTime: lockTime}
	for _, rin := range rawInputs {
		var m map[string]interface{}
		if err := json.Unmarshal(rin, &m); err != nil {
			return nil, -8, "bad input object"
		}
		txidStr, _ := m["txid"].(string)
		if strings.TrimSpace(txidStr) == "" {
			return nil, -8, "each input needs txid"
		}
		prev, err := decodeRPCPrevHashHex(txidStr)
		if err != nil {
			return nil, -8, err.Error()
		}
		voutF, ok := m["vout"].(float64)
		if !ok || voutF < 0 || voutF > 1e9 || voutF != float64(uint32(voutF)) {
			return nil, -8, "each input needs integer vout"
		}
		seq, code, msg := inputSequenceForCreate(m, lockTime, replaceable)
		if code != 0 {
			return nil, code, msg
		}
		tx.Vin = append(tx.Vin, wire.TxIn{
			PrevHash: prev,
			PrevIdx:  uint32(voutF),
			Sequence: seq,
		})
	}

	for key, rawVal := range rawOutputs {
		key = strings.TrimSpace(key)
		if key == "pqcommit" {
			var spec struct {
				Tag        string `json:"tag"`
				Commitment string `json:"commitment"`
			}
			if err := json.Unmarshal(rawVal, &spec); err != nil {
				return nil, -8, "pqcommit must be object with tag and commitment"
			}
			spec.Tag = strings.TrimSpace(strings.ToUpper(spec.Tag))
			spec.Commitment = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(spec.Commitment), "0x"))
			if err := consensus.ValidatePQCommitmentHex(spec.Commitment); err != nil {
				return nil, -8, err.Error()
			}
			commit, _ := hex.DecodeString(spec.Commitment)
			pk, err := consensus.BuildPQCommitmentScript(spec.Tag, commit)
			if err != nil {
				return nil, -8, err.Error()
			}
			tx.Vout = append(tx.Vout, wire.TxOut{Value: 0, PkScript: pk})
			continue
		}
		if key == "data" {
			var dataHex string
			if err := json.Unmarshal(rawVal, &dataHex); err != nil {
				return nil, -8, "data output must be hex string"
			}
			dataHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(dataHex), "0x"))
			data, err := hex.DecodeString(dataHex)
			if err != nil {
				return nil, -8, "invalid data hex"
			}
			if len(data) > 80 {
				return nil, -8, "OP_RETURN data exceeds 80 bytes"
			}
			pk, err := buildOpReturnScript(data)
			if err != nil {
				return nil, -8, err.Error()
			}
			tx.Vout = append(tx.Vout, wire.TxOut{Value: 0, PkScript: pk})
			continue
		}
		v, err := outputAmountKoinu(rawVal)
		if err != nil {
			return nil, -8, err.Error()
		}
		if v <= 0 {
			return nil, -8, "amount must be positive"
		}
		ver, h160, err := chain.Base58CheckDecode(key)
		if err != nil {
			return nil, -8, "invalid output address " + key
		}
		var pk []byte
		switch ver {
		case p.PubkeyHashAddrID:
			pk = append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
			pk = append(pk, 0x88, 0xac)
		case p.ScriptHashAddrID:
			pk = append([]byte{0xa9, 0x14}, h160[:]...)
			pk = append(pk, 0x87)
		default:
			return nil, -8, "unsupported address version for " + key
		}
		tx.Vout = append(tx.Vout, wire.TxOut{Value: v, PkScript: pk})
	}
	return tx, 0, ""
}

func inputSequenceForCreate(m map[string]interface{}, lockTime uint32, replaceable *bool) (uint32, int, string) {
	if s, ok := m["sequence"].(float64); ok {
		if s < 0 || s > float64(math.MaxUint32) || s != float64(uint32(s)) {
			return 0, -8, "bad sequence"
		}
		seq := uint32(s)
		if replaceable != nil && *replaceable && seq > wire.MaxBIP125RBFSequence {
			return 0, -8, "sequence number incompatible with replaceable option"
		}
		return seq, 0, ""
	}
	rep := true
	if replaceable != nil {
		rep = *replaceable
	}
	if lockTime != 0 {
		return sequenceLockTimeEnabled, 0, ""
	}
	if rep {
		return wire.MaxBIP125RBFSequence, 0, ""
	}
	return sequenceFinal, 0, ""
}

// parseCreateOutputsParam accepts Core createpsbt outputs as object or array of single-key objects.
func parseCreateOutputsParam(raw json.RawMessage) (map[string]json.RawMessage, int, string) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil && len(obj) > 0 {
		return obj, 0, ""
	}
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, -8, "outputs must be a JSON object or array"
	}
	if len(arr) == 0 {
		return nil, -8, "at least one output required"
	}
	merged := make(map[string]json.RawMessage)
	for i, elem := range arr {
		if len(elem) == 0 {
			return nil, -8, fmt.Sprintf("empty output object at index %d", i)
		}
		if len(elem) > 1 {
			return nil, -8, fmt.Sprintf("output at index %d must have exactly one key", i)
		}
		for k, v := range elem {
			if _, dup := merged[k]; dup {
				return nil, -8, "duplicate output key " + k
			}
			merged[k] = v
		}
	}
	return merged, 0, ""
}

func outputAmountKoinu(raw json.RawMessage) (int64, error) {
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		if f <= 0 || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, fmt.Errorf("bad amount")
		}
		k := int64(math.Round(f * 1e8))
		if k <= 0 {
			return 0, fmt.Errorf("amount too small")
		}
		return k, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, fmt.Errorf("amount must be number or string")
	}
	f2, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f2 <= 0 || math.IsNaN(f2) || math.IsInf(f2, 0) {
		return 0, fmt.Errorf("bad amount string")
	}
	return int64(math.Round(f2 * 1e8)), nil
}

func buildOpReturnScript(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{0x6a, 0x00}, nil
	}
	if len(data) <= 75 {
		out := make([]byte, 0, 2+len(data))
		out = append(out, 0x6a, byte(len(data)))
		out = append(out, data...)
		return out, nil
	}
	out := make([]byte, 0, 3+len(data))
	out = append(out, 0x6a, 0x4c, byte(len(data)))
	out = append(out, data...)
	return out, nil
}
