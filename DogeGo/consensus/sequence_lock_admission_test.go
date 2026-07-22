// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"testing"

	"dogego/chain"
)

func TestMempoolAdmissionRejectsNonBIP68FinalWithViewHeights(t *testing.T) {
	spend, j, view := NonBIP68FinalDifferentialSpend()
	adm := MempoolAdmission{
		Journal: j,
		View:    view,
		Net:     chain.MainnetDogecoin,
	}
	if err := adm.CheckSequenceLocks(spend); err == nil {
		t.Fatal("expected non-BIP68-final reject")
	} else if !errors.Is(err, ErrSequenceLock) {
		t.Fatalf("got %v", err)
	}
}

func TestMempoolAdmissionSkipsSequenceLocksWithoutHeightSource(t *testing.T) {
	spend, j, view := NonBIP68FinalDifferentialSpend()
	view.heights = nil
	adm := MempoolAdmission{
		Journal: j,
		View:    view,
		Net:     chain.MainnetDogecoin,
	}
	if err := adm.CheckSequenceLocks(spend); err != nil {
		t.Fatalf("expected skip without height source, got %v", err)
	}
}
