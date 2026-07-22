// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/wire"
)

func TestRebootTestnetGenesisBlockRaw(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	merkle, err := chain.Hash256FromDisplayHex(p.GenesisMerkleRootHex)
	if err != nil {
		t.Fatal(err)
	}
	txRaw, err := hex.DecodeString(chain.RebootTestnetGenesisCoinbaseHex)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := wire.ReadTx(bytes.NewReader(txRaw))
	if err != nil {
		t.Fatal(err)
	}
	root := wire.BlockMerkleRoot([]*wire.Tx{tx})
	if string(root[:]) != string(merkle[:]) {
		t.Fatalf("merkle %x want %x", root, merkle)
	}
	raw, err := chain.RebootTestnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	if got := pow.BlockHashHex(raw[:80]); got != p.GenesisBlockHashHex {
		t.Fatalf("block hash %s want %s", got, p.GenesisBlockHashHex)
	}
	pb, err := wire.ParseBlock(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := wire.VerifyBlockMerkle(pb); err != nil {
		t.Fatal(err)
	}
}

func TestMainnetGenesisBlockRaw(t *testing.T) {
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	if got := pow.BlockHashHex(raw[:80]); got != p.GenesisBlockHashHex {
		t.Fatalf("block hash %s want %s", got, p.GenesisBlockHashHex)
	}
	pb, err := wire.ParseBlock(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := wire.VerifyBlockMerkle(pb); err != nil {
		t.Fatal(err)
	}
	if len(raw) < 200 {
		t.Fatalf("mainnet genesis len %d below adequate threshold for height 0", len(raw))
	}
}
