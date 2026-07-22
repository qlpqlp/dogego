// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/clock"
	"dogego/wire"
)

func TestWireNetworkUnixHandshakeOffset(t *testing.T) {
	clock.SetMockUnix(1_000_000)
	defer clock.SetMockUnix(0)
	dv := &wire.DecodedVersion{Timestamp: 1_000_090}
	got := wireNetworkUnix(nil, dv)
	if got != 1_000_090 {
		t.Fatalf("got %d want 1000090", got)
	}
}
