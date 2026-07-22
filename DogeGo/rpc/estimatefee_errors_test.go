// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/consensus"
)

func TestSmartFeeErrorsInsufficient(t *testing.T) {
	orig := consensus.MinRelayTxFeePerKB()
	defer consensus.SetMinRelayTxFeePerKB(orig)

	_, _, errs := smartFeeKoinuPerKB(nil, 6, true)
	if len(errs) != 1 {
		t.Fatalf("errs %#v", errs)
	}
	m, ok := errs[0].(map[string]interface{})
	if !ok || m["type"] != "INSUFFICIENT_FEE" {
		t.Fatalf("%#v", errs[0])
	}
}

func TestSmartFeeErrorsEmptyWhenMarketData(t *testing.T) {
	paths := &DataPaths{
		MempoolFeeEstimate: func(int) uint64 { return 200_000 },
	}
	_, _, errs := smartFeeKoinuPerKB(paths, 6, false)
	if len(errs) != 0 {
		t.Fatalf("errs %#v", errs)
	}
}
