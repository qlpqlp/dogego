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

// execFinalizePsbt applies final_scriptSig fields and optionally extracts hex (Core finalizepsbt).
func execFinalizePsbt(params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	extract := true
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		if err := json.Unmarshal(params[1], &extract); err != nil {
			return nil, -8, "finalizepsbt: extract must be boolean"
		}
	}
	p, code, msg := loadPSBTParam(params)
	if code != 0 {
		if !strings.HasPrefix(msg, "finalizepsbt:") {
			msg = "finalizepsbt: " + msg
		}
		return nil, code, msg
	}
	tx, complete := p.ExtractedTx()
	out := map[string]interface{}{
		"complete": complete,
	}
	if complete && extract {
		ser, err := tx.Serialize()
		if err != nil {
			return nil, -8, "finalizepsbt: " + err.Error()
		}
		out["hex"] = hex.EncodeToString(ser)
		return out, 0, ""
	}
	b64, code, msg := encodePSBTBase64(p)
	if code != 0 {
		return nil, code, "finalizepsbt: " + msg
	}
	out["psbt"] = b64
	return out, 0, ""
}
