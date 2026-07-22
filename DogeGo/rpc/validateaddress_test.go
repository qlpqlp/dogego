// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dogego/chain"
)

func TestHandlerValidateAddress_validTestnet(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	params, _ := json.Marshal(addr)
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"validateaddress","params":[` + string(params) + `]}`)
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
		t.Fatalf("%+v", out.Error)
	}
	if out.Result == nil || out.Result["isvalid"] != true {
		t.Fatalf("%+v", out.Result)
	}
	spk, ok := out.Result["scriptPubKey"].(map[string]interface{})
	if !ok {
		t.Fatalf("scriptPubKey %#v", out.Result["scriptPubKey"])
	}
	addrs, ok := spk["addresses"].([]interface{})
	if !ok || len(addrs) != 1 || addrs[0].(string) != addr {
		t.Fatalf("addresses %#v want [%q]", spk["addresses"], addr)
	}
	if spk["address"].(string) != addr {
		t.Fatalf("scriptPubKey.address %#v", spk["address"])
	}
	if out.Result["witness_version"] != nil || out.Result["witness_program"] != nil {
		t.Fatalf("witness fields %#v %#v", out.Result["witness_version"], out.Result["witness_program"])
	}
	if out.Result["ismine"].(bool) != false || out.Result["iswatchonly"].(bool) != false {
		t.Fatalf("ismine %#v iswatchonly %#v", out.Result["ismine"], out.Result["iswatchonly"])
	}
}

func TestHandlerValidateAddress_invalid(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	params, _ := json.Marshal("not_an_address")
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"validateaddress","params":[` + string(params) + `]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || out.Result["isvalid"] != false {
		t.Fatalf("%+v", out.Result)
	}
	if out.Result["ismine"].(bool) != false || out.Result["iswatchonly"].(bool) != false {
		t.Fatalf("ismine %#v iswatchonly %#v", out.Result["ismine"], out.Result["iswatchonly"])
	}
}

func TestHandlerValidateAddress_wrongNetworkVersion(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1}
	srv := httptest.NewServer(Handler("main", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	pTN, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	var h [20]byte
	h[0] = 0xab
	addr := chain.Base58CheckEncode(pTN.PubkeyHashAddrID, h[:])
	params, _ := json.Marshal(addr)
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"validateaddress","params":[` + string(params) + `]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || out.Result["isvalid"] != false {
		t.Fatalf("testnet address on main RPC should be invalid, got %+v", out.Result)
	}
}

func TestHandlerValidateAddress_P2SH(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	var h [20]byte
	h[0] = 0x55
	addr := chain.Base58CheckEncode(p.ScriptHashAddrID, h[:])
	params, _ := json.Marshal(addr)
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"validateaddress","params":[` + string(params) + `]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result map[string]interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result["isvalid"] != true {
		t.Fatalf("%+v", out.Result)
	}
	spk, _ := out.Result["scriptPubKey"].(map[string]interface{})
	if spk["type"] != "scripthash" {
		t.Fatalf("type %#v", spk["type"])
	}
}

func TestImportWatchScriptArgP2SHAddress(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	var h [20]byte
	h[1] = 0xab
	addr := chain.Base58CheckEncode(p.ScriptHashAddrID, h[:])
	pk, code, msg := importWatchScriptArg("test", addr, false)
	if code != 0 || len(pk) != 23 {
		t.Fatalf("code=%d msg=%s len=%d", code, msg, len(pk))
	}
	if chain.PayToScriptHashAddress(pk, p.ScriptHashAddrID) != addr {
		t.Fatalf("roundtrip addr")
	}
}

func TestValidateAddressString(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	m, code, msg := ValidateAddressString("testnet", addr)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if m["isvalid"] != true {
		t.Fatalf("expected valid %#v", m)
	}
}

