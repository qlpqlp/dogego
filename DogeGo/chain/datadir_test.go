// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "testing"

func TestChainDataDirName(t *testing.T) {
	s, err := ChainDataDirName(MainnetDogecoin)
	if err != nil || s != "mainnet" {
		t.Fatalf("mainnet: %q %v", s, err)
	}
	s, err = ChainDataDirName(RebootTestnet)
	if err != nil || s != "testnet" {
		t.Fatalf("testnet: %q %v", s, err)
	}
}
