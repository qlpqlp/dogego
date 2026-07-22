// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "testing"

func TestAssistPeerRegistryPerSessionBytes(t *testing.T) {
	r := NewAssistPeerRegistry()
	ctr := r.Register("93.184.216.1:22556", 2)
	if ctr == nil {
		t.Fatal("ctr")
	}
	ctr.addRecv(100)
	ctr.addSent(50)
	snaps := r.Snapshot()
	if len(snaps) != 1 {
		t.Fatalf("snaps %d", len(snaps))
	}
	if snaps[0].BytesRecv != 100 || snaps[0].BytesSent != 50 {
		t.Fatalf("bytes recv=%d sent=%d", snaps[0].BytesRecv, snaps[0].BytesSent)
	}
	recv, sent := r.NetBytes()
	if recv != 100 || sent != 50 {
		t.Fatalf("netbytes recv=%d sent=%d", recv, sent)
	}
	r.Unregister("93.184.216.1:22556")
	if r.Count() != 0 {
		t.Fatal("unregister")
	}
}
