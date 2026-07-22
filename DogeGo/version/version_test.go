// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package version

import (
	"strings"
	"testing"
)

func TestDisplayBeta(t *testing.T) {
	if !Beta {
		t.Skip("beta off")
	}
	if Display() != ClientVersion+"-beta ("+CoreBaseVersion+")" {
		t.Fatalf("Display() = %q", Display())
	}
}

func TestDisplayIncludesCoreBase(t *testing.T) {
	if !strings.Contains(Display(), CoreBaseVersion) {
		t.Fatalf("Display() = %q missing Core %s", Display(), CoreBaseVersion)
	}
}

func TestBuildSubVersion(t *testing.T) {
	got := BuildSubVersion("my node")
	want := "/" + ClientName + ":" + Display() + "(my node)/"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHTTPUserAgent(t *testing.T) {
	if HTTPUserAgent() != ClientName+"/"+Display() {
		t.Fatalf("HTTPUserAgent() = %q", HTTPUserAgent())
	}
}
