// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"os"
	"testing"
)

func TestApplySetupParityEnv(t *testing.T) {
	t.Setenv("DOGEGO_CORE_COMPARE", "")
	t.Setenv("DOGEGO_CORE_COMPARE_MIN", "")
	ApplySetupParityEnv([]string{
		"DOGEGO_CORE_COMPARE=1",
		"DOGEGO_CORE_COMPARE_REQUIRED=1",
		"DOGEGO_CORE_COMPARE_MIN=24",
		"DOGEGO_CORE_RPC_PORT=44555",
	})
	if os.Getenv("DOGEGO_CORE_COMPARE") != "1" {
		t.Fatal("compare")
	}
	if os.Getenv("DOGEGO_CORE_COMPARE_MIN") != "24" {
		t.Fatal("compare min")
	}
	if !setupParityEnvConfigured() {
		t.Fatal("setup parity env not configured after apply")
	}
}

func TestVerifyProvisionRunSetupOfflineFails(t *testing.T) {
	if !hasGo() {
		t.Skip("go not in PATH")
	}
	r := VerifyProvision(ProvisionOptions{OfflineOnly: true, RunSetup: true})
	if r.OK {
		t.Fatalf("expected failure: %+v", r)
	}
	found := false
	for _, i := range r.Issues {
		if i == "run_setup_requires_live" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("issues=%v", r.Issues)
	}
}
