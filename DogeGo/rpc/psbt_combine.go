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

// execCombinePsbt merges PSBTs with the same unsigned transaction (Core combinepsbt).
func execCombinePsbt(params []json.RawMessage) (interface{}, int, string) {
	blobs, code, msg := parsePSBTStringArray(params)
	if code != 0 {
		if !strings.HasPrefix(msg, "combinepsbt:") {
			msg = "combinepsbt: " + msg
		}
		return nil, code, msg
	}
	psbts := make([]*wire.Psbt, 0, len(blobs))
	for i, raw := range blobs {
		p, err := wire.ParsePSBT(raw)
		if err != nil {
			return nil, -8, "combinepsbt: invalid PSBT at index " + strconv.Itoa(i)
		}
		psbts = append(psbts, p)
	}
	combined, err := wire.CombinePSBT(psbts)
	if err != nil {
		return nil, -8, "combinepsbt: " + err.Error()
	}
	b64, code, msg := encodePSBTBase64(combined)
	if code != 0 {
		return nil, code, "combinepsbt: " + msg
	}
	return b64, 0, ""
}
