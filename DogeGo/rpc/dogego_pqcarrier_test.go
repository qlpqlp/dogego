// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"dogego/consensus"
	"dogego/pqcrypto"
	"dogego/wire"
)

func TestDogegoPQCarrierGoldenErrors(t *testing.T) {
	t.Run("createpqcarrier_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execDogegoCreatePQCarrier(nil, nil)
		if code != -8 || !strings.Contains(msg, "expected 1 object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createpqcarrier_flag_off", func(t *testing.T) {
		_, code, msg := execDogegoCreatePQCarrier(&DataPaths{}, []json.RawMessage{json.RawMessage(`{"tx_base_hex":"00"}`)})
		if code != -8 || !strings.Contains(msg, "pq_carrier") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifypqcarrier_missing_txc", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCarrier([]json.RawMessage{json.RawMessage(`{"txr_hex":"00","pk_script_hex":"76a914"}`)})
		if code != -8 || !strings.Contains(msg, "txc_hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifypqcarrier_bad_object", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCarrier([]json.RawMessage{json.RawMessage(`"not-object"`)})
		if code != -8 || !strings.Contains(msg, "bad argument object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
}

func TestExecDogegoVerifyPQCarrierFalconRoundTrip(t *testing.T) {
	pkScript := []byte{
		0x76, 0xa9, 0x14,
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		0x88, 0xac,
	}
	scheme := pqcrypto.Falcon512{}
	seed := pqcrypto.DeriveSeed([]byte("rpc-carrier"), "falcon")
	pk, sk, err := scheme.GenerateKey(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	txBase := &wire.Tx{
		Version: 2,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000_000, PkScript: pkScript}},
	}
	plan, err := consensus.BuildPQCarrierTransactions(txBase, scheme, pk, sk, 0, pkScript, wire.SigHashAll, consensus.PQCarrierMinOutputKoinu())
	if err != nil {
		t.Fatal(err)
	}
	txcHex, err := serializeTxHex(plan.TXC)
	if err != nil {
		t.Fatal(err)
	}
	txrHex, err := serializeTxHex(plan.TXR)
	if err != nil {
		t.Fatal(err)
	}
	req, err := json.Marshal(map[string]string{
		"txc_hex":       txcHex,
		"txr_hex":       txrHex,
		"pk_script_hex": hex.EncodeToString(pkScript),
	})
	if err != nil {
		t.Fatal(err)
	}
	res, code, msg := execDogegoVerifyPQCarrier([]json.RawMessage{req})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("type %T", res)
	}
	if m["valid"] != true || m["pq_verify"] != "passed" || m["mode"] != "carrier_scriptsig" {
		t.Fatalf("res=%v", m)
	}
}

func TestExecDogegoVerifyPQCarrierDilithiumRoundTrip(t *testing.T) {
	pkScript := []byte{
		0x76, 0xa9, 0x14,
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		0x88, 0xac,
	}
	scheme := pqcrypto.Dilithium2{}
	seed := pqcrypto.DeriveSeed([]byte("rpc-carrier-dil2"), "dilithium")
	pk, sk, err := scheme.GenerateKey(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	txBase := &wire.Tx{
		Version: 2,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 10_000_000_000, PkScript: pkScript}},
	}
	plan, err := consensus.BuildPQCarrierTransactions(txBase, scheme, pk, sk, 0, pkScript, wire.SigHashAll, consensus.PQCarrierMinOutputKoinu())
	if err != nil {
		t.Fatal(err)
	}
	txcHex, err := serializeTxHex(plan.TXC)
	if err != nil {
		t.Fatal(err)
	}
	txrHex, err := serializeTxHex(plan.TXR)
	if err != nil {
		t.Fatal(err)
	}
	req, err := json.Marshal(map[string]string{
		"txc_hex":       txcHex,
		"txr_hex":       txrHex,
		"pk_script_hex": hex.EncodeToString(pkScript),
		"tag":           "DIL2",
	})
	if err != nil {
		t.Fatal(err)
	}
	res, code, msg := execDogegoVerifyPQCarrier([]json.RawMessage{req})
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("type %T", res)
	}
	if m["valid"] != true || m["pq_verify"] != "passed" || m["mode"] != "carrier_scriptsig" {
		t.Fatalf("res=%v", m)
	}
}
