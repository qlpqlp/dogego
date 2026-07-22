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
	"dogego/store"
	"dogego/wire"
)

func TestActiveChainFromJournalNoRawStore(t *testing.T) {
	j := &memJournal{tip: 7, best: "b", gen: "g", count: 8, hdrs: make([][]byte, 8)}
	for i := range j.hdrs {
		j.hdrs[i] = make([]byte, 80)
	}
	blocks, headers, cont := activeChainFromJournal(j, nil)
	if blocks != 7 || headers != 7 || cont != -1 {
		t.Fatalf("blocks=%d headers=%d cont=%d", blocks, headers, cont)
	}
}

func TestActiveChainFromJournalUsesContiguousCache(t *testing.T) {
	j := &memJournal{tip: 99_999, best: "b", gen: "g", count: 100_000, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{ContiguousRawHeight: func() int64 { return 42 }}
	blocks, headers, cont := activeChainFromJournal(j, &store.RawBlockStore{}, paths)
	if headers != 99_999 || cont != 42 || blocks != 42 {
		t.Fatalf("blocks=%d headers=%d cont=%d", blocks, headers, cont)
	}
}

func TestBlockHeaderJSONConfirmationsChainActive(t *testing.T) {
	h80 := make([]byte, 80)
	j := &memJournal{tip: 100, best: "b", gen: "g", count: 101, hdrs: [][]byte{h80}}
	m := blockHeaderJSON(j, h80, 90, "0", nil, 95)
	conf, ok := m["confirmations"].(int64)
	if !ok {
		conf = int64(m["confirmations"].(float64))
	}
	if conf != 6 {
		t.Fatalf("confirmations=%d want 6 (chainActive 95 - height 90 + 1)", conf)
	}
}

func TestConfirmationsAtChainActive(t *testing.T) {
	if c := confirmationsAtChainActive(nil, nil, 5); c != 0 {
		t.Fatalf("nil journal got %d", c)
	}
	h80 := make([]byte, 80)
	j := &memJournal{tip: 10, best: "b", gen: "g", count: 11, hdrs: make([][]byte, 11)}
	for i := range j.hdrs {
		j.hdrs[i] = append([]byte(nil), h80...)
	}
	if c := confirmationsAtChainActive(j, nil, 10); c != 1 {
		t.Fatalf("tip conf=%d want 1", c)
	}
	if c := confirmationsAtChainActive(j, nil, 11); c != 0 {
		t.Fatalf("ahead conf=%d want 0", c)
	}
}

func TestRawTxChainMetaIBD(t *testing.T) {
	h80 := make([]byte, 80)
	j := &memJournal{tip: 50, best: "b", gen: "g", count: 51, hdrs: make([][]byte, 51)}
	for i := range j.hdrs {
		j.hdrs[i] = append([]byte(nil), h80...)
	}
	conf, _, inActive := rawTxChainMeta(j, nil, 40)
	if conf != 11 {
		t.Fatalf("confirmations=%d want 11 (tip 50, block 40)", conf)
	}
	if !inActive {
		t.Fatal("height 40 should be in active chain when tip is header-only 50")
	}
	conf2, _, inActive2 := rawTxChainMeta(j, nil, 55)
	if conf2 != 0 || inActive2 {
		t.Fatalf("ahead-of-tip: conf=%d inActive=%v", conf2, inActive2)
	}
}

func TestExecListSinceBlockLastBlockChainActive(t *testing.T) {
	h80 := make([]byte, 80)
	j := &memJournal{tip: 9, best: "headerbest", gen: "g", count: 10, hdrs: make([][]byte, 10)}
	for i := range j.hdrs {
		j.hdrs[i] = append([]byte(nil), h80...)
	}
	res, code, msg := execListSinceBlock(j, nil, nil, nil)
	if code != 0 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("result %#v", res)
	}
	want := pow.BlockHashHex(h80)
	if m["lastblock"] != want {
		t.Fatalf("lastblock %q want computed tip hash %q when no raw store", m["lastblock"], want)
	}
}

func TestActiveChainBlockHeightExport(t *testing.T) {
	j := &memJournal{tip: 20, best: "b", gen: "g", count: 21, hdrs: make([][]byte, 21)}
	if h := ActiveChainBlockHeight(j, nil); h != 20 {
		t.Fatalf("nil raw got %d want header tip 20", h)
	}
}

func TestChainActiveTipNilRawUsesHeaderTip(t *testing.T) {
	h80 := make([]byte, 80)
	j := &memJournal{tip: 4, best: "b", gen: "g", count: 5, hdrs: make([][]byte, 5)}
	for i := range j.hdrs {
		j.hdrs[i] = append([]byte(nil), h80...)
	}
	h, hash, err := ChainActiveTip(j, nil)
	if err != nil {
		t.Fatal(err)
	}
	if h != 4 {
		t.Fatalf("height=%d want 4", h)
	}
	if hash == "" {
		t.Fatal("empty hash")
	}
}

