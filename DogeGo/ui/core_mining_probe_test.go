// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"dogego/config"
)

func miningProbeInvoke(blocks int64, auxEra bool) func(string, []json.RawMessage) map[string]interface{} {
	methods := map[string]interface{}{}
	for _, m := range coreMiningRequiredRPC {
		methods[m] = map[string]interface{}{}
	}
	gbt := map[string]interface{}{
		"capabilities":      []interface{}{"proposal", "longpoll"},
		"version":           float64(6422787),
		"rules":             []interface{}{},
		"vbrequired":        float64(0),
		"coinbaseaux":       map[string]interface{}{"flags": ""},
		"previousblockhash": "aa",
		"bits":              "1e0ffff0",
		"target":            "00000ffff0000000000000000000000000000000000000000000000000000000",
		"height":            float64(blocks + 1),
		"curtime":           float64(1),
		"mintime":           float64(1),
		"sigoplimit":        float64(20000),
		"sizelimit":         float64(1000000),
		"weightlimit":       float64(4000000),
		"coinbasevalue":     float64(10000000000),
		"transactions":      []interface{}{},
		"mutable":           []interface{}{"time", "transactions", "prevblock"},
		"noncerange":        "00000000ffffffff",
		"longpollid":        "lp1",
	}
	return func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getrpcinfo":
			return map[string]interface{}{"result": map[string]interface{}{"method": methods}}
		case "getblockchaininfo":
			out := map[string]interface{}{
				"blocks":  float64(blocks),
				"headers": float64(blocks),
				"initialblockdownload": false,
				"dogego_auxpow_parent_chain_id_core_parity": true,
			}
			return map[string]interface{}{"result": out}
		case "getblocktemplate":
			return map[string]interface{}{"result": gbt}
		case "getmininginfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"blocks": float64(blocks), "difficulty": 1.0, "networkhashps": 1.0, "pooledtx": float64(0), "chain": "main",
			}}
		case "createauxblock":
			if !auxEra {
				return map[string]interface{}{"error": map[string]interface{}{"code": -1.0, "message": "pre-aux"}}
			}
			return map[string]interface{}{"result": map[string]interface{}{
				"hash": "bb", "chainid": float64(0x62), "target": "00", "coinbasevalue": float64(1),
			}}
		default:
			return map[string]interface{}{"error": map[string]interface{}{"code": -32601.0, "message": "unknown " + method}}
		}
	}
}

func TestProbeCoreMiningAuxEraOK(t *testing.T) {
	// Reboot testnet aux activation is 158100; tip 158100 → next is aux era.
	out := ProbeCoreMining("testnet", config.File{}, miningProbeInvoke(158100, true))
	if !out.OK {
		t.Fatalf("ok=false issues=%v checks=%+v", out.Issues, out.Checks)
	}
	if !out.AuxEra || !out.CreateAuxOK || out.CreateAuxSkipped {
		t.Fatalf("aux: era=%v create=%v skipped=%v", out.AuxEra, out.CreateAuxOK, out.CreateAuxSkipped)
	}
	if !out.GBTFieldsOK || !out.MiningInfoOK {
		t.Fatalf("gbt=%v mininginfo=%v", out.GBTFieldsOK, out.MiningInfoOK)
	}
}

func TestProbeCoreMiningPreAuxSkipsCreateAux(t *testing.T) {
	out := ProbeCoreMining("mainnet", config.File{}, miningProbeInvoke(1000, false))
	if !out.OK {
		t.Fatalf("ok=false issues=%v", out.Issues)
	}
	if out.AuxEra || !out.CreateAuxSkipped || out.CreateAuxOK {
		t.Fatalf("pre-aux: era=%v skipped=%v create=%v", out.AuxEra, out.CreateAuxSkipped, out.CreateAuxOK)
	}
}

func TestProbeCoreMiningMissingGBTField(t *testing.T) {
	base := miningProbeInvoke(158100, true)
	invoke := func(method string, params []json.RawMessage) map[string]interface{} {
		r := base(method, params)
		if method == "getblocktemplate" {
			m := r["result"].(map[string]interface{})
			delete(m, "longpollid")
		}
		return r
	}
	out := ProbeCoreMining("testnet", config.File{}, invoke)
	if out.OK {
		t.Fatal("expected fail")
	}
	found := false
	for _, iss := range out.Issues {
		if strings.Contains(iss, "longpollid") {
			found = true
		}
	}
	if !found {
		t.Fatalf("issues=%v", out.Issues)
	}
}

func TestProbeCoreMiningRPCNotReady(t *testing.T) {
	out := ProbeCoreMining("mainnet", config.File{}, nil)
	if out.OK || len(out.Issues) == 0 {
		t.Fatalf("want issue: %+v", out)
	}
}

func TestApplyCoreOperatorCertMining(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{
		Mining: CoreMiningProbeResult{OK: true, GBTFieldsOK: true, MiningInfoOK: true, CreateAuxSkipped: true},
	})
	var row *CoreOperatorCertRow
	for i := range rows {
		if rows[i].ID == "mining" {
			row = &rows[i]
			break
		}
	}
	if row == nil || row.OK == nil || !*row.OK {
		t.Fatalf("mining row: %+v", row)
	}
}
