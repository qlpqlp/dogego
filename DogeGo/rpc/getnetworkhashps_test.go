// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"dogego/chain"
	"dogego/pow"
)

func TestExecGetNetworkHashPS_genesisOnly(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	res, code, msg := execGetNetworkHashPS(j, nil, nil, chain.RebootTestnet, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if res.(float64) != 0 {
		t.Fatalf("want 0 got %v", res)
	}
}

func TestExecGetNetworkHashPS_twoBlocks(t *testing.T) {
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

	w1, err := pow.BlockProofFromBits(binary.LittleEndian.Uint32(h1[72:76]))
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 1, best: "x", gen: "y", count: 2, hdrs: [][]byte{append([]byte(nil), g80[:]...), h1[:]}}
	res, code, msg := execGetNetworkHashPS(j, nil, nil, chain.RebootTestnet, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	want, _ := new(big.Rat).SetFrac(w1, big.NewInt(120)).Float64()
	got := res.(float64)
	d := got - want
	if d < 0 {
		d = -d
	}
	if d > want*1e-9 && d > 1e-12 {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestExecGetNetworkHashPS_tooManyParams(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 1, best: "x", gen: "y", count: 2, hdrs: [][]byte{g80[:], g80[:]}}
	p1, _ := json.Marshal(1)
	p2, _ := json.Marshal(1)
	p3, _ := json.Marshal(1)
	_, code, msg := execGetNetworkHashPS(j, nil, nil, chain.RebootTestnet, []json.RawMessage{p1, p2, p3})
	if code == 0 || msg == "" {
		t.Fatalf("expected error code=%d msg=%q", code, msg)
	}
}

func TestHandlerGetNetworkHashps(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	tipHash := pow.BlockHashLE(g80[:])
	var h1 [80]byte
	copy(h1[0:4], g80[0:4])
	copy(h1[4:36], tipHash[:])
	copy(h1[36:68], g80[36:68])
	binary.LittleEndian.PutUint32(h1[68:72], binary.LittleEndian.Uint32(g80[68:72])+60)
	copy(h1[72:76], g80[72:76])
	binary.LittleEndian.PutUint32(h1[76:80], binary.LittleEndian.Uint32(g80[76:80])+1)
	j := &memJournal{tip: 1, best: "x", gen: "y", count: 2, hdrs: [][]byte{append([]byte(nil), g80[:]...), h1[:]}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"jsonrpc":"1.0","id":1,"method":"getnetworkhashps","params":[1,-1]}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result float64 `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result <= 0 {
		t.Fatalf("expected positive hashps got %v", out.Result)
	}
}
