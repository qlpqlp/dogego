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
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
)

func TestHandlerListStuckTransactionsEmpty(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"liststucktransactions","params":[false,true],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerListStuckTransactionsBadVerbose(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"liststucktransactions","params":["yes"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerFundRawTransactionStub(t *testing.T) {
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
	body1 := []byte(`{"method":"createrawtransaction","params":[` + string(inp) + `,` + string(outJSON) + `],"id":1}`)
	res1, err := http.Post(srv.URL, "application/json", bytes.NewReader(body1))
	if err != nil {
		t.Fatal(err)
	}
	defer res1.Body.Close()
	var rawHex struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(res1.Body).Decode(&rawHex); err != nil {
		t.Fatal(err)
	}
	rawJ, _ := json.Marshal(rawHex.Result)
	opts := map[string]interface{}{"feeRate": 1}
	optsJ, _ := json.Marshal(opts)
	body2 := []byte(`{"method":"fundrawtransaction","params":[` + string(rawJ) + `,` + string(optsJ) + `],"id":2}`)
	res2, err := http.Post(srv.URL, "application/json", bytes.NewReader(body2))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res2.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -1 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerRescanStub(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"rescan","params":[0],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -1 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerRescanBadHeight(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"rescan","params":[-1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListLockUnspentEmpty(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listlockunspent","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerListLockUnspentWrongArity(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listlockunspent","params":[true],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -32602 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerLockUnspentOneArg(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"lockunspent","params":[true],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
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
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerLockUnspentTwoArgsValid(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	body := []byte(`{"method":"lockunspent","params":[false,[{"txid":"` + txid + `","vout":0}]],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
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
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerLockUnspentSecondArgNull(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"lockunspent","params":[true,null],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerLockUnspentBadTxid(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"lockunspent","params":[false,[{"txid":"ff","vout":0}]],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerLockUnspentBadVout(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	body := []byte(`{"method":"lockunspent","params":[false,[{"txid":"` + txid + `","vout":-1}]],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerKeypoolRefillNotImplemented(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"keypoolrefill","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -1 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerKeypoolRefillBadNewsize(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"keypoolrefill","params":[-1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListUnspentEmptyNoParams(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listunspent","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerListUnspentTooManyParams(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listunspent","params":[1,2,[],true,{},1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -32602 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListUnspentBadMinconf(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listunspent","params":["nope"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListUnspentInvalidAddress(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listunspent","params":[1,9999999,["not_a_valid_address"]],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -5 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListUnspentDuplicateAddresses(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	addrJ, err := json.Marshal(addr)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listunspent","params":[1,9999999,[` + string(addrJ) + `,` + string(addrJ) + `]],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListUnspentValidAddressFilter(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	addrJ, err := json.Marshal(addr)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listunspent","params":[1,9999999,[` + string(addrJ) + `],true,{"minimumAmount":0}],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerListUnspentQueryOptionsNotObject(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listunspent","params":[1,2,[],true,[]],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerGetWalletInfoShape(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getwalletinfo","params":[],"id":1}`)
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
	if out.Result["balance"].(float64) != 0 {
		t.Fatalf("balance %#v", out.Result["balance"])
	}
	if int(out.Result["txcount"].(float64)) != 0 {
		t.Fatalf("txcount %#v", out.Result["txcount"])
	}
}

func TestHandlerGetWalletInfoWrongArity(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getwalletinfo","params":[1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -32602 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerGetUnconfirmedBalance(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getunconfirmedbalance","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
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
	if out.Result != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerGetBalanceZero(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getbalance","params":["*",6,true],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
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
	if out.Result != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerGetBalanceBadAccount(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getbalance","params":[1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerGetNewAddressNotImplemented(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getnewaddress","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -1 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListTransactionsEmpty(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listtransactions","params":["*",10,0,false],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerListTransactionsNegativeCount(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listtransactions","params":["*",-1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerGetTransactionNotInWallet(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	body := []byte(`{"method":"gettransaction","params":["` + txid + `",true],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -5 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerGetTransactionBadTxid(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"gettransaction","params":["zz"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerGetReceivedByAddressZero(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	addrJ, err := json.Marshal(addr)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getreceivedbyaddress","params":[` + string(addrJ) + `,0],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
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
	if out.Result != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerGetReceivedByAddressInvalid(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getreceivedbyaddress","params":["not_an_address"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -5 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListReceivedByAddressEmpty(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listreceivedbyaddress","params":[1,false,false],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerSetTxFeeTrue(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"settxfee","params":["0.001"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
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
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerSetTxFeeBadArity(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"settxfee","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -32602 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerGetAccountValid(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	addrJ, err := json.Marshal(addr)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getaccount","params":[` + string(addrJ) + `],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result != "" {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerGetAccountInvalid(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getaccount","params":["not_an_address"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -5 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerGetAccountBadArity(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getaccount","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -32602 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerGetAddressesByAccountEmpty(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getaddressesbyaccount","params":[""],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerGetAddressesByAccountStar(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getaddressesbyaccount","params":["*"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerGetAddressesByAccountBadArity(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getaddressesbyaccount","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -32602 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListAccountsShape(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listaccounts","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result map[string]float64 `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Result) != 1 || out.Result[""] != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerListAccountsBadMinconf(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listaccounts","params":[-1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListAddressGroupingsEmpty(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listaddressgroupings","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerListAddressGroupingsExtraArg(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listaddressgroupings","params":[1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -32602 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListReceivedByAccountEmpty(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listreceivedbyaccount","params":[0,true,false],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerListReceivedByAccountBadMinconfMessage(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listreceivedbyaccount","params":["x"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error.Code != -8 {
		t.Fatalf("code %#v", out.Error.Code)
	}
	if !strings.Contains(out.Error.Message, "listreceivedbyaccount") {
		t.Fatalf("message %#v", out.Error.Message)
	}
}

func TestHandlerGetAccountAddressStar(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"getaccountaddress","params":["*"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerSetAccountNotOwn(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	addrJ, err := json.Marshal(addr)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"setaccount","params":[` + string(addrJ) + `,"tabby"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -1 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerMoveZeroAmount(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"move","params":["","tabby",0],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -3 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerBackupWalletBadArity(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"backupwallet","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -32602 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerWalletLockExtraArg(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"walletlock","params":[true],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -32602 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerListSinceBlockShape(t *testing.T) {
	h0 := make([]byte, 80)
	j := &memJournal{tip: 0, best: "deadbeef", gen: "b", count: 1, hdrs: [][]byte{h0}}
	wantLast := pow.BlockHashHex(h0)
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listsinceblock","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result struct {
			Transactions []interface{} `json:"transactions"`
			Lastblock    string        `json:"lastblock"`
		} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result.Transactions == nil || len(out.Result.Transactions) != 0 {
		t.Fatalf("transactions %#v", out.Result.Transactions)
	}
	if out.Result.Lastblock != wantLast {
		t.Fatalf("lastblock %q want %q", out.Result.Lastblock, wantLast)
	}
}

func TestHandlerListSinceBlockBadTargetConfirms(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"listsinceblock","params":["",0],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerSendToAddressInvalidAddr(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"sendtoaddress","params":["bad",0.1],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -5 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerSendManyEmptyAmounts(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"sendmany","params":["",{}],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -8 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerDumpPrivKeyNotKnown(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	addrJ, err := json.Marshal(addr)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"dumpprivkey","params":[` + string(addrJ) + `],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -4 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerImportPrivKeyBadWIF(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"importprivkey","params":["not_a_wif"],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -5 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerImportPrivKeyValidWIFThenStub(t *testing.T) {
	sk := make([]byte, 32)
	sk[31] = 1
	wif, err := chain.EncodeWIF(sk, 193, true)
	if err != nil {
		t.Fatal(err)
	}
	wifJ, err := json.Marshal(wif)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"importprivkey","params":[` + string(wifJ) + `,"lbl",false],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -1 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerImportAddressP2SHWithAddress(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	addrJ, err := json.Marshal(addr)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"importaddress","params":[` + string(addrJ) + `,"",false,true],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -5 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerResendWalletTransactionsEmpty(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"resendwallettransactions","params":[],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []interface{} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Result == nil || len(out.Result) != 0 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestHandlerSignMessageNoKey(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	addrJ, err := json.Marshal(addr)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	msgJ, _ := json.Marshal("hello")
	body := []byte(`{"method":"signmessage","params":[` + string(addrJ) + `,` + string(msgJ) + `],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Error map[string]interface{} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if int(out.Error["code"].(float64)) != -4 {
		t.Fatalf("code %#v", out.Error["code"])
	}
}

func TestHandlerImportMultiFailureShape(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"method":"importmulti","params":[[{"scriptPubKey":{"address":"x"},"timestamp":"now"}]],"id":1}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Result []struct {
			Success bool `json:"success"`
			Error   struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Result) != 1 || out.Result[0].Success || out.Result[0].Error.Code != -1 {
		t.Fatalf("result %#v", out.Result)
	}
}

func TestExecEncryptWalletArityAndEmptyPassphrase(t *testing.T) {
	_, code, msg := execEncryptWallet(nil)
	if code != -32602 || !strings.Contains(msg, "Wrong number") {
		t.Fatalf("nil params: code=%d msg=%q", code, msg)
	}
	empty, _ := json.Marshal("")
	_, code, msg = execEncryptWallet([]json.RawMessage{empty})
	if code != -8 || !strings.Contains(msg, "empty") {
		t.Fatalf("empty passphrase: code=%d msg=%q", code, msg)
	}
}

func TestExecEncryptWalletNoWalletStub(t *testing.T) {
	p, _ := json.Marshal("secret")
	_, code, msg := execEncryptWallet([]json.RawMessage{p})
	if code != -1 || !strings.Contains(msg, "not implemented") {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecImportPrunedFundsDecodeAndProof(t *testing.T) {
	badTx, _ := json.Marshal("zz")
	goodProof, _ := json.Marshal("aa")
	_, code, msg := execImportPrunedFunds("testnet", nil, nil, []json.RawMessage{badTx, goodProof})
	if code != -22 || msg != "TX decode failed" {
		t.Fatalf("bad hex: code=%d msg=%q", code, msg)
	}

	validTx := hex.EncodeToString(minimalCoinbaseTxBytes(t))
	txJ, _ := json.Marshal(validTx)
	oddProof, _ := json.Marshal("a")
	_, code, msg = execImportPrunedFunds("testnet", nil, nil, []json.RawMessage{txJ, oddProof})
	if code != -8 || !strings.Contains(msg, "txoutproof") {
		t.Fatalf("odd proof: code=%d msg=%q", code, msg)
	}

	_, code, msg = execImportPrunedFunds("testnet", nil, nil, []json.RawMessage{txJ, goodProof})
	if code != -5 {
		t.Fatalf("no chain/proof: code=%d msg=%q", code, msg)
	}
}
