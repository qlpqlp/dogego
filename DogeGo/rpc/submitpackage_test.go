// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

func TestExecSubmitPackageParentChild(t *testing.T) {
	pool := mempool.New(100)
	priv, pubC, pkScript := testP2PKHKeyMaterial()
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
	if err := utxo.ApplyBlock(&wire.ParsedBlock{Txs: []*wire.Tx{coin}}, 0); err != nil {
		t.Fatal(err)
	}
	parent := buildSignedSpendSubmit(coin, pkScript, priv, pubC, 900_000_000)
	child := buildSignedSpendSubmit(parent, pkScript, priv, pubC, 800_000_000)
	praw, err := parent.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	craw, err := child.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{Utxo: utxo}
	arr, _ := json.Marshal([]string{hex.EncodeToString(praw), hex.EncodeToString(craw)})
	res, code, msg := execSubmitPackage(pool, nil, nil, nil, paths, []json.RawMessage{arr, json.RawMessage(`0`)}, nil, false, chain.RebootTestnet)
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	m := res.(map[string]interface{})
	if m["package_msg"] != "success" {
		t.Fatalf("package_msg %#v results %#v", m["package_msg"], m["tx-results"])
	}
	if pool.Count() != 2 {
		t.Fatalf("pool count %d", pool.Count())
	}
	results := m["tx-results"].(map[string]interface{})
	childW := txidToRPC(child.WTxHash())
	row := results[childW].(map[string]interface{})
	fees := row["fees"].(map[string]interface{})
	inc, ok := fees["effective-includes"].([]string)
	if !ok || len(inc) < 2 {
		t.Fatalf("child effective-includes %#v", fees["effective-includes"])
	}
}

// TestExecSubmitPackageCPFPBelowMinRelayParent admits a below-min-relay parent when the child pays the package rate.
func TestExecSubmitPackageCPFPBelowMinRelayParent(t *testing.T) {
	pool := mempool.New(100)
	priv, pubC, pkScript := testP2PKHKeyMaterial()
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{4}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
	if err := utxo.ApplyBlock(&wire.ParsedBlock{Txs: []*wire.Tx{coin}}, 0); err != nil {
		t.Fatal(err)
	}
	// Parent fee 10_000 koinu: fails alone at default min relay (~20k+ for ~200B tx).
	parent := buildSignedSpendSubmit(coin, pkScript, priv, pubC, 999_990_000)
	child := buildSignedSpendSubmit(parent, pkScript, priv, pubC, 999_000_000)
	praw, err := parent.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	craw, err := child.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{Utxo: utxo}

	adm := newMempoolAdmission(pool, nil, nil, nil, paths, chain.RebootTestnet)
	if err := acceptMempoolTxRPC(praw, parent, pool, paths, adm); err == nil {
		t.Fatal("parent alone should fail min relay")
	}

	arr, _ := json.Marshal([]string{hex.EncodeToString(praw), hex.EncodeToString(craw)})
	res, code, msg := execSubmitPackage(pool, nil, nil, nil, paths, []json.RawMessage{arr, json.RawMessage(`0`)}, nil, false, chain.RebootTestnet)
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	m := res.(map[string]interface{})
	if m["package_msg"] != "success" {
		t.Fatalf("package_msg %#v results %#v", m["package_msg"], m["tx-results"])
	}
	if pool.Count() != 2 {
		t.Fatalf("pool count %d want 2", pool.Count())
	}
}

func TestValidateSubmitPackageDependentParents(t *testing.T) {
	p1 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	p2 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: p1.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: p2.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	pkg := []parsedPackageTx{{tx: p1}, {tx: p2}, {tx: child}}
	if err := validateSubmitPackageStructure(pkg); err == nil {
		t.Fatal("expected dependent parents error")
	}
}

func testP2PKHKeyMaterial() (*secp256k1.PrivateKey, []byte, []byte) {
	sec := make([]byte, 32)
	sec[0] = 0x55
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := pubkeyHash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)
	return priv, pubC, pkScript
}

func buildSignedSpendSubmit(funding *wire.Tx, pkScript []byte, priv *secp256k1.PrivateKey, pubC []byte, outVal int64) *wire.Tx {
	return buildSignedSpendSubmitSeq(funding, pkScript, priv, pubC, outVal, 0xffffffff)
}

func buildSignedSpendSubmitSeq(funding *wire.Tx, pkScript []byte, priv *secp256k1.PrivateKey, pubC []byte, outVal int64, seq uint32) *wire.Tx {
	spend := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: funding.TxHash(),
			PrevIdx:  0,
			Sequence: seq,
		}},
		Vout: []wire.TxOut{{Value: outVal, PkScript: append([]byte(nil), pkScript...)}},
	}
	digest, err := wire.CalcSignatureHashLegacy(pkScript, wire.SigHashAll, spend, 0)
	if err != nil {
		panic(err)
	}
	sigDER := ecdsa.Sign(priv, digest[:])
	sigWithType := append(sigDER.Serialize(), byte(wire.SigHashAll))
	script, err := concatPushes(sigWithType, pubC)
	if err != nil {
		panic(err)
	}
	spend.Vin[0].Script = script
	return spend
}

// TestExecSubmitPackageReplacedTransactions reports BIP125 conflicts removed during package admit.
func TestExecSubmitPackageReplacedTransactions(t *testing.T) {
	pool := mempool.New(100)
	priv, pubC, pkScript := testP2PKHKeyMaterial()
	utxo := store.NewUtxoCache()
	coin := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{5}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
	if err := utxo.ApplyBlock(&wire.ParsedBlock{Txs: []*wire.Tx{coin}}, 0); err != nil {
		t.Fatal(err)
	}
	conflict := buildSignedSpendSubmitSeq(coin, pkScript, priv, pubC, 900_000_000, wire.MaxBIP125RBFSequence)
	craw, err := conflict.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Add(craw); err != nil {
		t.Fatal(err)
	}
	conflictID := txidToRPC(conflict.TxHash())

	replacement := buildSignedSpendSubmitSeq(coin, pkScript, priv, pubC, 800_000_000, wire.MaxBIP125RBFSequence)
	child := buildSignedSpendSubmit(replacement, pkScript, priv, pubC, 700_000_000)
	rraw, err := replacement.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	chraw, err := child.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{Utxo: utxo}
	arr, _ := json.Marshal([]string{hex.EncodeToString(rraw), hex.EncodeToString(chraw)})
	res, code, msg := execSubmitPackage(pool, nil, nil, nil, paths, []json.RawMessage{arr, json.RawMessage(`0`)}, nil, false, chain.RebootTestnet)
	if code != 0 {
		t.Fatalf("%d %s", code, msg)
	}
	m := res.(map[string]interface{})
	if m["package_msg"] != "success" {
		t.Fatalf("package_msg %#v results %#v", m["package_msg"], m["tx-results"])
	}
	replaced, ok := m["replaced-transactions"].([]string)
	if !ok || len(replaced) != 1 || replaced[0] != conflictID {
		t.Fatalf("replaced-transactions %#v want [%s]", m["replaced-transactions"], conflictID)
	}
	if pool.ContainsTxID(conflictID) {
		t.Fatal("conflict should be removed")
	}
}
