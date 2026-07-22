// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestShouldPurgeBodiesOnHeaderRewind(t *testing.T) {
	if shouldPurgeBodiesOnHeaderRewind(3360, 3120, 1) != true {
		t.Fatal("want purge when headers drop a period and bodies only through genesis")
	}
	if shouldPurgeBodiesOnHeaderRewind(3360, 3120, 3000) != false {
		t.Fatal("want keep bodies when contiguous near keep height")
	}
	if shouldPurgeBodiesOnHeaderRewind(3360, 3300, 1) != false {
		t.Fatal("want keep bodies on small header rewind")
	}
}
