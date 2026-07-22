// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"strings"
	"testing"

	"dogego/secp256k1"

	"dogego/mempool"
	"dogego/wire"
)

func loadCoreMempoolVectors(t *testing.T) []coreMempoolVector {
	t.Helper()
	var vecs []coreMempoolVector
	loadJSONFixture(t, "core_mempool_vectors.json", &vecs)
	if len(vecs) == 0 {
		t.Fatal("no mempool differential vectors loaded")
	}
	return vecs
}

// TestCoreMempoolDifferentialVectors checks mempool admission and Core-shaped reject reasons.
func TestCoreMempoolDifferentialVectors(t *testing.T) {
	for _, v := range loadCoreMempoolVectors(t) {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			switch v.Template {
			case "min_relay_fee", "rbf_insufficient_fee", "rbf_sufficient_fee", "rbf_not_replaceable", "rbf_fullrbf", "coinbase_immature", "rbf_too_many_descendants", "rbf_too_many_conflicts", "rbf_new_unconfirmed_input", "non_bip68_final":
				err := EvaluateMempoolDifferentialCheck(v.Template)
				assertMempoolVectorReject(t, err, v)
				return
			case "package_ancestor_limit":
				runPackageAncestorLimitVector(t, v)
				return
			case "mempool_double_spend":
				runMempoolDoubleSpendVector(t, v)
				return
			case "package_descendant_limit":
				runPackageDescendantLimitVector(t, v)
				return
			case "package_ancestor_size":
				runPackageAncestorSizeVector(t, v)
				return
			case "package_descendant_size":
				runPackageDescendantSizeVector(t, v)
				return
			}
			tx, adm, err := buildMempoolAdmissionCase(v.Template)
			if err != nil {
				t.Fatal(err)
			}
			err = AcceptMempoolTxAdmission(tx, adm)
			if v.WantAccept {
				if err != nil {
					t.Fatalf("expected accept, got: %v (reason=%q)", err, MempoolRejectReason(err))
				}
				return
			}
			if err == nil {
				t.Fatal("expected reject, got nil")
			}
			got := MempoolRejectReason(err)
			if got != v.WantRejectReason {
				t.Fatalf("reject reason %q want %q (err=%v)", got, v.WantRejectReason, err)
			}
		})
	}
}

func runPackageAncestorLimitVector(t *testing.T, v coreMempoolVector) {
	t.Helper()
	pool := mempool.New(100)
	var prev [32]byte
	prev[0] = 0xaa
	parentHash := prev
	for i := 0; i < 26; i++ {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: p2pkhScript()}},
		}
		if err := pool.Add(parent.SerializeForHash()); err != nil {
			t.Fatal(err)
		}
		parentHash = parent.TxHash()
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: fixtureChildOutValue(), PkScript: p2pkhScript()}},
	}
	sizes, err := pool.BuildMempoolSizes()
	if err != nil {
		t.Fatal(err)
	}
	err = CheckMempoolPackageLimits(child, pool, sizes, 25, 25, 101)
	assertMempoolVectorReject(t, err, v)
}

func runMempoolDoubleSpendVector(t *testing.T, v coreMempoolVector) {
	t.Helper()
	sec := make([]byte, 32)
	sec[0] = 0x66
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)

	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
	fundRaw, _ := funding.Serialize()
	pool := mempool.New(10)
	_ = pool.Add(fundRaw)

	spend1 := buildSignedSpend(t, funding, pkScript, priv, pubC, 900_000_000)
	_ = pool.Add(spend1.SerializeForHash())

	spend2 := buildSignedSpend(t, funding, pkScript, priv, pubC, 800_000_000)
	adm := MempoolAdmission{
		View:             AdmissionPrevOutView(pool, nil, nil),
		Pool:             pool,
		MinRelayFeePerKB: DefaultMinRelayTxFeePerKB,
	}
	err := adm.CheckSpendConflicts(spend2)
	assertMempoolVectorReject(t, err, v)
}

func runPackageDescendantLimitVector(t *testing.T, v coreMempoolVector) {
	t.Helper()
	pool := mempool.New(100)
	var prevHash [32]byte
	prevHash[0] = 0xaa
	for i := 0; i < 25; i++ {
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: p2pkhScript()}},
		}
		if err := pool.Add(tx.SerializeForHash()); err != nil {
			t.Fatal(err)
		}
		prevHash = tx.TxHash()
	}
	sizes, err := pool.BuildMempoolSizes()
	if err != nil {
		t.Fatal(err)
	}
	extra := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: fixtureChildOutValue(), PkScript: p2pkhScript()}},
	}
	err = CheckMempoolPackageLimits(extra, pool, sizes, 25, 25, 101)
	assertMempoolVectorReject(t, err, v)
}

func runPackageAncestorSizeVector(t *testing.T, v coreMempoolVector) {
	t.Helper()
	pool := mempool.New(100)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff, Script: make([]byte, 900)}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: p2pkhScript()}},
	}
	if err := pool.Add(parent.SerializeForHash()); err != nil {
		t.Fatal(err)
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 49_000_000, PkScript: p2pkhScript()}},
	}
	ph := parent.TxHash()
	var k [36]byte
	copy(k[:32], ph[:])
	view := mempoolStubPrevOutView{}
	view[k] = PrevOut{Value: parent.Vout[0].Value, PkScript: parent.Vout[0].PkScript}
	err := CheckMempoolPackageSizeLimits(child, pool, view, 1, 101)
	assertMempoolVectorReject(t, err, v)
}

func runPackageDescendantSizeVector(t *testing.T, v coreMempoolVector) {
	t.Helper()
	pool := mempool.New(100)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: p2pkhScript()}},
	}
	if err := pool.Add(parent.SerializeForHash()); err != nil {
		t.Fatal(err)
	}
	child1 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff, Script: make([]byte, 900)}},
		Vout:    []wire.TxOut{{Value: 49_000_000, PkScript: p2pkhScript()}},
	}
	if err := pool.Add(child1.SerializeForHash()); err != nil {
		t.Fatal(err)
	}
	child2 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: child1.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 48_000_000, PkScript: p2pkhScript()}},
	}
	ph := parent.TxHash()
	c1h := child1.TxHash()
	view := mempoolStubPrevOutView{}
	view[outpointKey(ph, 0)] = PrevOut{Value: parent.Vout[0].Value, PkScript: parent.Vout[0].PkScript}
	view[outpointKey(c1h, 0)] = PrevOut{Value: child1.Vout[0].Value, PkScript: child1.Vout[0].PkScript}
	err := CheckMempoolPackageSizeLimits(child2, pool, view, 101, 1)
	assertMempoolVectorReject(t, err, v)
}

func assertMempoolVectorReject(t *testing.T, err error, v coreMempoolVector) {
	t.Helper()
	if v.WantAccept {
		if err != nil {
			t.Fatalf("expected accept, got: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatal("expected reject, got nil")
	}
	got := MempoolRejectReason(err)
	if got != v.WantRejectReason && !strings.HasPrefix(got, v.WantRejectReason) {
		t.Fatalf("reject reason %q want %q (err=%v)", got, v.WantRejectReason, err)
	}
}
