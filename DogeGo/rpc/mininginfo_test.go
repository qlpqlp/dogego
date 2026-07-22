// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"testing"

	"dogego/chain"
	"dogego/mempool"
	"dogego/pow"
)

func TestExecGetMiningInfoPrefersNetworkHashPS(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	tipHash := pow.BlockHashLE(g80[:])
	var h1 [80]byte
	copy(h1[0:4], g80[0:4])
	copy(h1[4:36], tipHash[:])
	copy(h1[36:68], g80[36:68])
	binary.LittleEndian.PutUint32(h1[68:72], binary.LittleEndian.Uint32(g80[68:72])+120)
	copy(h1[72:76], g80[72:76])
	binary.LittleEndian.PutUint32(h1[76:80], binary.LittleEndian.Uint32(g80[76:80])+1)
	j := &memJournal{tip: 1, best: "x", gen: "y", count: 2, hdrs: [][]byte{append([]byte(nil), g80[:]...), h1[:]}}
	want, code, msg := execGetNetworkHashPS(j, nil, nil, chain.RebootTestnet, nil)
	if code != 0 || msg != "" {
		t.Fatalf("getnetworkhashps code=%d msg=%q", code, msg)
	}
	wf := want.(float64)
	res, code2, msg2 := execGetMiningInfo(j, mempool.New(0), nil, nil, "test", nil, 0)
	if code2 != 0 || msg2 != "" {
		t.Fatalf("mininginfo code=%d msg=%q", code2, msg2)
	}
	got := res["networkhashps"].(float64)
	if got != wf {
		t.Fatalf("networkhashps got %v want %v", got, wf)
	}
}

func TestExecGetMiningInfoTestnetByChainName(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	for _, tc := range []struct {
		chain string
		want  bool
	}{
		{"test", true},
		{"testnet", true},
		{"main", false},
		{"mainnet", false},
	} {
		res, code, msg := execGetMiningInfo(j, nil, nil, nil, tc.chain, nil, 0)
		if code != 0 || msg != "" {
			t.Fatalf("chain %q code=%d msg=%q", tc.chain, code, msg)
		}
		if res["testnet"].(bool) != tc.want {
			t.Fatalf("chain %q testnet got %v want %v", tc.chain, res["testnet"], tc.want)
		}
	}
}

func TestExecGetMiningInfoNetworkActiveFromPaths(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	paths := &DataPaths{NetworkActive: func() bool { return false }}
	res, code, msg := execGetMiningInfo(j, nil, nil, nil, "main", paths, 0)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if res["networkactive"].(bool) != false {
		t.Fatalf("networkactive got %v", res["networkactive"])
	}
	res2, code2, msg2 := execGetMiningInfo(j, nil, nil, nil, "main", nil, 0)
	if code2 != 0 || msg2 != "" {
		t.Fatalf("code=%d msg=%q", code2, msg2)
	}
	if res2["networkactive"].(bool) != true {
		t.Fatalf("networkactive default got %v", res2["networkactive"])
	}
}

func TestExecGetMiningInfoBlockMaxWeight(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	res, code, msg := execGetMiningInfo(j, nil, nil, nil, "main", nil, 3_000_000)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if int(res["weightlimit"].(int)) != 3_000_000 {
		t.Fatalf("weightlimit %#v", res["weightlimit"])
	}
}
