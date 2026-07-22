// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
)

// parseSendFundOptionsJSON unmarshals an optional trailing fundrawtransaction options object.
func parseSendFundOptionsJSON(raw json.RawMessage, method string) (map[string]interface{}, int, string) {
	if strings.TrimSpace(string(raw)) == "" || string(raw) == "null" {
		return nil, 0, ""
	}
	var opts map[string]interface{}
	if err := json.Unmarshal(raw, &opts); err != nil {
		return nil, -8, method + ": fund options must be a JSON object"
	}
	return opts, 0, ""
}
