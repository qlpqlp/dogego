// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/chain"
)

func TestBuildSoftforksForTipMainnetActive(t *testing.T) {
	soft, bip9 := BuildSoftforksForTip(nil, 5_000_000, chain.MainnetDogecoin)
	if len(soft) != 3 {
		t.Fatalf("softforks=%d", len(soft))
	}
	csv, ok := bip9["csv"].(map[string]interface{})
	if !ok || csv["status"] != "active" {
		t.Fatalf("csv %#v", csv)
	}
}

func TestBuildSoftforksForTipBeforeCSV(t *testing.T) {
	_, bip9 := BuildSoftforksForTip(nil, 100_000, chain.MainnetDogecoin)
	csv := bip9["csv"].(map[string]interface{})
	if csv["status"] != "defined" {
		t.Fatalf("status=%v", csv["status"])
	}
	if csv["bit"].(int) != 0 {
		t.Fatalf("bit %#v", csv["bit"])
	}
	if csv["startTime"].(int64) != 1462060800 {
		t.Fatalf("startTime %#v", csv["startTime"])
	}
	seg := bip9["segwit"].(map[string]interface{})
	if seg["status"] != "defined" {
		t.Fatalf("segwit status=%v", seg["status"])
	}
}
