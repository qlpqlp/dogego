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

	"dogego/consensus"
)

func TestIsMisbehaviorBlockError(t *testing.T) {
	if isMisbehaviorBlockError(errors.New("block parse: short")) != true {
		t.Fatal("parse error should count")
	}
	if isMisbehaviorBlockError(errors.New("defer connect at height 2")) != false {
		t.Fatal("defer connect should not count")
	}
	if isMisbehaviorBlockError(errors.New("connect height 2 pending (missing ancestor)")) != false {
		t.Fatal("pending connect should not count")
	}
	err := fmt.Errorf("block connect: bad-cb-amount: out %d subsidy %d fees 0",
		consensus.KoinuPerCoin*729752, consensus.KoinuPerCoin*553518)
	if isMisbehaviorBlockError(err) != false {
		t.Fatal("legacy subsidy local bug must not misbehavior peer")
	}
}
