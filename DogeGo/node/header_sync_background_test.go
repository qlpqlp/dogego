// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestRecoveryShouldRefreshDiscovery(t *testing.T) {
	if !recoveryShouldRefreshDiscovery(1, 10) {
		t.Fatal("pass 1 should refresh")
	}
	if !recoveryShouldRefreshDiscovery(4, 10) {
		t.Fatal("pass 4 should refresh")
	}
	if recoveryShouldRefreshDiscovery(2, 10) {
		t.Fatal("pass 2 should not refresh when candidates exist")
	}
	if !recoveryShouldRefreshDiscovery(3, 0) {
		t.Fatal("zero candidates should refresh")
	}
}
