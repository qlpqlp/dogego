// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/chain"
)

func TestScriptFlagsCSVWhenBIP9ActiveBeforeBuriedHeight(t *testing.T) {
	if testing.Short() {
		t.Skip("needs mainnet-scale version-bits period")
	}
	const period = 10080
	headers := make([]header80, period*4)
	for i := range headers {
		sig := i >= period
		headers[i] = header80{version: vbVersion(sig), time: uint32(1462060800 + i)}
	}
	j := &vbJournal{headers: headers}
	p := BIP9ParamsForNetwork(chain.MainnetDogecoin)
	var dep BIP9Deployment
	for _, d := range p.Deployments {
		if d.Name == "csv" {
			dep = d
			break
		}
	}
	r, err := EvaluateBIP9AtTip(j, chain.MainnetDogecoin, dep, p.Period, p.Threshold)
	if err != nil || r.Status != ThresholdActive {
		t.Fatalf("csv deployment %s err=%v", r.Status, err)
	}
	f := ScriptFlagsForChain(100_000, chain.MainnetDogecoin, j)
	if f&ScriptVerifyCheckSequenceVerify == 0 {
		t.Fatal("expected CSV script flag when deployment active at tip")
	}
}

func TestScriptFlagsCSVWithoutJournalUsesBuriedHeight(t *testing.T) {
	f := ScriptFlagsForChain(419327, chain.MainnetDogecoin, nil)
	if f&ScriptVerifyCheckSequenceVerify != 0 {
		t.Fatal("CSV flag should be off before buried height without journal")
	}
	f2 := ScriptFlagsForChain(419328, chain.MainnetDogecoin, nil)
	if f2&ScriptVerifyCheckSequenceVerify == 0 {
		t.Fatal("CSV flag should be on at buried height")
	}
}
