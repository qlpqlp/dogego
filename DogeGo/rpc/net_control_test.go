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
	"dogego/wire"
)

func TestExecPrioritiseTransaction(t *testing.T) {
	p := mempool.New(100)
	_, code, msg := execPrioritiseTransaction(p, nil)
	if code == 0 {
		t.Fatal("expected error for missing params")
	}
	raw := minimalPrioritiseTestRaw(t)
	if err := p.Add(raw); err != nil {
		t.Fatal(err)
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		t.Fatal(err)
	}
	txid := mempool.TxIDDisplayHex(tx.TxHash())
	params := []json.RawMessage{
		json.RawMessage(`"` + txid + `"`),
		json.RawMessage(`0.0`),
		json.RawMessage(`10000`),
	}
	ok, code, msg := execPrioritiseTransaction(p, params)
	if code != 0 || !ok {
		t.Fatalf("code=%d msg=%s ok=%v", code, msg, ok)
	}
	if got := p.FeeDeltaKoinu(txid); got != 10000 {
		t.Fatalf("FeeDeltaKoinu=%d", got)
	}
	ok, code, msg = execPrioritiseTransaction(p, []json.RawMessage{
		json.RawMessage(`"` + txid + `"`),
		json.RawMessage(`0.0`),
		json.RawMessage(`5000.0`),
	})
	if code != 0 || !ok {
		t.Fatalf("float fee_delta code=%d msg=%s", code, msg)
	}
	if got := p.FeeDeltaKoinu(txid); got != 15000 {
		t.Fatalf("cumulative FeeDeltaKoinu=%d", got)
	}
	ghost := "abcd0000abcd0000abcd0000abcd0000abcd0000abcd0000abcd0000abcd0000"
	ok, code, msg = execPrioritiseTransaction(p, []json.RawMessage{
		json.RawMessage(`"` + ghost + `"`),
		json.RawMessage(`0.0`),
		json.RawMessage(`2500`),
	})
	if code != 0 || !ok {
		t.Fatalf("latent mapDelta code=%d msg=%s ok=%v", code, msg, ok)
	}
	if got := p.FeeDeltaKoinu(ghost); got != 2500 {
		t.Fatalf("latent FeeDeltaKoinu=%d", got)
	}
	_, code, _ = execPrioritiseTransaction(p, []json.RawMessage{
		json.RawMessage(`"nothex"`),
		json.RawMessage(`0`),
		json.RawMessage(`0`),
	})
	if code == 0 {
		t.Fatal("expected bad txid")
	}
}

func minimalPrioritiseTestRaw(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, int32(1))
	_ = wire.WriteCompactSize(&buf, 1)
	var zeros [32]byte
	_, _ = buf.Write(zeros[:])
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_, _ = buf.Write([]byte{0x00})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0xffffffff))
	_ = wire.WriteCompactSize(&buf, 1)
	_ = binary.Write(&buf, binary.LittleEndian, int64(8800000000))
	_ = wire.WriteCompactSize(&buf, 2)
	_, _ = buf.Write([]byte{0x51, 0x51})
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	return buf.Bytes()
}

func TestExecPrioritiseTransactionNoPool(t *testing.T) {
	_, code, _ := execPrioritiseTransaction(nil, []json.RawMessage{
		json.RawMessage(`"abcd0000abcd0000abcd0000abcd0000abcd0000abcd0000abcd0000abcd0000"`),
		json.RawMessage(`0`),
		json.RawMessage(`0`),
	})
	if code != -18 {
		t.Fatalf("code %d", code)
	}
}

func TestHandlerListbannedClearbanned(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{BanManager: NewMemoryBanManager()}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	for _, method := range []string{"listbanned", "clearbanned"} {
		body := []byte(`{"jsonrpc":"1.0","id":1,"method":"` + method + `","params":[]}`)
		res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			res.Body.Close()
			t.Fatal(err)
		}
		res.Body.Close()
		if out["error"] != nil {
			t.Fatalf("%s %+v", method, out["error"])
		}
		if method == "listbanned" {
			arr, ok := out["result"].([]interface{})
			if !ok || arr == nil {
				t.Fatalf("listbanned result %#v", out["result"])
			}
			if len(arr) != 0 {
				t.Fatalf("want empty bans")
			}
		} else if out["result"] != nil {
			t.Fatalf("clearbanned want null got %#v", out["result"])
		}
	}
}

func TestExecGetAddedNodeInfoFilterMissing(t *testing.T) {
	_, code, msg := execGetAddedNodeInfo(nil, []json.RawMessage{json.RawMessage(`"1.2.3.4:22556"`)})
	if code == 0 {
		t.Fatal("expected error")
	}
	if msg == "" {
		t.Fatal("message")
	}
}

func TestExecGetAddedNodeInfoWithAddedNodes(t *testing.T) {
	paths := &DataPaths{
		AddedNodes: func() []string { return []string{"peer.example:22556"} },
	}
	out, code, msg := execGetAddedNodeInfo(paths, []json.RawMessage{json.RawMessage(`"peer.example:22556"`)})
	if code != 0 {
		t.Fatalf("%s", msg)
	}
	if len(out) != 1 {
		t.Fatalf("len %d", len(out))
	}
}

func TestExecGetAddedNodeInfoConnected(t *testing.T) {
	paths := &DataPaths{
		AddedNodes: func() []string { return []string{"10.0.0.1:22556"} },
		IsPeerConnected: func(addr string) bool {
			return addr == "10.0.0.1:22556"
		},
	}
	out, code, msg := execGetAddedNodeInfo(paths, nil)
	if code != 0 {
		t.Fatalf("%s", msg)
	}
	m, ok := out[0].(map[string]interface{})
	if !ok || m["connected"] != true {
		t.Fatalf("connected %#v", out[0])
	}
}

func TestExecGetAddedNodeInfoAddresses(t *testing.T) {
	paths := &DataPaths{
		AddedNodes: func() []string { return []string{"10.0.0.1:22556"} },
		IsPeerConnected: func(addr string) bool {
			return addr == "10.0.0.1:22556"
		},
		PeerAddresses: func(added string) []interface{} {
			if added == "10.0.0.1:22556" {
				return []interface{}{
					map[string]interface{}{
						"address":   "10.0.0.1:22556",
						"connected": "outbound",
					},
				}
			}
			return nil
		},
	}
	out, code, msg := execGetAddedNodeInfo(paths, nil)
	if code != 0 {
		t.Fatalf("%s", msg)
	}
	m, ok := out[0].(map[string]interface{})
	if !ok {
		t.Fatal(out[0])
	}
	arr, ok := m["addresses"].([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("addresses %#v", m["addresses"])
	}
}

func TestExecGetAddedNodeInfoBoolLegacy(t *testing.T) {
	out, code, msg := execGetAddedNodeInfo(nil, []json.RawMessage{json.RawMessage(`true`)})
	if code != 0 {
		t.Fatalf("%s", msg)
	}
	if len(out) != 0 {
		t.Fatalf("%#v", out)
	}
}
