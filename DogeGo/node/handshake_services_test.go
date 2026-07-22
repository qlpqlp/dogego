// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"

	"dogego/chain"
	"dogego/wire"
)

func TestHandshakeAdvertisesCompactFiltersBit(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	svc := chain.EffectiveP2PServices(p, true, false)
	if svc&chain.ServiceCompactFilters == 0 {
		t.Fatalf("services %x", svc)
	}
	pl := wire.BuildVersionPayload(p.ProtocolVersion, svc, nil, 0, 1, "/DogeGo/", 0, true)
	dv, err := wire.ParseVersionPayload(pl)
	if err != nil {
		t.Fatal(err)
	}
	if dv.Services&chain.ServiceCompactFilters == 0 {
		t.Fatalf("parsed services %x", dv.Services)
	}
}
