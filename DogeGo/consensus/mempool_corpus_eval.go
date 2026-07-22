// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"strings"

	"dogego/mempool"
	"dogego/secp256k1"
	"dogego/wire"
)

// MempoolCorpusEvalResult is one offline evaluation of core_mempool_vectors.json.
type MempoolCorpusEvalResult struct {
	Name             string `json:"name"`
	Template         string `json:"template"`
	WantAccept       bool   `json:"want_accept"`
	WantRejectReason string `json:"want_reject_reason,omitempty"`
	GotAccept        bool   `json:"got_accept"`
	GotRejectReason  string `json:"got_reject_reason,omitempty"`
	Match            bool   `json:"match"`
	Error            string `json:"error,omitempty"`
	Stateful         bool   `json:"stateful,omitempty"`
}

// EvalMempoolCorpus runs all rows from core_mempool_vectors.json offline (harness parity).
func EvalMempoolCorpus() ([]MempoolCorpusEvalResult, error) {
	vecs, err := LoadMempoolDifferentialVectors()
	if err != nil {
		return nil, err
	}
	out := make([]MempoolCorpusEvalResult, 0, len(vecs))
	for _, v := range vecs {
		out = append(out, EvalMempoolCorpusRow(v))
	}
	return out, nil
}

// EvalMempoolCorpusRow evaluates one mempool differential vector offline.
func EvalMempoolCorpusRow(v MempoolDifferentialVector) MempoolCorpusEvalResult {
	res := MempoolCorpusEvalResult{
		Name:             v.Name,
		Template:         v.Template,
		WantAccept:       v.WantAccept,
		WantRejectReason: v.WantRejectReason,
	}
	err := evalMempoolCorpusTemplate(v.Template)
	if err == nil {
		res.GotAccept = true
	} else {
		res.GotRejectReason = MempoolRejectReason(err)
	}
	res.Match = corpusRowMatches(res.GotAccept, res.GotRejectReason, v.WantAccept, v.WantRejectReason)
	if !res.Match && err != nil && res.GotRejectReason == "" {
		res.Error = err.Error()
	}
	res.Stateful = !rpcStatelessMempoolTemplates[v.Template]
	return res
}

func corpusRowMatches(gotAccept bool, gotReason string, wantAccept bool, wantReason string) bool {
	if gotAccept != wantAccept {
		return false
	}
	if wantAccept {
		return true
	}
	if gotReason == wantReason || strings.HasPrefix(gotReason, wantReason) {
		return true
	}
	return wantReason != "" && strings.HasPrefix(wantReason, gotReason)
}

func evalMempoolCorpusTemplate(template string) error {
	switch template {
	case "min_relay_fee", "rbf_insufficient_fee", "rbf_sufficient_fee", "rbf_not_replaceable", "rbf_fullrbf", "coinbase_immature", "rbf_too_many_descendants", "rbf_too_many_conflicts", "rbf_new_unconfirmed_input", "non_bip68_final":
		return EvaluateMempoolDifferentialCheck(template)
	case "package_ancestor_limit":
		return evalPackageAncestorLimit()
	case "mempool_double_spend":
		return evalMempoolDoubleSpend()
	case "package_descendant_limit":
		return evalPackageDescendantLimit()
	case "package_ancestor_size":
		return evalPackageAncestorSize()
	case "package_descendant_size":
		return evalPackageDescendantSize()
	}
	tx, adm, err := buildMempoolAdmissionCase(template)
	if err != nil {
		return err
	}
	return AcceptMempoolTxAdmission(tx, adm)
}

func evalPackageAncestorLimit() error {
	pool := mempool.New(100)
	var prev [32]byte
	prev[0] = 0xaa
	parentHash := prev
	for i := 0; i < 26; i++ {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: fixtureP2PKHScript()}},
		}
		raw, err := parent.Serialize()
		if err != nil {
			return err
		}
		if err := pool.Add(raw); err != nil {
			return err
		}
		parentHash = parent.TxHash()
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: fixtureChildOutValue(), PkScript: fixtureP2PKHScript()}},
	}
	sizes, err := pool.BuildMempoolSizes()
	if err != nil {
		return err
	}
	return CheckMempoolPackageLimits(child, pool, sizes, 25, 25, 101)
}

