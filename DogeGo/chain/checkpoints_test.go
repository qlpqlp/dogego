// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "testing"

func TestCheckpointHashAtMainnetGenesis(t *testing.T) {
	h, ok := CheckpointHashAt(MainnetDogecoin, 0)
	if !ok || h != "1a91e3dace36e2be3bf030a65679fe821aa1d6ef92e7c9902eb318182c355691" {
		t.Fatalf("got %q ok=%v", h, ok)
	}
}

func TestCheckpointHashAtRebootTestnetGenesis(t *testing.T) {
	h, ok := CheckpointHashAt(RebootTestnet, 0)
	if !ok || h != GenesisBlockHashHex {
		t.Fatalf("got %q want %s", h, GenesisBlockHashHex)
	}
}
