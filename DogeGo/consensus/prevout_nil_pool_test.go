// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/mempool"
)

func TestAdmissionPrevOutViewNilPoolPointer(t *testing.T) {
	var p *mempool.Pool
	if v := AdmissionPrevOutView(p, nil, nil); v != nil {
		t.Fatalf("expected nil view, got %T", v)
	}
}
