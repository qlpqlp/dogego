// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strconv"
	"strings"

	"dogego/wire"
)

// execJoinPsbt joins distinct PSBTs into one (Core joinpsbts).
func execJoinPsbt(params []json.RawMessage) (interface{}, int, string) {
	blobs, code, msg := parsePSBTStringArray(params)
	if code != 0 {
		if !strings.HasPrefix(msg, "joinpsbts:") {
			msg = "joinpsbts: " + msg
		}
		return nil, code, msg
	}
	psbts := make([]*wire.Psbt, len(blobs))
	for i, raw := range blobs {
		p, err := wire.ParsePSBT(raw)
		if err != nil {
			return nil, -8, "joinpsbts: invalid PSBT at index " + strconv.Itoa(i)
		}
		psbts[i] = p
	}
	joined, err := wire.JoinPSBT(psbts)
	if err != nil {
		return nil, -8, "joinpsbts: " + err.Error()
	}
	b64, code, msg := encodePSBTBase64(joined)
	if code != 0 {
		return nil, code, "joinpsbts: " + msg
	}
	return b64, 0, ""
}
