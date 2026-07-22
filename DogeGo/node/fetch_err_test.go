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

func TestSessionFailureHardFromFetchErr(t *testing.T) {
	if sessionFailureHardFromFetchErr(errors.New("reject before block: bad")) != true {
		t.Fatal("reject should be hard")
	}
	if sessionFailureHardFromFetchErr(errors.New("notfound for block")) != true {
		t.Fatal("notfound should be hard")
	}
	if sessionFailureHardFromFetchErr(errors.New("batch incomplete: 3/16 block(s) missing (notfound or timeout)")) != true {
		t.Fatal("batch incomplete should be hard")
	}
	if sessionFailureHardFromFetchErr(errors.New("raw block too short at height 1: 190 bytes (need >= 280)")) != true {
		t.Fatal("too short should be hard")
	}
	if sessionFailureHardFromFetchErr(errors.New("batch incomplete: 1/1 missing; rejected 1 undersized stub(s)")) != true {
		t.Fatal("stub batch should be hard")
	}
	if sessionFailureHardFromFetchErr(errors.New("timeout: no valid block")) != false {
		t.Fatal("plain timeout should be soft")
	}
	if sessionFailureHardFromFetchErr(errors.New("bad magic fdd00702, expected c0c0c0c0")) != true {
		t.Fatal("bad magic should be hard")
	}
	if sessionFailureHardFromFetchErr(context.Canceled) != false {
		t.Fatal("shutdown cancel should be soft")
	}
}
