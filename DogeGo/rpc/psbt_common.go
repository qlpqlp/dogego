// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"

	"dogego/wire"
)

func loadPSBTParam(params []json.RawMessage) (*wire.Psbt, int, string) {
	if len(params) < 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var psbtStr string
	if err := json.Unmarshal(params[0], &psbtStr); err != nil {
		return nil, -8, "psbt must be a string"
	}
	raw, code, msg := decodePsbtBlob(psbtStr)
	if code != 0 {
		return nil, code, msg
	}
	p, err := wire.ParsePSBT(raw)
	if err != nil {
		return nil, -8, err.Error()
	}
	return p, 0, ""
}

func encodePSBTBase64(p *wire.Psbt) (string, int, string) {
	raw, err := p.Serialize()
	if err != nil {
		return "", -8, err.Error()
	}
	return base64.StdEncoding.EncodeToString(raw), 0, ""
}

func parsePSBTStringArray(params []json.RawMessage) ([][]byte, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	raw := params[0]
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		var s string
		if err2 := json.Unmarshal(raw, &s); err2 != nil {
			return nil, -8, "must pass a JSON array of PSBT strings"
		}
		if err3 := json.Unmarshal([]byte(strings.TrimSpace(s)), &arr); err3 != nil {
			return nil, -8, "must pass a JSON array of PSBT strings"
		}
	}
	if len(arr) == 0 {
		return nil, -8, "must pass at least one PSBT"
	}
	out := make([][]byte, 0, len(arr))
	for i, psbtStr := range arr {
		blob, code, msg := decodePsbtBlob(psbtStr)
		if code != 0 {
			return nil, code, "PSBT at index " + strconv.Itoa(i) + ": " + msg
		}
		out = append(out, blob)
	}
	return out, 0, ""
}
