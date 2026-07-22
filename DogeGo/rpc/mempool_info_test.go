// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"

	"dogego/consensus"
	"dogego/mempool"
)

func TestGetMempoolInfoMinRelayVsMempoolMin(t *testing.T) {
	orig := consensus.MinRelayTxFeePerKB()
	defer consensus.SetMinRelayTxFeePerKB(orig)

	pool := mempool.New(10)
	pool.SetIncrementalRelayFeePerKB(100_000)
	consensus.SetMinRelayTxFeePerKB(100_000)
	pool.SetRollingMinFeePerKBForTest(500_000)

	paths := &DataPaths{
		MempoolMinRelayFee: func() uint64 { return pool.MinRelayFeePerKB() },
	}
	effective := minRelayFeeFromPaths(paths)
	if effective < 500_000 {
		t.Fatalf("effective %d want >= 500000", effective)
	}

	res, code, msg := execGetMempoolInfo(pool, nil, nil, effective, 0, 0, 300*1000*1000, false, 500_000, paths)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("result type %T", res)
	}
	minRelay, _ := m["minrelaytxfee"].(float64)
	mempoolMin, _ := m["mempoolminfee"].(float64)
	if minRelay >= mempoolMin {
		t.Fatalf("minrelaytxfee %v should be < mempoolminfee %v", minRelay, mempoolMin)
	}
	switch exp := m["mempoolexpiry"].(type) {
	case float64:
		if int(exp) != mempool.DefaultMempoolExpiryHours {
			t.Fatalf("mempoolexpiry %v", exp)
		}
	case int:
		if exp != mempool.DefaultMempoolExpiryHours {
			t.Fatalf("mempoolexpiry %d", exp)
		}
	default:
		t.Fatalf("mempoolexpiry type %T", m["mempoolexpiry"])
	}
}
