// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"errors"
	"testing"
)

func TestIsBenignShutdownErr(t *testing.T) {
	if !IsBenignShutdownErr(context.Canceled) {
		t.Fatal("context.Canceled")
	}
	if !IsBenignShutdownErr(errors.New("read: context canceled")) {
		t.Fatal("wrapped message")
	}
	if IsBenignShutdownErr(errors.New("bad magic")) {
		t.Fatal("sync error should not be benign")
	}
}
