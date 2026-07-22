// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package chain holds constants for the rebooted -testnet in this repo.
package chain

// Rebooted testnet (see src/chainparams.cpp CTestNetParams).
// From block 1: Digishield + 10k DOGE tail subsidy + PR #3967 strict min-difficulty; auxpow from height 158100.
var (
	Magic = [4]byte{0xfd, 0xd4, 0xdc, 0xe1}
	Port  = 44556

	// Genesis header (CreateGenesisBlock: time, nonce, bits; same merkle as legacy testnet coinbase).
	GenesisTime  uint32 = 1747000000
	GenesisNonce uint32 = 2139303
	GenesisBits  uint32 = 0x1e0ffff0
	GenesisVer   int32  = 1

	// Display-order hex (block explorer style); 32-byte LE in wire is byte-reverse of this hex.
	GenesisMerkleRootHex = "5b2a3f53f605d62c53e62932dac6925e3d74afa5a4b459745c36d42d0ed26a69"
	GenesisBlockHashHex  = "d5d619f8be025d4700940883c86f271d08cffa8dd1d3d4afa474c9ed9e8b68a0"

	ProtocolVersion  int32  = 70015
	InitProtoVersion int32  = 209
	MinProtoVersion  int32  = 70003
	NodeNetwork      uint64 = 1 // NODE_NETWORK
)
