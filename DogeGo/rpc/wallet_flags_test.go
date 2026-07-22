// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSetWalletFlagAvoidReuse(t *testing.T) {
	var set bool
	paths := &DataPaths{
		WalletAddress: func() string { return "DAddr" },
		WalletSetAvoidReuse: func(v bool) error {
			set = v
			return nil
		},
	}
	v, _ := json.Marshal(true)
	res, code, msg := execSetWalletFlag(paths, []json.RawMessage{
		json.RawMessage(`"avoid_reuse"`),
		v,
	})
	if code != 0 || res != true || !set {
		t.Fatalf("res=%v code=%d msg=%s set=%v", res, code, msg, set)
	}
}

func TestSetWalletFlagPqCommitments(t *testing.T) {
	var set bool
	paths := &DataPaths{
		WalletAddress: func() string { return "DAddr" },
		WalletSetAvoidReuse: func(bool) error { return nil },
		WalletSetPqCommitmentsEnabled: func(v bool) error {
			set = v
			return nil
		},
	}
	v, _ := json.Marshal(true)
	res, code, msg := execSetWalletFlag(paths, []json.RawMessage{
		json.RawMessage(`"pq_commitments"`),
		v,
	})
	if code != 0 || res != true || !set {
		t.Fatalf("res=%v code=%d msg=%s set=%v", res, code, msg, set)
	}
}

func TestPeelPQCommitRequiresFlag(t *testing.T) {
	opts := map[string]interface{}{
		"pqcommit": map[string]interface{}{"tag": "FLC1", "commitment": strings.Repeat("ab", 32)},
	}
	_, code, msg := peelPQCommitFromSendOptions(opts, &DataPaths{}, "sendtoaddress")
	if code != -8 || !strings.Contains(msg, "pq_commitments") {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}
