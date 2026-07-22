// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import (
	"encoding/hex"
)

// LegacyGenesisCoinbaseHex is the Core CreateGenesisBlock coinbase tx ("Nintondo", 88 DOGE subsidy).
// Mainnet and reboot testnet share this coinbase; only the 80-byte header differs (time/nonce/bits).
const LegacyGenesisCoinbaseHex = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff1004ffff001d0104084e696e746f6e646fffffffff010058850c020000004341040184710fa689ad5023690c80f3a49c8f13f8d45b8c857fbcbc8bc4a8e4d3eb4b10f4d4604fa08dce601aaf0f470216fe1b51850b4acf21b179c45070ac7b03a9ac00000000"

// RebootTestnetGenesisCoinbaseHex is an alias kept for existing callers/tests.
const RebootTestnetGenesisCoinbaseHex = LegacyGenesisCoinbaseHex

// GenesisBlockRaw returns the full serialized legacy block at height 0 from chainparams (Core CreateGenesisBlock).
func GenesisBlockRaw(net Network) ([]byte, error) {
	p, err := ParamsFor(net)
	if err != nil {
		return nil, err
	}
	h80, err := genesisHeader80(p)
	if err != nil {
		return nil, err
	}
	txRaw, err := hex.DecodeString(LegacyGenesisCoinbaseHex)
	if err != nil {
		return nil, err
	}
	var buf []byte
	buf = append(buf, h80...)
	buf = append(buf, 1)
	buf = append(buf, txRaw...)
	return buf, nil
}

// RebootTestnetGenesisBlockRaw returns the reboot testnet height-0 block.
func RebootTestnetGenesisBlockRaw() ([]byte, error) {
	return GenesisBlockRaw(RebootTestnet)
}

// MainnetGenesisBlockRaw returns the Dogecoin mainnet height-0 block.
func MainnetGenesisBlockRaw() ([]byte, error) {
	return GenesisBlockRaw(MainnetDogecoin)
}

func genesisHeader80(p Params) ([]byte, error) {
	var h [80]byte
	putU32LE(h[0:4], uint32(p.GenesisVer))
	merkle, err := Hash256FromDisplayHex(p.GenesisMerkleRootHex)
	if err != nil {
		return nil, err
	}
	copy(h[36:68], merkle[:])
	putU32LE(h[68:72], p.GenesisTime)
	putU32LE(h[72:76], p.GenesisBits)
	putU32LE(h[76:80], p.GenesisNonce)
	return h[:], nil
}

func putU32LE(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
