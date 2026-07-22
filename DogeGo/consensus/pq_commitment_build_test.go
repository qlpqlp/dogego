// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestBuildPQCommitmentScriptRoundTrip(t *testing.T) {
	commit := make([]byte, 32)
	commit[0] = 0xaa
	script, err := BuildPQCommitmentScript(PQTagDilithium, commit)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := DetectPQCommitment(script)
	if !ok || c.Tag != PQTagDilithium {
		t.Fatalf("%+v ok=%v", c, ok)
	}
}
