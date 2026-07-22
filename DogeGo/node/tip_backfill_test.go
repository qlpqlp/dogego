// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"
)

func TestTipBackfillCoordinatorDeferred(t *testing.T) {
	c := newTipBackfillCoordinator(100, true)
	if c.done {
		t.Fatal("should not be done while deferred")
	}
	c.noteStartupRan()
	if c.done {
		t.Fatal("still deferred after startup skip")
	}
}

func TestTipBackfillCoordinatorImmediate(t *testing.T) {
	c := newTipBackfillCoordinator(100, false)
	if !c.done {
		t.Fatal("non-deferred with startup run should mark done via noteStartupRan")
	}
	c.noteStartupRan()
}
