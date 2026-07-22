// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/chain"
	"dogego/store"
)

func rebootTestnetMiningH160(t *testing.T, p chain.Params) [20]byte {
	t.Helper()
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	h160, err := p2pkhScriptFromAddress("testnet", addr)
	if err != nil {
		t.Fatal(err)
	}
	return h160
}

func mineRebootTestnetBlock(t *testing.T, j *store.HeaderJournal, h160 [20]byte, maxTries uint64) []byte {
	t.Helper()
	if maxTries == 0 {
		maxTries = DefaultGenerateMaxTries()
	}
	_, payload, err := MineLegacyBlockPayload(j, nil, nil, nil, nil, chain.RebootTestnet, h160, maxTries)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
