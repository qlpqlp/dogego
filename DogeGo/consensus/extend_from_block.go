// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// ExtendHeadersFromTipBlock appends one header when the block builds on the journal tip and
// passes ValidateHeaders (PoW, difficulty, auxpow, MTP).
func ExtendHeadersFromTipBlock(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, pb *wire.ParsedBlock, nowUnix int64) (int64, error) {
	tip, err := j.TipHeight()
	if err != nil {
		return -1, err
	}
	return ExtendHeadersFromParentHeight(j, aux, p, pb, tip, nowUnix)
}

// ExtendHeadersFromPayload appends one header from serialized block bytes (no full ParseBlock).
func ExtendHeadersFromPayload(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, blockRaw []byte, parentHeight int64, nowUnix int64) (int64, error) {
	hdr, auxPow, err := wire.BlockHeaderAuxFromPayload(blockRaw)
	if err != nil {
		return -1, err
	}
	return ExtendHeadersFromParentHeight(j, aux, p, &wire.ParsedBlock{Header: hdr, Aux: auxPow}, parentHeight, nowUnix)
}

// ExtendHeadersFromParentHeight appends one header when the block builds on parentHeight (Core chainActive tip).
func ExtendHeadersFromParentHeight(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, pb *wire.ParsedBlock, parentHeight int64, nowUnix int64) (int64, error) {
	if j == nil || pb == nil {
		return -1, fmt.Errorf("no header journal")
	}
	if parentHeight < 0 {
		return -1, fmt.Errorf("invalid parent height %d", parentHeight)
	}
	if nowUnix <= 0 {
		nowUnix = time.Now().Unix()
	}
	h80 := pb.Header.EncodeWire80()
	var prev [32]byte
	copy(prev[:], h80[4:36])
	parent80, err := j.ReadHeaderAt(parentHeight)
	if err != nil || len(parent80) != 80 {
		return -1, fmt.Errorf("parent header read failed at height %d", parentHeight)
	}
	wantParent := pow.BlockHashLE(parent80)
	if prev != wantParent {
		return -1, fmt.Errorf("block parent does not extend height %d", parentHeight)
	}
	decoded := []wire.DecodedHeader{{
		Header80: append([]byte(nil), h80[:]...),
		Aux:      pb.Aux,
	}}
	if err := ValidateHeaders(j, p, decoded, nowUnix); err != nil {
		return -1, fmt.Errorf("header extend: %w", err)
	}
	verLE := nVersionLE(h80[:])
	if err := requireAuxJournalForExtend(verLE, aux); err != nil {
		return -1, err
	}
	if err := j.AppendHeaders([][]byte{decoded[0].Header80}); err != nil {
		return -1, err
	}
	if isAuxpowVersionU(verLE) {
		blob, err := wire.SerializeAuxPow(pb.Aux)
		if err != nil {
			return -1, fmt.Errorf("aux serialize: %w", err)
		}
		if err := aux.AppendEntries([][]byte{blob}); err != nil {
			return -1, fmt.Errorf("aux journal: %w", err)
		}
	}
	height, err := j.TipHeight()
	if err != nil {
		return -1, err
	}
	applog.Line("headers", fmt.Sprintf("extended header chain to height %d (submitblock/block)", height))
	return height, nil
}

func requireAuxJournalForExtend(verLE uint32, aux *store.HeaderAuxJournal) error {
	if !isAuxpowVersionU(verLE) {
		return nil
	}
	if aux == nil {
		return fmt.Errorf("headers_aux.bin required for auxpow header extend")
	}
	return nil
}
