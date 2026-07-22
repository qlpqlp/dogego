// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/pow"
)

// mainnetFieldBlockEntry is committed hex from export_mainnet_field_blocks.ps1 (Core or DogeGo RPC).
type mainnetFieldBlockEntry struct {
	Height int64  `json:"height"`
	Hex    string `json:"hex"`
}

type mainnetCanonicalBlockSpec struct {
	Height      int64
	WantHash    string
	PrevHash    string
	MerkleRoot  string
	Time        uint32
	Bits        uint32
	Nonce       uint32
	CoinbaseHex string
	// FullHex is optional committed raw block (Blockchair/Core getblock); used when indexer metadata cannot be reassembled.
	FullHex string
}

// mainnetCanonicalBlockSpecs are Core-chain mainnet blocks (verified block hashes).
// Coinbase tx hex from public chain indexers; headers match Dogecoin Core wire encoding.
var mainnetCanonicalBlockSpecs = []mainnetCanonicalBlockSpec{
	{
		Height:      1,
		WantHash:    "82bc68038f6034c0596b6e313729793a887fded6e92a31fbdf70863f89d9bea2",
		PrevHash:    "1a91e3dace36e2be3bf030a65679fe821aa1d6ef92e7c9902eb318182c355691",
		MerkleRoot:  "5f7e779f7600f54e528686e91d5891f3ae226ee907f461692519e549105f521c",
		Time:        1386474927,
		Bits:        0x1e0ffff0,
		Nonce:       1417875456,
		CoinbaseHex: "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0e04afeda3520102062f503253482fffffffff01004023ef3806000023210338bf57d51a50184cf5ef0dc42ecd519fb19e24574c057620262cc1df94da2ae5ac00000000",
	},
	{
		Height:      2,
		WantHash:    "ea5380659e02a68c073369e502125c634b2fb0aaf351b9360c673368c4f20c96",
		PrevHash:    "82bc68038f6034c0596b6e313729793a887fded6e92a31fbdf70863f89d9bea2",
		MerkleRoot:  "3b14b76d22a3f2859d73316002bc1b9bfc7f37e2c3393be9b722b62bbd786983",
		Time:        1386474933,
		Bits:        0x1e0ffff0,
		Nonce:       3404207872,
		CoinbaseHex: "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0e04b5eda3520101062f503253482fffffffff010098dfdc5e42000023210245177fc121518d812eeb3afbc746afa9e3a1a589b617f028a75c6089b846685eac00000000",
	},
	{
		Height:      3,
		WantHash:    "76f80a8a81e6f6669d340651723b874f97395c4dbda200f8b024df4c6566a92c",
		PrevHash:    "ea5380659e02a68c073369e502125c634b2fb0aaf351b9360c673368c4f20c96",
		MerkleRoot:  "1e10c28574e3b9d7032329b624ce4ac8064d0e91324aa14634aa2da61146ddfd",
		Time:        1386474940,
		Bits:        0x1e0ffff0,
		Nonce:       3785361152,
		CoinbaseHex: "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0e04bceda3520101062f503253482fffffffff0100cfdf5f0401000023210382d15f92b26ac556b5fe13df19629be1f68440547fcb236516757cf64f6bff6cac00000000",
	},
	{
		Height:      100,
		WantHash:    "48d8b2a0ea39518a288f739d10c13fb9e4ef713198fc2b7c81d0be2646ebd8d2",
		PrevHash:    "0aadf846868fc48f790a8874e0b7f3beef0334700a5c57a8a6dda9b19835dd30",
		MerkleRoot:  "920c67666f1a6d3c7866813cbfee955b3dd663088b7f3c4f9567fb689b5d1c6c",
		Time:        1386475411,
		Bits:        0x1e0ffff0,
		Nonce:       3616408064,
		CoinbaseHex: "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0e0493efa3520101062f503253482fffffffff01001333d1325100002321024274baa26a5da9cf9ddc784d6a5cb439cfa840c0eb77c60f2448ff88beb5a734ac00000000",
	},
	{
		Height:      200,
		WantHash:    "79fcadf5e7a955c0b6f026d68d0fa768968b0975233608b6590011e4776aab67",
		PrevHash:    "8890102516608058a2b85c5353aae662802846d5569869d9091e0882bc5bf23e",
		MerkleRoot:  "fa60c961bd4822fde7b28b9b1eb1b05489dde95e6c50c7588fa2907c6d36bf67",
		Time:        1386475606,
		Bits:        0x1e0ffff0,
		Nonce:       3852795904,
		CoinbaseHex: "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0e0456f0a3520102062f503253482fffffffff0100c87cbbe523000023210268dbf0532c8123a3c7bfc91d4d380a255e4c37ea163935523403849c237589f5ac00000000",
	},
	{
		Height:      272,
		WantHash:    "e0128b097df99aeebb1ba9e6b0a7c4875632e02f3a2670ddb2b9abfa995a1983",
		PrevHash:    "8093adb5e0947df701d236fff87bf7e9864a32eb227eb85cbde79c2e82961fe6",
		MerkleRoot:  "c6dcfd59431a7a5b7a5e57cd85d30b72fb9be6c840e3f5ed4251dfe4b58cff0e",
		Time:        1386475664,
		Bits:        0x1e0fffff,
		Nonce:       2314273536,
		CoinbaseHex: "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0e0490f0a3520103062f503253482fffffffff0100df89329c1d00002321034208c11189c936c0ce37d3f7ea6434cb430eb7a0d3586ff80beac49d7d823d64ac00000000",
	},
	{
		Height:      10006,
		WantHash:    "db9434cc7dedd6efcafb7dcd73913e836668ff997a583b2d410b9a15f8a2b053",
		PrevHash:    "e14814371ab4f855a11395ef959485b17288ce565240f09367abda03b9799041",
		MerkleRoot:  "7e9405a53e86f3621fd629806ffbd0c5932b59b050b5345f8fa3ef522e70db16",
		Time:        1386976766,
		Bits:        0x1c145dba,
		Nonce:       381604608,
		CoinbaseHex: "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff2f021627062f503253482f040b96ab5208f80001e101000000162f646f67652e6c75636b796d696e6572732e636f6d2f000000000100e055e4dd0700001976a914ef948da5c19f77b94e09d474492cec6cf656971188ac00000000",
	},
}

