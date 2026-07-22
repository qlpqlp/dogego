// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus_test

import (
	"encoding/binary"
	"testing"

	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
)

func TestValidateHeaderChainLink(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	p.RelaxedPoW = true
	g80, err := pow.Header80()
	if err != nil {
		t.Fatal(err)
	}
	tip := pow.BlockHashLE(g80[:])
	var h1 [80]byte
	copy(h1[0:4], g80[0:4])
	copy(h1[4:36], tip[:])
	copy(h1[36:68], g80[36:68])
	binary.LittleEndian.PutUint32(h1[68:72], binary.LittleEndian.Uint32(g80[68:72])+1)
	binary.LittleEndian.PutUint32(h1[72:76], binary.LittleEndian.Uint32(g80[72:76]))
	binary.LittleEndian.PutUint32(h1[76:80], binary.LittleEndian.Uint32(g80[76:80])+1)
	if err := consensus.ValidateHeaderChain(p, tip, [][]byte{h1[:]}); err != nil {
		t.Fatal(err)
	}
}
