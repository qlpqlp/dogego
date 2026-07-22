// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "testing"

func TestRebootTestnetDNSSeedsFirst(t *testing.T) {
	p, err := ParamsFor(RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.DNSSeeds) == 0 || p.DNSSeeds[0] != "seed.dogego.org" {
		t.Fatalf("reboot testnet DNSSeeds[0]=%#v want seed.dogego.org first", p.DNSSeeds)
	}
	got := WithDNSSeeds(p, []string{"extra.example.org", "SEED.dogego.org"})
	if len(got.DNSSeeds) < 2 || got.DNSSeeds[0] != "seed.dogego.org" || got.DNSSeeds[1] != "extra.example.org" {
		t.Fatalf("extra seeds should append after seed.dogego.org: %#v", got.DNSSeeds)
	}
	if len(p.FixedPeers) == 0 {
		t.Fatal("fixed Core pnSeed6_test peers should remain")
	}
}

func TestWithDNSSeedsDedupes(t *testing.T) {
	p, _ := ParamsFor(RebootTestnet)
	p.DNSSeeds = []string{"seed.example.com"}
	got := WithDNSSeeds(p, []string{"SEED.example.com", "other.org", ""})
	if len(got.DNSSeeds) != 2 {
		t.Fatalf("seeds %#v", got.DNSSeeds)
	}
	if got.DNSSeeds[0] != "seed.example.com" || got.DNSSeeds[1] != "other.org" {
		t.Fatalf("order %#v", got.DNSSeeds)
	}
}

func TestWithoutDNSSeeds(t *testing.T) {
	p, _ := ParamsFor(MainnetDogecoin)
	if len(p.DNSSeeds) == 0 {
		t.Fatal("mainnet should have DNS seeds")
	}
	got := WithoutDNSSeeds(p)
	if len(got.DNSSeeds) != 0 {
		t.Fatalf("DNSSeeds %#v", got.DNSSeeds)
	}
	if len(got.FixedPeers) == 0 {
		t.Fatal("fixed peers should remain")
	}
}
