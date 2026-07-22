// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dogego/secp256k1"

	"dogego/chain"
)

func TestExecCreateMultisig2of2Compressed(t *testing.T) {
	k1, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	k2, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	p1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	p2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	params := []json.RawMessage{
		json.RawMessage(`2`),
		json.RawMessage(`[` + string(mustJSON(t, p1)) + `,` + string(mustJSON(t, p2)) + `]`),
	}
	res, code, msg := execCreateMultisig("test", params)
	if code != 0 {
		t.Fatalf("code=%d %s", code, msg)
	}
	addr, _ := res["address"].(string)
	if addr == "" {
		t.Fatalf("address %#v", res)
	}
	redeemHex, _ := res["redeemScript"].(string)
	redeem, err := hex.DecodeString(redeemHex)
	if err != nil {
		t.Fatal(err)
	}
	if len(redeem) < 5 || redeem[0] != 0x52 { // OP_2
		t.Fatalf("redeem prefix %x", redeem[:minLen(5, len(redeem))])
	}
	if redeem[len(redeem)-1] != 0xae {
		t.Fatalf("want CHECKMULTISIG last byte %x", redeem[len(redeem)-1])
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	h := scriptHash160(redeem)
	want := chain.Base58CheckEncode(p.ScriptHashAddrID, h[:])
	if want != addr {
		t.Fatalf("addr got %s want %s", addr, want)
	}
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestExecCreateMultisigDuplicateKey(t *testing.T) {
	k1, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	p1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	params := []json.RawMessage{
		json.RawMessage(`2`),
		json.RawMessage(`[` + string(mustJSON(t, p1)) + `,` + string(mustJSON(t, p1)) + `]`),
	}
	_, code, msg := execCreateMultisig("test", params)
	if code == 0 {
		t.Fatal("expected duplicate key error")
	}
	if msg != "createmultisig: duplicate key" {
		t.Fatalf("msg %q", msg)
	}
}

func TestExecCreateMultisigInvalidKey(t *testing.T) {
	k1, _ := secp256k1.NewPrivateKey()
	p1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	params := []json.RawMessage{
		json.RawMessage(`2`),
		json.RawMessage(`[` + string(mustJSON(t, p1)) + `,"notahexkey"]`),
	}
	_, code, msg := execCreateMultisig("test", params)
	if code == 0 {
		t.Fatal("expected error")
	}
	if msg == "" {
		t.Fatal("expected message")
	}
}

func TestHandlerGetinfo(t *testing.T) {
	j := &memJournal{tip: 3, best: "b", gen: "g", count: 4, hdrs: [][]byte{bytes.Repeat([]byte{0}, 80), bytes.Repeat([]byte{0}, 80), bytes.Repeat([]byte{0}, 80), func() []byte {
		h := bytes.Repeat([]byte{0}, 80)
		h[72] = 0xf0
		h[73] = 0xff
		h[74] = 0x0f
		h[75] = 0x1e
		return h
	}()}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"getinfo","params":[]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result map[string]interface{} `json:"result"`
		Error  interface{}            `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error %+v", out.Error)
	}
	if out.Result["testnet"] != true {
		t.Fatalf("testnet %#v", out.Result["testnet"])
	}
	if out.Result["blocks"].(float64) != 3 {
		t.Fatalf("blocks %#v", out.Result["blocks"])
	}
	if _, ok := out.Result["difficulty"].(float64); !ok {
		t.Fatalf("difficulty %#v", out.Result["difficulty"])
	}
}

func TestHandlerCreatemultisig(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	p1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	p2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	params := `2, [` + string(mustJSON(t, p1)) + `,` + string(mustJSON(t, p2)) + `]`
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"createmultisig","params":[` + params + `]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result map[string]interface{} `json:"result"`
		Error  interface{}            `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("error %+v", out.Error)
	}
	if out.Result["address"] == nil || out.Result["redeemScript"] == nil {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestExecCreateMultisigWrongNRequired(t *testing.T) {
	k1, _ := secp256k1.NewPrivateKey()
	p1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	params := []json.RawMessage{
		json.RawMessage(`3`),
		json.RawMessage(`[` + string(mustJSON(t, p1)) + `]`),
	}
	_, code, _ := execCreateMultisig("test", params)
	if code == 0 {
		t.Fatal("expected error")
	}
}
