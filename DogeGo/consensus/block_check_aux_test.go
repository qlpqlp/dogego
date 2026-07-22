// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/primitives"
	"dogego/wire"
)

func TestCheckBlockAuxPowWrongChainID(t *testing.T) {
	h80 := auxChildHeader80()
	binary.LittleEndian.PutUint32(h80[0:4], 0x00630102) // chain id 99, mainnet wants 98
	var hdr primitives.BlockHeader
	if err := hdr.DecodeWire80(h80); err != nil {
		t.Fatal(err)
	}
	pb := &wire.ParsedBlock{
		Header: hdr,
		Aux:    minimalAuxPow(0x00000102),
		Txs: []*wire.Tx{{
			Version: 1,
			Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Script: []byte{0x01}}},
			Vout:    []wire.TxOut{{Value: 50e8, PkScript: []byte{0x51}}},
		}},
	}
	err := checkBlockAuxPow(pb, 2_000_000, chain.MainnetDogecoin)
	if err == nil || !strings.Contains(err.Error(), "bad-chain-id") {
		t.Fatalf("got %v", err)
	}
}

func TestRequireAuxJournalForExtend(t *testing.T) {
	verLE := binary.LittleEndian.Uint32(auxChildHeader80()[0:4])
	if err := requireAuxJournalForExtend(verLE, nil); err == nil {
		t.Fatal("expected error without aux journal")
	}
	if err := requireAuxJournalForExtend(1, nil); err != nil {
		t.Fatalf("legacy version: %v", err)
	}
}
