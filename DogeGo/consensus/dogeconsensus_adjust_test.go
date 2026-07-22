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

func TestDifficultyAdjustmentBlocks(t *testing.T) {
	pre := LookupConsensus(chain.MainnetDogecoin, 100_000)
	if pre.DifficultyAdjustmentBlocks() != 240 {
		t.Fatalf("pre-digishield want 240 got %d", pre.DifficultyAdjustmentBlocks())
	}
	post := LookupConsensus(chain.MainnetDogecoin, 500_000)
	if post.DifficultyAdjustmentBlocks() != 1 {
		t.Fatalf("post-digishield want 1 got %d", post.DifficultyAdjustmentBlocks())
	}
}
