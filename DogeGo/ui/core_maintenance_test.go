// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/config"
)

func TestProbeCoreMaintenanceNoInvoke(t *testing.T) {
	r := ProbeCoreMaintenance("mainnet", config.File{}, nil)
	if len(r.Issues) == 0 || r.Issues[0] != "dogego_rpc_not_ready" {
		t.Fatalf("issues=%v", r.Issues)
	}
}
