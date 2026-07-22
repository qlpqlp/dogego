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
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/wire"
)

// BuildRelaxedLegacyBlockPayload mines a legacy block (RelaxedPoW) with coinbase + optional extra txs.
// prevHeader80 is the 80-byte tip header; height is the new block height.
func BuildRelaxedLegacyBlockPayload(prevHeader80 []byte, height int64, net chain.Network, coinbaseH160 [20]byte, extraTxs []*wire.Tx) (displayHash string, payload []byte, err error) {
	if len(prevHeader80) != 80 {
		return "", nil, fmt.Errorf("relaxed block: prev header must be 80 bytes")
	}
	if height < 1 {
		return "", nil, fmt.Errorf("relaxed block: invalid height")
	}
	dc := LookupConsensus(net, height)
	if !dc.AllowLegacyBlocks {
		return "", nil, fmt.Errorf("height %d requires merge-mined auxpow blocks", height)
	}
	prevLE := pow.BlockHashLE(prevHeader80)
	subsidy := BlockSubsidy(height, prevLE, net)
	coin := BuildCoinbaseTx(height, subsidy, P2PKHPkScript(coinbaseH160))
	txs := make([]*wire.Tx, 0, 1+len(extraTxs))
	txs = append(txs, coin)
	txs = append(txs, extraTxs...)
	merkle := wire.BlockMerkleRoot(txs)
	var h80 [80]byte
	binary.LittleEndian.PutUint32(h80[0:4], 1)
	copy(h80[4:36], prevLE[:])
	copy(h80[36:68], merkle[:])
	blockTime := uint32(time.Now().Unix())
	if blockTime <= binary.LittleEndian.Uint32(prevHeader80[68:72]) {
		blockTime = binary.LittleEndian.Uint32(prevHeader80[68:72]) + 1
	}
	binary.LittleEndian.PutUint32(h80[68:72], blockTime)
	bits := pow.DogePowLimitCompact()
	binary.LittleEndian.PutUint32(h80[72:76], bits)
	binary.LittleEndian.PutUint32(h80[76:80], 0)
	var buf []byte
	buf = append(buf, h80[:]...)
	buf = append(buf, byte(len(txs)))
	for _, tx := range txs {
		raw, err := tx.Serialize()
		if err != nil {
			return "", nil, err
		}
		buf = append(buf, raw...)
	}
	return pow.BlockHashHex(h80[:]), buf, nil
}

// AttachSubmitBlockPrep builds a submitblock payload for templates whose prep tx cannot enter mempool (P2PK funding).
func AttachSubmitBlockPrep(probe *StatefulLiveProbe, prevHeader80 []byte, height int64, net chain.Network, coinbaseH160 [20]byte) error {
	if probe == nil || probe.Template != "p2pk_non_standard_input" || len(probe.PrepTxHex) == 0 {
		return nil
	}
	anchorRaw, err := hex.DecodeString(probe.PrepTxHex[0])
	if err != nil {
		return err
	}
	anchor, err := wire.DeserializeTx(anchorRaw)
	if err != nil {
		return err
	}
	_, payload, err := BuildRelaxedLegacyBlockPayload(prevHeader80, height, net, coinbaseH160, []*wire.Tx{anchor})
	if err != nil {
		return err
	}
	probe.PrepSubmitBlockHex = hex.EncodeToString(payload)
	return nil
}
