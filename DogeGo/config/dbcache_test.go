// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import "testing"

func TestEffectiveDBCacheMB(t *testing.T) {
	if g := EffectiveDBCacheMB(2000, 8000); g != 2000 {
		t.Fatalf("explicit: got %d", g)
	}
	if g := EffectiveDBCacheMB(0, -1); g != DefaultDBCacheMB {
		t.Fatalf("unknown free: got %d want %d", g, DefaultDBCacheMB)
	}
	// 16 GiB free → (16384-2048)*80% = 11468.8 → 11468
	if g := EffectiveDBCacheMB(0, 16384); g < 8000 || g > MaxAutoDBCacheMB {
		t.Fatalf("auto large: got %d", g)
	}
	if g := EffectiveDBCacheMB(0, 500); g < MinAutoDBCacheMB && g != DefaultDBCacheMB {
		t.Fatalf("tight free: got %d", g)
	}
	if g := EffectiveDBCacheMB(99999, 1000); g != MaxAutoDBCacheMB {
		t.Fatalf("cap: got %d", g)
	}
}
