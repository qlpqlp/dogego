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
)

// execCreateRawTransaction builds an unsigned legacy transaction (Core createrawtransaction subset).
// Supported outputs: P2PKH/P2SH addresses, {"data":"hex"} OP_RETURN (≤80 bytes), or {"pqcommit":{"tag":"FLC1","commitment":"<64 hex>"}}.
// Inputs: [{"txid":"...","vout":n,"sequence":optional}, ...]. Optional locktime as third param (default 0).
func execCreateRawTransaction(chainName string, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 2 {
		return nil, -8, "createrawtransaction: inputs and outputs required"
	}
	var rawInputs []json.RawMessage
	if err := json.Unmarshal(params[0], &rawInputs); err != nil {
		return nil, -8, "createrawtransaction: inputs must be a JSON array"
	}
	var rawOutputs map[string]json.RawMessage
	if err := json.Unmarshal(params[1], &rawOutputs); err != nil {
		return nil, -8, "createrawtransaction: outputs must be a JSON object"
	}
	lockTime := uint32(0)
	if len(params) >= 3 && string(params[2]) != "null" {
		var lt float64
		if err := json.Unmarshal(params[2], &lt); err != nil {
			return nil, -8, "createrawtransaction: bad locktime"
		}
		if lt < 0 || lt > float64(^uint32(0)) || lt != float64(uint32(lt)) {
			return nil, -8, "createrawtransaction: locktime must be uint32"
		}
		lockTime = uint32(lt)
	}
	tx, code, msg := buildUnsignedTxFromRPC(chainName, rawInputs, rawOutputs, lockTime, 1, nil)
	if code != 0 {
		if !strings.HasPrefix(msg, "createrawtransaction:") {
			msg = "createrawtransaction: " + msg
		}
		return nil, code, msg
	}
	ser, err := tx.Serialize()
	if err != nil {
		return nil, -8, err.Error()
	}
	return hex.EncodeToString(ser), 0, ""
}
