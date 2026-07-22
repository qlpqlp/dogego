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
	"testing"

	"dogego/consensus"
)

func TestExecDogegoVerifyPQCommitmentScriptHex(t *testing.T) {
	commit := make([]byte, 32)
	commit[0] = 0xab
	script, err := consensus.BuildPQCommitmentScript(consensus.PQTagFalcon, commit)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(hex.EncodeToString(script))
	res, code, msg := execDogegoVerifyPQCommitment([]json.RawMessage{raw})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok || m["valid"] != true || m["tag"] != consensus.PQTagFalcon {
		t.Fatalf("result %#v", res)
	}
}

func TestExecDogegoVerifyPQCommitmentTagAndHex(t *testing.T) {
	commitHex := "ab" + strings.Repeat("00", 31)
	tagRaw, _ := json.Marshal(consensus.PQTagDilithium)
	commitRaw, _ := json.Marshal(commitHex)
	res, code, _ := execDogegoVerifyPQCommitment([]json.RawMessage{tagRaw, commitRaw})
	if code != 0 {
		t.Fatalf("code %d", code)
	}
	m := res.(map[string]interface{})
	if m["tag"] != consensus.PQTagDilithium {
		t.Fatalf("tag %#v", m["tag"])
	}
}

func TestExecDogegoVerifyPQCommitmentReject(t *testing.T) {
	raw, _ := json.Marshal("6a04deadbeef")
	_, code, _ := execDogegoVerifyPQCommitment([]json.RawMessage{raw})
	if code != -8 {
		t.Fatalf("code %d want -8", code)
	}
}
