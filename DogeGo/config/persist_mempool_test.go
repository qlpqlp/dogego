// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import "testing"

func TestEffectivePersistMempool(t *testing.T) {
	if !EffectivePersistMempool(File{}) {
		t.Fatal("default should persist")
	}
	off := false
	if EffectivePersistMempool(File{PersistMempool: &off}) {
		t.Fatal("expected off")
	}
}
