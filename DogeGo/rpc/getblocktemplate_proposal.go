// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// validateGBTProposal checks a hex block proposal against the current tip (Core getblocktemplate proposal mode / BIP22).
// Returns nil on accept, or a rejection string (RPC result) on failure.
func validateGBTProposal(j HeaderJournal, pool *mempool.Pool, txIndex *store.TxIndex, rawBlocks *store.RawBlockStore, paths *DataPaths, chainName string, blockMaxWeight int, req map[string]interface{}, hexData string) interface{} {
	hexData = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexData), "0x"))
	if len(hexData)%2 != 0 {
		return "rejected: block decode failed"
	}
	payload, err := hex.DecodeString(hexData)
	if err != nil {
		return "rejected: block decode failed"
	}
	if len(payload) < 81 {
		return "rejected: block too short"
	}
	want := pow.BlockHashLE(payload[:80])
	display := pow.BlockHashHex(payload[:80])
	if err := wire.ValidateBlockPayload(payload, want); err != nil {
		return "rejected: " + err.Error()
	}
	hdr, err := wire.BlockHeaderFromPayload(payload)
	if err != nil {
		return "rejected: " + err.Error()
	}
	if j != nil {
		if _, err := j.HeightByDisplayHash(display); err == nil {
			return "duplicate"
		}
	}
	if rawBlocks != nil && rawBlocks.Has(want) {
		return "duplicate"
	}
	tip, _, _ := activeChainFromJournal(j, rawBlocks, paths)
	next := tip + 1
	h80, err := j.ReadHeaderAt(tip)
	if err != nil || len(h80) != 80 {
		return "rejected: header journal read failed"
	}
	wantPrev := pow.BlockHashLE(h80[:])
	if !bytes.Equal(hdr.PrevBlock[:], wantPrev[:]) {
		return "inconclusive-not-best-prevblk"
	}
	net, _ := chain.ParseNetwork(chainName)
	if err := consensus.CheckBlockPayload(payload, want, next, net); err != nil {
		return bip22Reject(err)
	}
	if blockMaxWeight <= 0 {
		blockMaxWeight = consensus.MaxBlockWeight
	}
	w, err := consensus.BlockWeightRaw(payload)
	if err != nil {
		return bip22Reject(err)
	}
	if w > blockMaxWeight {
		return fmt.Sprintf("bad-blk-weight, weight %d > limit %d", w, blockMaxWeight)
	}
	view := consensus.AdmissionPrevOutView(pool, txIndex, rawBlocks)
	var index consensus.TxIndexer
	if txIndex != nil {
		index = txIndex
	}
	hj, _ := j.(*store.HeaderJournal)
	if err := consensus.ConnectBlockRaw(payload, hdr, next, net, view, index, hj); err != nil {
		return bip22Reject(err)
	}
	if rules, ok := req["rules"].([]interface{}); ok {
		for _, r := range rules {
			if s, ok := r.(string); ok && strings.EqualFold(s, "segwit") {
				return "bad-witness-disabled"
			}
		}
	}
	return nil
}

func bip22Reject(err error) string {
	if err == nil {
		return "rejected"
	}
	msg := err.Error()
	if msg == "" {
		return "rejected"
	}
	return msg
}