func TestResolveGetBlockHeaderDefaultUsesChainActive(t *testing.T) {
	h80 := make([]byte, 80)
	j := &memJournal{tip: 9, best: "b", gen: "g", count: 10, hdrs: make([][]byte, 10)}
	for i := range j.hdrs {
		j.hdrs[i] = append([]byte(nil), h80...)
	}
	_, height, err := resolveGetBlockHeader(j, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if height != 9 {
		t.Fatalf("nil raw default height=%d want header tip 9", height)
	}
}

func TestActiveChainBlockHeight(t *testing.T) {
	if g := activeChainBlockHeight(100, 40, true, -1); g != 40 {
		t.Fatalf("ibd got %d want 40", g)
	}
	if g := activeChainBlockHeight(100, -1, true, -1); g != 0 {
		t.Fatalf("no bodies got %d want 0", g)
	}
	if g := activeChainBlockHeight(100, 100, true, -1); g != 100 {
		t.Fatalf("caught up got %d", g)
	}
	if g := activeChainBlockHeight(100, 40, false, -1); g != 100 {
		t.Fatalf("no raw store got %d want header tip", g)
	}
	if g := activeChainBlockHeight(100, 500, true, 2); g != 2 {
		t.Fatalf("connected tip got %d want 2 (Core chainActive, not orphan stored bodies)", g)
	}
}

func TestActiveChainFromJournalUsesConnectedTip(t *testing.T) {
	j := &memJournal{tip: 99_999, best: "b", gen: "g", count: 100_000, hdrs: [][]byte{make([]byte, 80)}}
	utxo := store.NewUtxoCache()
	_ = utxo.ApplyBlock(&wire.ParsedBlock{Txs: []*wire.Tx{{Vin: []wire.TxIn{{PrevIdx: 0xffffffff}}, Vout: []wire.TxOut{{Value: 1}}}}}, 1)
	paths := &DataPaths{
		ContiguousRawHeight: func() int64 { return 500 },
		Utxo:                utxo,
	}
	blocks, headers, cont := activeChainFromJournal(j, &store.RawBlockStore{}, paths)
	if headers != 99_999 || cont != 500 || blocks != 1 {
		t.Fatalf("blocks=%d headers=%d cont=%d", blocks, headers, cont)
	}
}

func TestTipMedianTimePastTwoHeaders(t *testing.T) {
	h0 := make([]byte, 80)
	binary.LittleEndian.PutUint32(h0[68:72], 100)
	h1 := make([]byte, 80)
	binary.LittleEndian.PutUint32(h1[68:72], 200)
	j := &memJournal{tip: 1, best: "x", gen: "y", count: 2, hdrs: [][]byte{h0, h1}}
	mt, err := tipMedianTimePast(j)
	if err != nil {
		t.Fatal(err)
	}
	// tip-1=0: only one timestamp in window → median 100
	if mt != 100 {
		t.Fatalf("mediantime got %d want 100", mt)
	}
	hmt, err := headerMedianTimePast(j, 1)
	if err != nil {
		t.Fatal(err)
	}
	if hmt != mt {
		t.Fatalf("header at tip MTP %d != tip MTP %d", hmt, mt)
	}
}

func TestHeaderMedianTimePastHeight1(t *testing.T) {
	h0 := make([]byte, 80)
	binary.LittleEndian.PutUint32(h0[68:72], 42)
	h1 := make([]byte, 80)
	binary.LittleEndian.PutUint32(h1[68:72], 99)
	j := &memJournal{tip: 1, best: "x", gen: "y", count: 2, hdrs: [][]byte{h0, h1}}
	mt, err := headerMedianTimePast(j, 1)
	if err != nil {
		t.Fatal(err)
	}
	if mt != 42 {
		t.Fatalf("got %d want 42 (only height 0 in window)", mt)
	}
}

func TestHandlerGetBlockchainInfoDifficultyMediantime(t *testing.T) {
	h0 := make([]byte, 80)
	binary.LittleEndian.PutUint32(h0[68:72], 50)
	binary.LittleEndian.PutUint32(h0[72:76], 0x1e0ffff0)
	h1 := make([]byte, 80)
	binary.LittleEndian.PutUint32(h1[68:72], 150)
	binary.LittleEndian.PutUint32(h1[72:76], 0x1e0ffff0)
	h2 := make([]byte, 80)
	binary.LittleEndian.PutUint32(h2[68:72], 999)
	binary.LittleEndian.PutUint32(h2[72:76], 0x1e0ffff0)
	// tip=2 → MTP window uses heights 1..0 → timestamps 150,50 → sorted median index len/2 = 150
	j := &memJournal{tip: 2, best: "b", gen: "g", count: 3, hdrs: [][]byte{h0, h1, h2}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getblockchaininfo"}`)))
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
	if out.Result["mediantime"].(float64) != 150 {
		t.Fatalf("mediantime %#v want 150 (median of 50,150 at tip-1 window)", out.Result["mediantime"])
	}
	if out.Result["difficulty"].(float64) <= 0 {
		t.Fatalf("difficulty %#v", out.Result["difficulty"])
	}
	sf, ok := out.Result["softforks"].([]interface{})
	if !ok || sf == nil {
		t.Fatalf("softforks %#v", out.Result["softforks"])
	}
	if len(sf) != 3 {
		t.Fatalf("softforks len %d want 3 (bip34/bip66/bip65)", len(sf))
	}
	bip9, ok := out.Result["bip9_softforks"].(map[string]interface{})
	if !ok || bip9["csv"] == nil {
		t.Fatalf("bip9_softforks %#v", out.Result["bip9_softforks"])
	}
	if out.Result["automatic_pruning"].(bool) != false {
		t.Fatalf("automatic_pruning %#v", out.Result["automatic_pruning"])
	}
	if out.Result["prune_height"] != nil {
		t.Fatalf("prune_height %#v want null", out.Result["prune_height"])
	}
	if out.Result["prune_target_size"].(float64) != 0 {
		t.Fatalf("prune_target_size %#v", out.Result["prune_target_size"])
	}
}

func TestHandlerGetBlockchainInfoPrunedMarker(t *testing.T) {
	dir := t.TempDir()
	if err := store.SavePruneMarker(dir, 42); err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{ChainDataDir: dir}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"method":"getblockchaininfo"}`)))
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
	if out.Result["pruned"].(bool) != true {
		t.Fatalf("pruned %#v", out.Result["pruned"])
	}
	if int(out.Result["prune_height"].(float64)) != 42 {
		t.Fatalf("prune_height %#v want 42", out.Result["prune_height"])
	}
}

func TestMinimumChainWorkFieldsUsesChainWorkCache(t *testing.T) {
	min, ok := chain.MinimumChainWorkForRPCChain("main")
	if !ok || min == nil {
		t.Skip("no minimum chain work for main")
	}
	cached := new(big.Int).Add(min, big.NewInt(1))
	paths := &DataPaths{
		CumulativeChainWork: func(through int64) (*big.Int, bool) {
			if through == 534_000 {
				return cached, true
			}
			return nil, false
		},
	}
	out := minimumChainWorkFields(nil, "main", 534_000, paths)
	if out == nil {
		t.Fatal("nil out")
	}
	if met, ok := out["dogego_minimum_chain_work_met"].(bool); !ok || !met {
		t.Fatalf("minimum_chain_work_met=%v", out["dogego_minimum_chain_work_met"])
	}
	if got, ok := out["dogego_header_chainwork"].(string); !ok || got != pow.ChainworkHex(cached) {
		t.Fatalf("header_chainwork=%v", out["dogego_header_chainwork"])
	}
}

func TestChainWorkThroughSkipsCacheWhileWarming(t *testing.T) {
	called := false
	paths := &DataPaths{
		ChainWorkCacheReady: func() bool { return false },
		CumulativeChainWork: func(through int64) (*big.Int, bool) {
			called = true
			return nil, false
		},
	}
	if _, ok := chainWorkThrough(nil, 534_000, paths); ok {
		t.Fatal("expected miss while warming")
	}
	if called {
		t.Fatal("CumulativeChainWork should not run for large heights while cache is warming")
	}
}

func TestDogegoCmpctRelayCounterKeys(t *testing.T) {
	keys := DogegoCmpctRelayCounterKeys()
	want := []string{
		"dogego_cmpct_in",
		"dogego_cmpct_mempool_hit",
		"dogego_cmpct_getblocktxn_out",
		"dogego_cmpct_blocktxn_in",
		"dogego_cmpct_reconstruct_ok",
		"dogego_cmpct_reconstruct_fail",
		"dogego_cmpct_announced_out",
		"dogego_cmpct_served_getdata",
		"dogego_cmpct_fallback_full_block",
		"dogego_cmpct_blocktxn_served",
		"dogego_cmpct_reconstruct_fallback_getdata",
	}
	if len(keys) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(keys), len(want), keys)
	}
	for i, k := range want {
		if keys[i] != k {
			t.Fatalf("keys[%d]=%q want %q", i, keys[i], k)
		}
	}
}

func TestMergeDogegoCmpctRelayFromP2P(t *testing.T) {
	res := map[string]interface{}{}
	snap := map[string]interface{}{
		"bip152_hb_to":                  2,
		"dogego_cmpct_in":               uint64(5),
		"dogego_cmpct_reconstruct_ok":   uint64(3),
		"dogego_cmpct_announced_out":    uint64(1),
	}
	mergeDogegoCmpctRelayFromP2P(res, snap)
	if res["bip152_hb_to"] != 2 {
		t.Fatalf("hb_to=%v", res["bip152_hb_to"])
	}
	if res["dogego_cmpct_reconstruct_ok"] != uint64(3) {
		t.Fatalf("reconstruct_ok=%v", res["dogego_cmpct_reconstruct_ok"])
	}
	if res["dogego_cmpct_mempool_hit"] != uint64(0) {
		t.Fatalf("missing keys should default to 0: mempool_hit=%v", res["dogego_cmpct_mempool_hit"])
	}
	if res["dogego_cmpct_reconstruct_fallback_getdata"] != uint64(0) {
		t.Fatalf("reconstruct_fallback_getdata default=%v", res["dogego_cmpct_reconstruct_fallback_getdata"])
	}
}
