// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestProbeMainnetFieldDiskStatusDefaultPath(t *testing.T) {
	st := ProbeMainnetFieldDiskStatus()
	if st.ChainDir == "" {
		t.Fatal("empty chain dir")
	}
	if st.HeadersPresent && st.Error != "" {
		t.Fatalf("headers present but error=%q", st.Error)
	}
	if st.LiveHeaderPoWReady && !st.HeadersPresent {
		t.Fatal("live without headers")
	}
	if st.LiveDiskConnectReady && !st.LiveHeaderPoWReady {
		t.Fatal("disk connect without header pow")
	}
}
