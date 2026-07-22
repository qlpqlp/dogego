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

	"dogego/chain"
)

func TestDecodeRPCPrevHashRoundTrip(t *testing.T) {
	var want [32]byte
	want[0] = 0x01
	want[31] = 0xfe
	s := txidToRPC(want)
	got, err := decodeRPCPrevHashHex(s)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %x want %x", got, want)
	}
}

func TestHandlerDecodeScriptP2PKH(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	pk := []byte{0x76, 0xa9, 0x14, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 0x88, 0xac}
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	addr := chain.Base58CheckEncode(p.PubkeyHashAddrID, pk[3:23])
	scriptHex := hex.EncodeToString(pk)
	params, _ := json.Marshal(scriptHex)
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"decodescript","params":[` + string(params) + `]}`)
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
	if out.Result["type"] != "pubkeyhash" {
		t.Fatalf("type %#v", out.Result["type"])
	}
	addrs, _ := out.Result["addresses"].([]interface{})
	if len(addrs) != 1 || addrs[0] != addr {
		t.Fatalf("addresses %#v addr=%q", addrs, addr)
	}
}

func TestHandlerCreateRawTransactionDecodeRoundTrip(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()

	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	inp, _ := json.Marshal([]map[string]interface{}{{"txid": txid, "vout": 0}})
	outObj := map[string]interface{}{addr: 0.5}
	outJSON, _ := json.Marshal(outObj)
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"createrawtransaction","params":[` + string(inp) + `,` + string(outJSON) + `]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result string      `json:"result"`
		Error  interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error != nil {
		t.Fatalf("%+v", out.Error)
	}
	if len(out.Result) < 20 {
		t.Fatalf("hex %q", out.Result)
	}

	params2, _ := json.Marshal(out.Result)
	body2 := []byte(`{"jsonrpc":"1.0","id":2,"method":"decoderawtransaction","params":[` + string(params2) + `]}`)
	res2, err := http.Post(srv.URL, "application/json", bytes.NewReader(body2))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var out2 struct {
		Result map[string]interface{} `json:"result"`
		Error  interface{}            `json:"error"`
	}
	if err := json.NewDecoder(res2.Body).Decode(&out2); err != nil {
		t.Fatal(err)
	}
	if out2.Error != nil {
		t.Fatalf("%+v", out2.Error)
	}
	vins, ok := out2.Result["vin"].([]interface{})
	if !ok || len(vins) != 1 {
		t.Fatalf("vin %#v", out2.Result["vin"])
	}
	vouts, ok := out2.Result["vout"].([]interface{})
	if !ok || len(vouts) != 1 {
		t.Fatalf("vout %#v", out2.Result["vout"])
	}
}