func buildMainnetCanonicalBlockRaw(spec mainnetCanonicalBlockSpec) ([]byte, error) {
	if spec.FullHex != "" {
		raw, err := hex.DecodeString(spec.FullHex)
		if err != nil {
			return nil, err
		}
		if len(raw) < 80 {
			return nil, fmt.Errorf("height %d full hex too short", spec.Height)
		}
		got := pow.BlockHashHex(raw[:80])
		if got != spec.WantHash {
			return nil, fmt.Errorf("height %d hash %s want %s", spec.Height, got, spec.WantHash)
		}
		return raw, nil
	}
	prev, err := chain.Hash256FromDisplayHex(spec.PrevHash)
	if err != nil {
		return nil, err
	}
	merkle, err := chain.Hash256FromDisplayHex(spec.MerkleRoot)
	if err != nil {
		return nil, err
	}
	tx, err := hex.DecodeString(spec.CoinbaseHex)
	if err != nil {
		return nil, err
	}
	hdr := make([]byte, 80)
	binary.LittleEndian.PutUint32(hdr[0:4], 1)
	copy(hdr[4:36], prev[:])
	copy(hdr[36:68], merkle[:])
	binary.LittleEndian.PutUint32(hdr[68:72], spec.Time)
	binary.LittleEndian.PutUint32(hdr[72:76], spec.Bits)
	binary.LittleEndian.PutUint32(hdr[76:80], spec.Nonce)
	raw := append(hdr, 0x01)
	raw = append(raw, tx...)
	got := pow.BlockHashHex(hdr)
	if got != spec.WantHash {
		return nil, fmt.Errorf("height %d hash %s want %s", spec.Height, got, spec.WantHash)
	}
	return raw, nil
}

func mainnetCanonicalFieldBlocks() ([]mainnetFieldBlockEntry, error) {
	out := make([]mainnetFieldBlockEntry, 0, len(mainnetCanonicalBlockSpecs))
	for _, spec := range mainnetCanonicalBlockSpecs {
		raw, err := buildMainnetCanonicalBlockRaw(spec)
		if err != nil {
			return nil, err
		}
		out = append(out, mainnetFieldBlockEntry{
			Height: spec.Height,
			Hex:    hex.EncodeToString(raw),
		})
	}
	return out, nil
}

func catalogMainnetFieldBlockPayloadVectors() ([]coreBlockVector, error) {
	blocks, err := mainnetCanonicalFieldBlocks()
	if err != nil {
		return nil, err
	}
	out := make([]coreBlockVector, 0, len(blocks))
	for _, e := range blocks {
		if e.Height <= 0 {
			continue
		}
		out = append(out, coreBlockVector{
			Name:       fmt.Sprintf("mainnet_field_block_%d_payload_accept", e.Height),
			Kind:       "check_block_payload",
			Network:    "mainnet",
			Height:     e.Height,
			Source:     "hex",
			Hex:        strings.ToUpper(e.Hex),
			WantAccept: true,
		})
	}
	return out, nil
}
