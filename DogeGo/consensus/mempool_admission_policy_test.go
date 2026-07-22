// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"strings"
	"testing"

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

func TestAcceptMempoolTxPolicyRejectsPackageAncestorLimit(t *testing.T) {
	fix, err := buildPackageAncestorLimitFixture()
	if err != nil {
		t.Fatal(err)
	}
	pool := mempool.New(100)
	if fix.Prep != nil {
		if err := fix.Prep(pool); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := wire.DeserializeTx(fix.Raw)
	if err != nil {
		t.Fatal(err)
	}
	sizes, err := pool.BuildMempoolSizes()
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckMempoolPackageLimits(tx, pool, sizes, 25, 25, 101); err == nil {
		t.Fatal("expected package ancestor reject from CheckMempoolPackageLimits")
	}
	adm := NewMempoolAdmission(pool, pool, nil, nil, nil, chain.RebootTestnet)
	polErr := AcceptMempoolTxPolicy(tx, adm)
	if polErr == nil {
		t.Fatal("expected package ancestor reject from AcceptMempoolTxPolicy")
	}
	if got := MempoolRejectReason(polErr); got != "too-long-mempool-chain" && !strings.HasPrefix(got, "too-long-mempool-chain") {
		t.Fatalf("reject %q", got)
	}
}
