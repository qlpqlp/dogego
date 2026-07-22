// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"
	"testing"

	"dogego/chain"
)

func TestIsPostAuxEraStallTipMainnet(t *testing.T) {
	if !isPostAuxEraStallTipMainnet(chain.MainnetDogecoin, 510_000) {
		t.Fatal("510000 should be in stall band")
	}
	if isPostAuxEraStallTipMainnet(chain.MainnetDogecoin, 600_000) {
		t.Fatal("600000 outside band")
	}
	if isPostAuxEraStallTipMainnet(chain.RebootTestnet, 510_000) {
		t.Fatal("testnet should not use mainnet stall band")
	}
}

func TestMaybeNotePostAuxEraHeaderStall_setsHint(t *testing.T) {
	clearHeaderSyncRecoveryHint()
	maybeNotePostAuxEraHeaderStall(chain.MainnetDogecoin, 510_000)
	h := headerSyncRecoveryHintStr()
	if h == "" || !strings.Contains(h, "510000") {
		t.Fatalf("hint=%q", h)
	}
	clearHeaderSyncRecoveryHint()
}

func TestMaybeClearPostAuxEraStallHint(t *testing.T) {
	setHeaderSyncRecoveryHint(postAuxEraStallRecoveryHint())
	maybeClearPostAuxEraStallHint(chain.MainnetDogecoin, 510_501)
	if headerSyncRecoveryHintStr() != "" {
		t.Fatalf("hint should clear past band, got %q", headerSyncRecoveryHintStr())
	}
	setHeaderSyncRecoveryHint(postAuxEraStallRecoveryHint())
	maybeClearPostAuxEraStallHint(chain.MainnetDogecoin, 510_100)
	if headerSyncRecoveryHintStr() == "" {
		t.Fatal("hint must remain inside band")
	}
	clearHeaderSyncRecoveryHint()
}

func TestMaybeKickPostAuxEraHeaderRecovery_nilJournal(t *testing.T) {
	var kicked bool
	maybeKickPostAuxEraHeaderRecovery(chain.MainnetDogecoin, nil, func(bool) bool {
		kicked = true
		return true
	})
	if kicked {
		t.Fatal("nil journal must not kick")
	}
}