func evalPackageDescendantLimit() error {
	pool := mempool.New(100)
	var prevHash [32]byte
	prevHash[0] = 0xaa
	for i := 0; i < 25; i++ {
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: fixtureP2PKHScript()}},
		}
		raw, err := tx.Serialize()
		if err != nil {
			return err
		}
		if err := pool.Add(raw); err != nil {
			return err
		}
		prevHash = tx.TxHash()
	}
	sizes, err := pool.BuildMempoolSizes()
	if err != nil {
		return err
	}
	extra := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: fixtureChildOutValue(), PkScript: fixtureP2PKHScript()}},
	}
	return CheckMempoolPackageLimits(extra, pool, sizes, 25, 25, 101)
}

func evalPackageAncestorSize() error {
	pool := mempool.New(100)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff, Script: make([]byte, 900)}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: fixtureP2PKHScript()}},
	}
	if err := pool.Add(parent.SerializeForHash()); err != nil {
		return err
	}
	child := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 49_000_000, PkScript: fixtureP2PKHScript()}},
	}
	ph := parent.TxHash()
	view := mempoolStubPrevOutView{}
	view[outpointKey(ph, 0)] = PrevOut{Value: parent.Vout[0].Value, PkScript: parent.Vout[0].PkScript}
	return CheckMempoolPackageSizeLimits(child, pool, view, 1, 101)
}

func evalPackageDescendantSize() error {
	pool := mempool.New(100)
	parent := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: fixtureP2PKHScript()}},
	}
	if err := pool.Add(parent.SerializeForHash()); err != nil {
		return err
	}
	child1 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff, Script: make([]byte, 900)}},
		Vout:    []wire.TxOut{{Value: 49_000_000, PkScript: fixtureP2PKHScript()}},
	}
	if err := pool.Add(child1.SerializeForHash()); err != nil {
		return err
	}
	child2 := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: child1.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 48_000_000, PkScript: fixtureP2PKHScript()}},
	}
	ph := parent.TxHash()
	c1h := child1.TxHash()
	view := mempoolStubPrevOutView{}
	view[outpointKey(ph, 0)] = PrevOut{Value: parent.Vout[0].Value, PkScript: parent.Vout[0].PkScript}
	view[outpointKey(c1h, 0)] = PrevOut{Value: child1.Vout[0].Value, PkScript: child1.Vout[0].PkScript}
	return CheckMempoolPackageSizeLimits(child2, pool, view, 101, 1)
}

func evalMempoolDoubleSpend() error {
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
	fundRaw, err := funding.Serialize()
	if err != nil {
		return err
	}
	pool := mempool.New(10)
	if err := pool.Add(fundRaw); err != nil {
		return err
	}
	spend1, err := buildSignedSpendTx(funding, pkScript, priv, pubC, 900_000_000)
	if err != nil {
		return err
	}
	if err := pool.Add(spend1.SerializeForHash()); err != nil {
		return err
	}
	spend2, err := buildSignedSpendTx(funding, pkScript, priv, pubC, 800_000_000)
	if err != nil {
		return err
	}
	adm := MempoolAdmission{
		View:             AdmissionPrevOutView(pool, nil, nil),
		Pool:             pool,
		MinRelayFeePerKB: DefaultMinRelayTxFeePerKB,
	}
	return adm.CheckSpendConflicts(spend2)
}

// SummarizeMempoolCorpusEval tallies pass/fail counts for probe responses.
func SummarizeMempoolCorpusEval(rows []MempoolCorpusEvalResult) (passed, failed int, ok bool) {
	for _, r := range rows {
		if r.Match {
			passed++
		} else {
			failed++
		}
	}
	return passed, failed, failed == 0 && len(rows) > 0
}

// MempoolCorpusEvalHint explains offline vs live RPC probe scope.
func MempoolCorpusEvalHint(statelessRPCRows int) string {
	if statelessRPCRows <= 0 {
		statelessRPCRows = 32
	}
	total := 58
	if vecs, err := LoadMempoolDifferentialVectors(); err == nil && len(vecs) > 0 {
		total = len(vecs)
	}
	return fmt.Sprintf("Offline harness for all core_mempool_vectors.json templates (%d rows). Live Core RPC comparison uses %d stateless rows in mempool_parity_rpc.json.", total, statelessRPCRows)
}
