// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"os"
	"testing"
)

func TestCheckpointRPCEnrichDisabledByDefault(t *testing.T) {
	os.Unsetenv("DOGEGO_ENRICH_CHECKPOINT_RPC")
	if _, ok := tryReadMainnetCheckpointHeaderFromCoreCLI(104679); ok {
		t.Fatal("expected no RPC enrich without env")
	}
}
