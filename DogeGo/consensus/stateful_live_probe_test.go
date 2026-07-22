// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"testing"

	"dogego/secp256k1"
)

func TestWalletAnchoredStatefulProbesMatchCorpus(t *testing.T) {
	sec := make([]byte, 32)
	sec[0] = 0x5a
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := standardP2PKHScript(h160[:])
	fund := WalletFundingUTXO{
		PrevHash: [32]byte{0xfe, 0xed},
		PrevIdx:  0,
		Value:    100_000_000,
		PkScript: pkScript,
	}
	for _, tmpl := range StatefulLiveProbeTemplates {
		t.Run(tmpl, func(t *testing.T) {
			probe, err := BuildWalletAnchoredStatefulProbe(tmpl, priv, pubC, fund, 500)
			if err != nil {
				t.Fatal(err)
			}
			vec, err := loadMempoolVector(tmpl)
			if err != nil {
				t.Fatal(err)
			}
			if probe.WantAccept != vec.WantAccept {
				t.Fatalf("want_accept probe=%v corpus=%v", probe.WantAccept, vec.WantAccept)
			}
			if !vec.WantAccept && vec.WantRejectReason != "" && probe.WantRejectReason != vec.WantRejectReason {
				t.Fatalf("reject reason probe=%q corpus=%q", probe.WantRejectReason, vec.WantRejectReason)
			}
			if probe.ProbeTxHex == "" {
				t.Fatal("empty probe hex")
			}
			if _, err := hex.DecodeString(probe.ProbeTxHex); err != nil {
				t.Fatalf("probe hex: %v", err)
			}
			row := EvalMempoolCorpusRow(vec)
			if !row.Stateful {
				t.Fatalf("template %q not stateful in corpus", tmpl)
			}
		})
	}
}

func TestStatefulLiveProbeFromFixturePrepNonEmpty(t *testing.T) {
	skipPrep := map[string]bool{
		"pq_commitment_op_return": true,
		"pq_carrier_p2sh_accept":  true,
	}
	for _, tmpl := range StatefulLiveProbeTemplates {
		if skipPrep[tmpl] {
			continue
		}
		probe, err := StatefulLiveProbeFromFixture(tmpl)
		if err != nil {
			t.Fatalf("%s: %v", tmpl, err)
		}
		if len(probe.PrepTxHex) == 0 {
			t.Fatalf("%s: expected prep txs", tmpl)
		}
	}
}
