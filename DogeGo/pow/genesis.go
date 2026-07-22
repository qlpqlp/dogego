// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pow

import (
	"encoding/binary"

	"dogego/chain"
)

// Header80FromParams builds the 80-byte genesis header for the given chain params (no auxpow).
func Header80FromParams(p chain.Params) ([80]byte, error) {
	var h [80]byte
	binary.LittleEndian.PutUint32(h[0:4], uint32(p.GenesisVer))
	merkle, err := chain.Hash256FromDisplayHex(p.GenesisMerkleRootHex)
	if err != nil {
		return h, err
	}
	copy(h[36:68], merkle[:])
	binary.LittleEndian.PutUint32(h[68:72], p.GenesisTime)
	binary.LittleEndian.PutUint32(h[72:76], p.GenesisBits)
	binary.LittleEndian.PutUint32(h[76:80], p.GenesisNonce)
	return h, nil
}
