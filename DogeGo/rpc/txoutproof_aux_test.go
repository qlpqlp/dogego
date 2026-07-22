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
	"testing"

	"dogego/mempool"
	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

func TestHandlerGetTxOutProofAuxPowBlock(t *testing.T) {
	mainTx := minimalCoinbaseTxBytes(t)
	cbTx, err := wire.DeserializeTx(mainTx)
	if err != nil {
		t.Fatal(err)
	}
	th := cbTx.TxHash()
	hdr := primitives.BlockHeader{
		Version:    1 | (1 << 8),
		PrevBlock:  [32]byte{},
		MerkleRoot: th,
		Timestamp:  1747000000,
		Bits:       0x1e0ffff0,
		Nonce:      2139303,
	}
	h80 := hdr.EncodeWire80()
	var aux bytes.Buffer
	inner := minimalCoinbaseTxBytes(t)
	_, _ = aux.Write(inner)
	var z [32]byte
	_, _ = aux.Write(z[:])
	_ = wire.WriteCompactSize(&aux, 0)
	_ = binary.Write(&aux, binary.LittleEndian, int32(-1))
	_ = wire.WriteCompactSize(&aux, 0)
	_ = binary.Write(&aux, binary.LittleEndian, int32(0))
	var parent [80]byte
	_, _ = aux.Write(parent[:])
	var block bytes.Buffer
	_, _ = block.Write(h80[:])
	_, _ = block.Write(aux.Bytes())
	_ = wire.WriteCompactSize(&block, 1)
	_, _ = block.Write(mainTx)

	id0 := pow.BlockHashLE(h80[:])
	dir := t.TempDir()
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Put(id0, block.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := ix.IndexBlock(id0, block.Bytes()); err != nil {
		t.Fatal(err)
	}
	best := pow.BlockHashHex(h80[:])
	j := &memJournal{tip: 0, best: best, gen: best, count: 1, hdrs: [][]byte{append([]byte(nil), h80[:]...)}}
	srv := httptest.NewServer(Handler("test", j, mempool.New(1), nil, rs, ix, nil, true, nil))
	defer srv.Close()

	txid := txidToRPC(th)
	txidsJSON, _ := json.Marshal([]string{txid})
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"gettxoutproof","params":[` + string(txidsJSON) + `]}`)
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
		t.Fatalf("gettxoutproof error: %+v", out.Error)
	}
	if len(out.Result) < 160 {
		t.Fatalf("proof too short: len=%d", len(out.Result))
	}
	proofJSON, _ := json.Marshal(out.Result)
	body2 := []byte(`{"jsonrpc":"1.0","id":2,"method":"verifytxoutproof","params":[` + string(proofJSON) + `]}`)
	res2, err := http.Post(srv.URL, "application/json", bytes.NewReader(body2))
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var out2 struct {
		Result []interface{} `json:"result"`
		Error  interface{}   `json:"error"`
	}
	if err := json.NewDecoder(res2.Body).Decode(&out2); err != nil {
		t.Fatal(err)
	}
	if out2.Error != nil {
		t.Fatalf("verify error: %+v", out2.Error)
	}
	if len(out2.Result) != 1 || out2.Result[0] != txid {
		t.Fatalf("verify result %#v want [%s]", out2.Result, txid)
	}
}
