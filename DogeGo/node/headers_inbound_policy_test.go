// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"fmt"
	"testing"
)

func TestInboundHeadersErrorPolicy(t *testing.T) {
	retry, pause, mis := InboundHeadersErrorPolicy(fmt.Errorf("headers: rewound journal to height 371336 after auxpow/legacy mismatch (retry getheaders)"))
	if !retry || pause || mis {
		t.Fatalf("rewind retry: retry=%v pause=%v mis=%v", retry, pause, mis)
	}
	retry, pause, mis = InboundHeadersErrorPolicy(errors.New("header batch index 0: legacy scrypt header after auxpow activation"))
	if retry || !pause || mis {
		t.Fatalf("auxpow peer: retry=%v pause=%v mis=%v", retry, pause, mis)
	}
	retry, pause, mis = InboundHeadersErrorPolicy(errors.New("header 3: bad prev"))
	if retry || !pause || mis {
		t.Fatalf("bad prev: retry=%v pause=%v mis=%v", retry, pause, mis)
	}
	retry, pause, mis = InboundHeadersErrorPolicy(errors.New("headers: fork rejected (insufficient chain work)"))
	if retry || pause || !mis {
		t.Fatalf("fork reject: retry=%v pause=%v mis=%v", retry, pause, mis)
	}
	retry, pause, mis = InboundHeadersErrorPolicy(errors.New("headers: fork deferred (marginal chain work +1; need >5 to reorg)"))
	if retry || pause || mis {
		t.Fatalf("marginal reorg: retry=%v pause=%v mis=%v", retry, pause, mis)
	}
}

func TestShouldTryNextHeaderSyncPeer(t *testing.T) {
	if !shouldTryNextHeaderSyncPeer(fmt.Errorf("headers: rewound journal to height 1 (retry getheaders)")) {
		t.Fatal("rewind retry")
	}
	if !shouldTryNextHeaderSyncPeer(errors.New("legacy scrypt header after auxpow")) {
		t.Fatal("auxpow peer")
	}
	if shouldTryNextHeaderSyncPeer(errors.New("headers: fork rejected (insufficient chain work)")) {
		t.Fatal("fork reject should stop")
	}
}
