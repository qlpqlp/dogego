// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/json"
	"testing"

	"dogego/pqcrypto"
	"dogego/wire"
)

func testPQCarrierSpendPkScript() []byte {
	return []byte{
		0x76, 0xa9, 0x14,
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
		0x88, 0xac,
	}
}

func testPQCarrierFundedTxBase(pkScript []byte) *wire.Tx {
	return &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{},
			PrevIdx:  0,
			Sequence: 0xffffffff,
		}},
		Vout: []wire.TxOut{{Value: 10_000_000_000, PkScript: pkScript}},
	}
}

func TestVerifyPQCarrierPairFalconRoundTrip(t *testing.T) {
	runPQCarrierVerifyRoundTrip(t, pqcrypto.Falcon512{}, "falcon-carrier")
}

func TestVerifyPQCarrierPairDilithiumRoundTrip(t *testing.T) {
	runPQCarrierVerifyRoundTrip(t, pqcrypto.Dilithium2{}, "dilithium-carrier")
}

func TestVerifyPQCarrierPairRaccoonRoundTrip(t *testing.T) {
	s := pqcrypto.RaccoonG44{}
	if !s.Available() {
		t.Skip("Raccoon-G-44 requires CGO_ENABLED=1 -tags raccoon_g (libgmp+libmpfr)")
	}
	runPQCarrierVerifyRoundTrip(t, s, "raccoon-carrier")
}

func runPQCarrierVerifyRoundTrip(t *testing.T, scheme pqcrypto.Scheme, label string) {
	t.Helper()
	pkScript := testPQCarrierSpendPkScript()
	seed := pqcrypto.DeriveSeed([]byte(label), scheme.Name())
	pk, sk, err := scheme.GenerateKey(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPQCarrierTransactions(
		testPQCarrierFundedTxBase(pkScript),
		scheme, pk, sk, 0, pkScript, wire.SigHashAll, PQCarrierMinOutputKoinu(),
	)
	if err != nil {
		t.Fatal(err)
	}
	out, err := VerifyPQCarrierPair(plan.TXC, plan.TXR, 0, pkScript, wire.SigHashAll, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out["valid"] != true {
		t.Fatalf("valid=%v out=%v", out["valid"], out)
	}
	if out["pq_verify"] != "passed" {
		t.Fatalf("pq_verify=%v", out["pq_verify"])
	}
	if out["linkage_ok"] != true || out["commitment_match"] != true {
		t.Fatalf("linkage/commitment out=%v", out)
	}
	if out["dogego_pqc_matched_txc_txid"] == "" || out["dogego_pqc_txr_txid"] == "" {
		t.Fatalf("missing txids out=%v", out)
	}
}

func TestVerifyPQCarrierPairRejectsLinkage(t *testing.T) {
	scheme := pqcrypto.Falcon512{}
	pkScript := testPQCarrierSpendPkScript()
	seed := pqcrypto.DeriveSeed([]byte("linkage"), "falcon")
	pk, sk, err := scheme.GenerateKey(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPQCarrierTransactions(
		testPQCarrierFundedTxBase(pkScript),
		scheme, pk, sk, 0, pkScript, wire.SigHashAll, PQCarrierMinOutputKoinu(),
	)
	if err != nil {
		t.Fatal(err)
	}
	bad := cloneTx(plan.TXR)
	bad.Vin[0].PrevHash = [32]byte{0xff}
	_, err = VerifyPQCarrierPair(plan.TXC, bad, 0, pkScript, wire.SigHashAll, scheme)
	if err == nil {
		t.Fatal("expected linkage error")
	}
}

func TestVerifyPQCarrierPairRejectsWrongSighash(t *testing.T) {
	scheme := pqcrypto.Falcon512{}
	pkScript := testPQCarrierSpendPkScript()
	wrongPkScript := append([]byte(nil), pkScript...)
	wrongPkScript[5] ^= 0xff
	seed := pqcrypto.DeriveSeed([]byte("sighash"), "falcon")
	pk, sk, err := scheme.GenerateKey(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPQCarrierTransactions(
		testPQCarrierFundedTxBase(pkScript),
		scheme, pk, sk, 0, pkScript, wire.SigHashAll, PQCarrierMinOutputKoinu(),
	)
	if err != nil {
		t.Fatal(err)
	}
	out, err := VerifyPQCarrierPair(plan.TXC, plan.TXR, 0, wrongPkScript, wire.SigHashAll, scheme)
	if err != nil {
		t.Fatal(err)
	}
	if out["valid"] != false || out["pq_verify"] != "failed" {
		t.Fatalf("out=%v", out)
	}
}

func TestVerifyPQCarrierPairRPCFieldsJSON(t *testing.T) {
	scheme := pqcrypto.Falcon512{}
	pkScript := testPQCarrierSpendPkScript()
	seed := pqcrypto.DeriveSeed([]byte("json"), "falcon")
	pk, sk, err := scheme.GenerateKey(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPQCarrierTransactions(
		testPQCarrierFundedTxBase(pkScript),
		scheme, pk, sk, 0, pkScript, wire.SigHashAll, PQCarrierMinOutputKoinu(),
	)
	if err != nil {
		t.Fatal(err)
	}
	out, err := VerifyPQCarrierPair(plan.TXC, plan.TXR, 0, pkScript, wire.SigHashAll, scheme)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"valid", "pq_verify", "commitment_match", "linkage_ok", "sighash32", "commitment"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("missing %q in %v", key, m)
		}
	}
}
