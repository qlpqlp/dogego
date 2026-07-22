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

func TestShouldRotatePeerForForwardIBDFetch(t *testing.T) {
	if !shouldRotatePeerForForwardIBDFetch(errors.New("read tcp: i/o timeout"), 3086) {
		t.Fatal("i/o timeout at ancient height should rotate")
	}
	if !shouldRotatePeerForForwardIBDFetch(errors.New("read tcp: i/o timeout"), 10_000) {
		t.Fatal("i/o timeout should rotate peer during forward IBD")
	}
	if !shouldRotatePeerForForwardIBDFetch(errors.New("batch incomplete: 0/16 block(s) missing (notfound or timeout)"), 3086) {
		t.Fatal("batch incomplete should rotate during forward IBD")
	}
	if !shouldRotatePeerForForwardIBDFetch(ErrBlockDownloadStall, 5000) {
		t.Fatal("stall error should rotate")
	}
}
