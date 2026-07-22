// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"strings"

	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

// execGetTxOutProof returns hex-encoded CMerkleBlock (Core gettxoutproof) for txids in one block.
// Optional second param is block hash hex. Uses 80-byte non-auxpow header from stored raw block (first 80 bytes).
func execGetTxOutProof(ix *store.TxIndex, raw *store.RawBlockStore, j HeaderJournal, params []json.RawMessage) (interface{}, int, string) {
	if ix == nil || raw == nil {
		return nil, -18, "gettxoutproof: transaction index and raw blocks are required"
	}
	if len(params) < 1 {
		return nil, -8, "gettxoutproof: txids array required"
	}
	var txids []string
	if err := json.Unmarshal(params[0], &txids); err != nil {
		return nil, -8, "gettxoutproof: expected JSON array of txid strings"
	}
	if len(txids) == 0 {
		return nil, -8, "gettxoutproof: txids array is empty"
	}
	seen := make(map[string]struct{})
	norm := make([]string, 0, len(txids))
	for _, id := range txids {
		s := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(id), "0x"))
		if len(s) != 64 {
			return nil, -8, "gettxoutproof: invalid txid " + id
		}
		for _, c := range s {
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
				continue
			}
			return nil, -8, "gettxoutproof: invalid txid " + id
		}
		if _, ok := seen[s]; ok {
			return nil, -8, "gettxoutproof: duplicated txid " + id
		}
		seen[s] = struct{}{}
		norm = append(norm, s)
	}

	var payload []byte
	var err error
	if len(params) >= 2 {
		var blockHashStr string
		if err := json.Unmarshal(params[1], &blockHashStr); err != nil {
			return nil, -8, "gettxoutproof: bad blockhash"
		}
		blockHashStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(blockHashStr), "0x"))
		if len(blockHashStr) != 64 {
			return nil, -8, "gettxoutproof: blockhash must be 64 hex characters"
		}
		h, err := j.HeightByDisplayHash(blockHashStr)
		if err != nil {
			return nil, -5, "gettxoutproof: block not found"
		}
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return nil, -5, err.Error()
		}
		bhLE := pow.BlockHashLE(h80)
		payload, err = raw.Get(bhLE)
		if err != nil {
			return nil, -5, "gettxoutproof: block not in local raw store"
		}
	} else {
		blockHashLE, _, err := ix.Lookup(norm[0])
		if err != nil {
			return nil, -5, "gettxoutproof: transaction not in local index"
		}
		payload, err = raw.Get(blockHashLE)
		if err != nil {
			return nil, -5, "gettxoutproof: block payload missing"
		}
	}

	hdr, err := wire.BlockHeaderFromPayload(payload)
	if err != nil {
		return nil, -8, "gettxoutproof: corrupt block: " + err.Error()
	}
	wantLeft := make(map[string]struct{}, len(norm))
	for _, w := range norm {
		wantLeft[w] = struct{}{}
	}
	var vTxid [][32]byte
	var vMatch []bool
	if err := wire.ForEachBlockTx(payload, func(_ uint32, tx *wire.Tx) error {
		h := tx.TxHash()
		vTxid = append(vTxid, h)
		rpc := strings.ToLower(txidToRPC(h))
		if _, ok := wantLeft[rpc]; ok {
			vMatch = append(vMatch, true)
			delete(wantLeft, rpc)
		} else {
			vMatch = append(vMatch, false)
		}
		return nil
	}); err != nil {
		return nil, -8, "gettxoutproof: corrupt block: " + err.Error()
	}
	if len(wantLeft) > 0 {
		return nil, -5, "(Not all) transactions not found in specified block"
	}

	pmt, err := wire.NewPartialMerkleTree(vTxid, vMatch)
	if err != nil {
		return nil, -8, err.Error()
	}
	h80 := hdr.EncodeWire80()
	proof, err := wire.SerializeMerkleBlock(h80[:], pmt)
	if err != nil {
		return nil, -8, err.Error()
	}
	return hex.EncodeToString(proof), 0, ""
}

// execVerifyTxOutProof decodes a CMerkleBlock proof; returns [] of RPC txids on success, empty [] if proof is invalid.
// Returns RPC error if the proof is valid but the block header is not on the active header chain.
func execVerifyTxOutProof(j HeaderJournal, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 {
		return nil, -8, "verifytxoutproof: proof hex required"
	}
	var hexStr string
	if err := json.Unmarshal(params[0], &hexStr); err != nil {
		return nil, -8, "verifytxoutproof: bad hex param"
	}
	hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	data, err := hex.DecodeString(hexStr)
	if err != nil || len(data) < 84 {
		return []interface{}{}, 0, ""
	}
	header80, pmt, err := wire.ParseMerkleBlockProof(data)
	if err != nil {
		return []interface{}{}, 0, ""
	}
	var hdr primitives.BlockHeader
	if err := hdr.DecodeWire80(header80); err != nil {
		return []interface{}{}, 0, ""
	}
	root, matches, _, ok := pmt.ExtractMatches()
	if !ok || root != hdr.MerkleRoot {
		return []interface{}{}, 0, ""
	}

	blockID := pow.BlockHashLE(header80)
	height, err := j.HeightByDisplayHash(pow.LEUint256DisplayHex(blockID[:]))
	if err != nil {
		return nil, -5, "verifytxoutproof: block not found in chain"
	}
	stored, err := j.ReadHeaderAt(height)
	if err != nil || !bytes.Equal(stored, header80) {
		return nil, -5, "verifytxoutproof: block not found in chain"
	}

	out := make([]interface{}, len(matches))
	for i, th := range matches {
		out[i] = txidToRPC(th)
	}
	return out, 0, ""
}
