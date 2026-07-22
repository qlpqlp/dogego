// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
)

func TestExecGetBlockTemplateFields(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	res, code, msg := execGetBlockTemplate(j, nil, nil, nil, nil, "main", 0, []json.RawMessage{json.RawMessage(`{}`)})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	m := res.(map[string]interface{})
	if m["height"].(int64) != 1 {
		t.Fatalf("height %#v", m["height"])
	}
	if m["weightlimit"].(int) != consensus.MaxBlockWeight {
		t.Fatalf("weightlimit %#v", m["weightlimit"])
	}
	if _, ok := m["previousblockhash"].(string); !ok {
		t.Fatal("missing previousblockhash")
	}
	if aux, ok := m["coinbaseaux"].(map[string]interface{}); !ok || aux["flags"] != "" {
		t.Fatalf("coinbaseaux %#v", m["coinbaseaux"])
	}
	switch m["rules"].(type) {
	case []string, []interface{}, nil:
	default:
		t.Fatalf("rules %#v", m["rules"])
	}
	if m["vbrequired"].(int) != 0 {
		t.Fatalf("vbrequired %#v", m["vbrequired"])
	}
}

func TestExecGetBlockTemplateCustomWeight(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	res, code, msg := execGetBlockTemplate(j, nil, nil, nil, nil, "main", 2_500_000, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if res.(map[string]interface{})["weightlimit"].(int) != 2_500_000 {
		t.Fatal("weightlimit")
	}
}

func TestExecGetBlockTemplateRejectsSegwitRules(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	_, code, _ := execGetBlockTemplate(j, nil, nil, nil, nil, "main", 0, []json.RawMessage{json.RawMessage(`{"rules":["segwit"]}`)})
	if code != -8 {
		t.Fatalf("code %d", code)
	}
}

func TestExecGetBlockTemplateProposalMode(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	res, code, msg := execGetBlockTemplate(j, nil, nil, nil, nil, "main", 0, []json.RawMessage{json.RawMessage(`{"mode":"proposal","data":"00"}`)})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if s, ok := res.(string); !ok || s == "" {
		t.Fatalf("result %#v", res)
	}
}

func TestExecGetBlockTemplateBitsMatchNextBlockBits(t *testing.T) {
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 0, best: "x", gen: "y", count: 1, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	res, code, msg := execGetBlockTemplate(j, nil, nil, nil, nil, "main", 0, []json.RawMessage{json.RawMessage(`{}`)})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	m := res.(map[string]interface{})
	bitsHex, _ := m["bits"].(string)
	wantBits, err := consensus.NextBlockBits(j, chain.MainnetDogecoin, 1, uint32(m["curtime"].(int64)))
	if err != nil {
		t.Fatal(err)
	}
	if bitsHex != pow.BitsHex(wantBits) {
		t.Fatalf("bits=%q want %q", bitsHex, pow.BitsHex(wantBits))
	}
	if m["target"].(string) != pow.TargetHexFromCompact(wantBits) {
		t.Fatalf("target mismatch")
	}
	caps, ok := m["capabilities"].([]string)
	if !ok {
		if raw, ok2 := m["capabilities"].([]interface{}); ok2 {
			caps = make([]string, len(raw))
			for i, v := range raw {
				caps[i], _ = v.(string)
			}
		}
	}
	joined := strings.Join(caps, ",")
	if !strings.Contains(joined, "proposal") || !strings.Contains(joined, "longpoll") {
		t.Fatalf("capabilities %#v", m["capabilities"])
	}
	if _, ok := m["longpollid"].(string); !ok || m["longpollid"] == "" {
		t.Fatal("missing longpollid")
	}
}

func TestWaitGBTLongpollWake(t *testing.T) {
	same := true
	go func() {
		time.Sleep(40 * time.Millisecond)
		same = false
		NotifyGBTWake()
	}()
	start := time.Now()
	waitGBTLongpoll(3*time.Second, func() bool { return same })
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("longpoll did not wake promptly: %v", elapsed)
	}
}
