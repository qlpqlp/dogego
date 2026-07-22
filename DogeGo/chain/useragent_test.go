// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import (
	"testing"

	"dogego/version"
)

func TestBuildSubVersion(t *testing.T) {
	wantBase := "/" + version.ClientName + ":" + version.Display() + "/"
	if got := BuildSubVersion(""); got != wantBase {
		t.Fatalf("empty: %q want %q", got, wantBase)
	}
	wantLab := "/" + version.ClientName + ":" + version.Display() + "(lab)/"
	if got := BuildSubVersion("  lab  "); got != wantLab {
		t.Fatalf("comment: %q want %q", got, wantLab)
	}
	wantBad := "/" + version.ClientName + ":" + version.Display() + "(bad)/"
	if got := BuildSubVersion("bad/)("); got != wantBad {
		t.Fatalf("strip: %q want %q", got, wantBad)
	}
}
