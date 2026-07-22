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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dogego/pow"
	"dogego/store"
)

func TestExecVerifyChain_genesisOnly(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	res, code, msg := execVerifyChain("test", j, nil, nil, nil, nil, nil, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if res != true {
		t.Fatalf("result %#v", res)
	}
}

func TestExecVerifyChain_linkedTipWindow(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	tipHash := pow.BlockHashLE(g80[:])
	var h1 [80]byte
	copy(h1[0:4], g80[0:4])
	copy(h1[4:36], tipHash[:])
	copy(h1[36:68], g80[36:68])
	binary.LittleEndian.PutUint32(h1[68:72], binary.LittleEndian.Uint32(g80[68:72])+1)
	binary.LittleEndian.PutUint32(h1[72:76], binary.LittleEndian.Uint32(g80[72:76]))
	binary.LittleEndian.PutUint32(h1[76:80], binary.LittleEndian.Uint32(g80[76:80])+1)
	j := &memJournal{tip: 1, best: "x", gen: "y", count: 2, hdrs: [][]byte{append([]byte(nil), g80[:]...), h1[:]}}
	p1, _ := json.Marshal(3)
	p2, _ := json.Marshal(1)
	res, code, msg := execVerifyChain("test", j, nil, nil, nil, nil, nil, []json.RawMessage{p1, p2})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if res != true {
		t.Fatalf("result %#v", res)
	}
}

func TestExecVerifyChain_brokenLink(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	bad := append([]byte(nil), g80[:]...)
	binary.LittleEndian.PutUint32(bad[76:80], binary.LittleEndian.Uint32(bad[76:80])^0x11111111)
	j := &memJournal{tip: 1, best: "x", gen: "y", count: 2, hdrs: [][]byte{append([]byte(nil), g80[:]...), bad}}
	res, code, msg := execVerifyChain("test", j, nil, nil, nil, nil, nil, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if res != false {
		t.Fatalf("want false got %#v", res)
	}
}

func TestExecVerifyChain_badChecklevel(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	p, _ := json.Marshal(5)
	_, code, msg := execVerifyChain("test", j, nil, nil, nil, nil, nil, []json.RawMessage{p})
	if code == 0 {
		t.Fatalf("expected error")
	}
	if msg == "" {
		t.Fatal("empty message")
	}
}

func TestExecVerifyChain_level4RequiresTxIndex(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	rs, err := store.OpenRawBlockStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p1, _ := json.Marshal(4)
	p2, _ := json.Marshal(1)
	_, code, msg := execVerifyChain("test", j, nil, rs, nil, nil, nil, []json.RawMessage{p1, p2})
	if code == 0 {
		t.Fatal("expected error without tx index")
	}
	if msg == "" || !strings.Contains(msg, "tx index") {
		t.Fatalf("msg=%q", msg)
	}
}

func TestExecVerifyChain_verboseReportsFailure(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	bad := append([]byte(nil), g80[:]...)
	binary.LittleEndian.PutUint32(bad[76:80], binary.LittleEndian.Uint32(bad[76:80])^0x22222222)
	j := &memJournal{tip: 1, best: "x", gen: "y", count: 2, hdrs: [][]byte{append([]byte(nil), g80[:]...), bad}}
	p1, _ := json.Marshal(3)
	p2, _ := json.Marshal(1)
	p3, _ := json.Marshal(true)
	_, code, msg := execVerifyChain("test", j, nil, nil, nil, nil, nil, []json.RawMessage{p1, p2, p3})
	if code == 0 {
		t.Fatal("expected verbose error")
	}
	if msg == "" {
		t.Fatal("empty message")
	}
}

func TestHandlerVerifyChain(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"jsonrpc":"1.0","id":1,"method":"verifychain","params":[3,0]}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result bool `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.Result {
		t.Fatal("expected true")
	}
}
