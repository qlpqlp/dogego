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
)

func TestMempoolRejectsDiscouragedNOP1InScriptSig(t *testing.T) {
	flags := ScriptFlagsForMempool(0, chain.MainnetDogecoin, nil)
	if err := checkScriptDiscouragedOps([]byte{0xb0}, flags); err == nil || !strings.Contains(err.Error(), "DISCOURAGE") {
		t.Fatalf("want discourage error, got %v", err)
	}
}

func TestConnectBlockAllowsNOP1WithoutDiscourageFlag(t *testing.T) {
	flags := ScriptFlagsForChain(0, chain.MainnetDogecoin, nil)
	if err := checkScriptDiscouragedOps([]byte{0xb0}, flags); err != nil {
		t.Fatalf("connect flags should not scan discourage: %v", err)
	}
}
