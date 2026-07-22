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

	"dogego/consensus"
)

// execDogegoVerifyPQCommitment validates Phase-1 OP_RETURN PQ commitment scripts off-chain.
// Params: [script_hex] or [tag, commitment_hex] (tag FLC1/DIL2/RCG4).
func execDogegoVerifyPQCommitment(params []json.RawMessage) (interface{}, int, string) {
	if len(params) == 0 || len(params) > 2 {
		return nil, -8, "dogego_verifypqcommitment: expected 1 or 2 arguments"
	}
	if len(params) == 1 {
		var scriptHex string
		if err := json.Unmarshal(params[0], &scriptHex); err != nil {
			return nil, -8, "dogego_verifypqcommitment: bad script hex argument"
		}
		out, err := consensus.VerifyPQCommitmentScriptHex(scriptHex)
		if err != nil {
			return nil, -8, err.Error()
		}
		return out, 0, ""
	}
	var tag, commitHex string
	if err := json.Unmarshal(params[0], &tag); err != nil {
		return nil, -8, "dogego_verifypqcommitment: bad tag argument"
	}
	if err := json.Unmarshal(params[1], &commitHex); err != nil {
		return nil, -8, "dogego_verifypqcommitment: bad commitment hex argument"
	}
	if err := consensus.ValidatePQCommitmentHex(strings.TrimSpace(commitHex)); err != nil {
		return nil, -8, err.Error()
	}
	b, _ := hex.DecodeString(commitHex)
	script, err := consensus.BuildPQCommitmentScript(strings.TrimSpace(tag), b)
	if err != nil {
		return nil, -8, err.Error()
	}
	out, err := consensus.VerifyPQCommitmentScriptHex(hex.EncodeToString(script))
	if err != nil {
		return nil, -8, err.Error()
	}
	return out, 0, ""
}
