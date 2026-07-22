// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"testing"
)

func TestBackgroundHeaderSyncContinuesOnRewindRetry(t *testing.T) {
	err := errors.New("headers: rewound journal to height 371336 after auxpow/legacy mismatch (retry getheaders)")
	if !shouldTryNextHeaderSyncPeer(err) {
		t.Fatal("background should try next peer after local rewind during getheaders")
	}
	if recoverableHeaderPeerErr(err) {
		t.Fatal("rewind retry is not generic recoverable transport")
	}
}
