// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestCloneStringAnyMapNested(t *testing.T) {
	src := map[string]any{
		"a": 1,
		"dogego_sync_activity": map[string]any{"headline": "old"},
	}
	cp := cloneStringAnyMap(src)
	act := cp["dogego_sync_activity"].(map[string]any)
	act["headline"] = "new"
	if src["dogego_sync_activity"].(map[string]any)["headline"] != "old" {
		t.Fatal("nested map must be cloned")
	}
}
