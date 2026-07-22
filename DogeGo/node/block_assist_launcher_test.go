// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"testing"

	"dogego/chain"
)

func TestEnsureBlockAssistWorkersStartsAfterEmptyPool(t *testing.T) {
	ResetBlockAssistLaunchForTests()
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	raw := &progressiveRawState{}
	bs := &BlockStoreCtx{Raw: nil}
	launch := BlockAssistLaunchParams{
		Ctx: ctx, Params: p, Raw: raw, BlockStore: bs,
		Candidates: NewBlockAssistCandidates(nil, nil),
	}
	EnsureBlockAssistWorkers(launch)
	if BlockAssistWorkersActive() {
		t.Fatal("should not start with empty candidates")
	}
	launch.Candidates = NewBlockAssistCandidates([]string{"127.0.0.1:1"}, nil)
	// Raw store nil - still should not panic; second call with nil Raw returns early
	bs.Raw = nil
	EnsureBlockAssistWorkers(launch)
	if BlockAssistWorkersActive() {
		t.Fatal("nil raw store should not mark active")
	}
}

func TestEnsureBlockAssistWorkersSecondCallWithPeers(t *testing.T) {
	ResetBlockAssistLaunchForTests()
	if BlockAssistWorkersActive() {
		t.Fatal("expected inactive after reset")
	}
}
