// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"crypto/sha256"

	"dogego/wire"
)

// BlockFilterHash is double-SHA256 of the encoded filter (Core BlockFilter::GetHash).
func BlockFilterHash(encoded []byte) [32]byte {
	h := sha256.Sum256(encoded)
	return sha256.Sum256(h[:])
}

// BlockFilterHeader is double-SHA256(filterHash || prevHeader) (Core BlockFilter::ComputeHeader).
func BlockFilterHeader(filterHash, prevHeader [32]byte) [32]byte {
	var b [64]byte
	copy(b[:32], filterHash[:])
	copy(b[32:], prevHeader[:])
	h := sha256.Sum256(b[:])
	return sha256.Sum256(h[:])
}

func appendBasicFilterOutputScripts(tx *wire.Tx, out *[][]byte) {
	if tx == nil {
		return
	}
	for _, o := range tx.Vout {
		if len(o.PkScript) == 0 || o.PkScript[0] == 0x6a {
			continue
		}
		*out = append(*out, append([]byte(nil), o.PkScript...))
	}
}

// CollectBasicFilterOutputScripts returns non-OP_RETURN output scriptPubKeys from a block.
func CollectBasicFilterOutputScripts(pb *wire.ParsedBlock) [][]byte {
	if pb == nil {
		return nil
	}
	var out [][]byte
	for _, tx := range pb.Txs {
		appendBasicFilterOutputScripts(tx, &out)
	}
	return out
}

// CollectBasicFilterOutputScriptsRaw scans block payload without retaining all txs.
func CollectBasicFilterOutputScriptsRaw(blockRaw []byte) ([][]byte, error) {
	if len(blockRaw) < 80 {
		return nil, nil
	}
	var out [][]byte
	err := wire.ForEachBlockTx(blockRaw, func(_ uint32, tx *wire.Tx) error {
		appendBasicFilterOutputScripts(tx, &out)
		return nil
	})
	return out, err
}
