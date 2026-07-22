// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"

	"dogego/chain"
)

func TestExecSendToAddressNoWallet(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	dest, _ := json.Marshal(addr)
	_, code, msg := execSendToAddress("test", nil, nil, nil, nil, nil, []json.RawMessage{
		dest,
		json.RawMessage(`0.1`),
	}, nil, false, 0)
	if code != -1 || msg == "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecSendToAddressInvalidAddress(t *testing.T) {
	paths := &DataPaths{
		WalletAddress: func() string { return "naddr" },
	}
	_, code, _ := execSendToAddress("test", paths, nil, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`"not-an-address"`),
		json.RawMessage(`1`),
	}, nil, false, 0)
	if code != -5 {
		t.Fatalf("expected -5 got %d", code)
	}
}
