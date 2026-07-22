// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"fmt"

	"dogego/primitives"
)

// BuildHeaderAndShortIDsFromBlock builds a BIP152 cmpctblock body from a full block payload.
// Coinbase (index 0) is always prefilled; other txs are sent as short IDs.
// Returns an error for auxpow blocks (cmpct header is 80 bytes only; aux must come from elsewhere).
func BuildHeaderAndShortIDsFromBlock(raw []byte, nonce uint64) (*HeaderAndShortIDs, error) {
	if len(raw) < 81 {
		return nil, fmt.Errorf("block too short")
	}
	var hdr [80]byte
	copy(hdr[:], raw[:80])
	var h primitives.BlockHeader
	if err := h.DecodeWire80(hdr[:]); err != nil {
		return nil, err
	}
	if isAuxPowVersion(h.Version) {
		return nil, fmt.Errorf("cmpct: auxpow blocks need full block relay")
	}
	offsets, err := BlockTxDiskOffsets(raw)
	if err != nil {
		return nil, err
	}
	if len(offsets) == 0 {
		return nil, fmt.Errorf("block has no txs")
	}
	coinEnd := len(raw)
	if len(offsets) > 1 {
		coinEnd = int(offsets[1])
	}
	coinRaw := append([]byte(nil), raw[offsets[0]:coinEnd]...)
	out := &HeaderAndShortIDs{
		Header80: hdr,
		Nonce:    nonce,
		Prefilled: []PrefilledTransaction{
			{Index: 0, Tx: coinRaw},
		},
	}
	for i := 1; i < len(offsets); i++ {
		start := int(offsets[i])
		end := len(raw)
		if i+1 < len(offsets) {
			end = int(offsets[i+1])
		}
		txRaw := raw[start:end]
		tx, err := DeserializeTx(txRaw)
		if err != nil {
			return nil, fmt.Errorf("tx %d: %w", i, err)
		}
		sid := CmpctShortTxID(hdr[:], nonce, tx.TxHash())
		out.ShortIDs = append(out.ShortIDs, sid)
	}
	return out, nil
}

// ReconstructBlockFromCmpct rebuilds a full block payload from HeaderAndShortIDs.
// shortIDToTx maps short IDs to wire tx bytes; missing indexes must be listed in extraByIndex.
func ReconstructBlockFromCmpct(h *HeaderAndShortIDs, shortIDToTx map[uint64][]byte, extraByIndex map[uint64][]byte) ([]byte, error) {
	if h == nil {
		return nil, fmt.Errorf("nil cmpct block")
	}
	nTx := CmpctBlockTxCount(h)
	if nTx == 0 {
		return nil, fmt.Errorf("cmpct block has no txs")
	}
	pref := make(map[uint64][]byte, len(h.Prefilled))
	for _, pf := range h.Prefilled {
		if pf.Index >= uint64(nTx) {
			return nil, fmt.Errorf("prefilled index %d out of range", pf.Index)
		}
		if _, dup := pref[pf.Index]; dup {
			return nil, fmt.Errorf("duplicate prefilled index %d", pf.Index)
		}
		pref[pf.Index] = pf.Tx
	}
	txs := make([][]byte, nTx)
	shortI := 0
	for i := 0; i < nTx; i++ {
		if raw, ok := pref[uint64(i)]; ok {
			txs[i] = raw
			continue
		}
		if shortI >= len(h.ShortIDs) {
			return nil, fmt.Errorf("cmpct short id underflow at tx %d", i)
		}
		sid := h.ShortIDs[shortI]
		shortI++
		if raw, ok := shortIDToTx[sid]; ok {
			txs[i] = raw
			continue
		}
		if extraByIndex != nil {
			if raw, ok := extraByIndex[uint64(i)]; ok {
				txs[i] = raw
				continue
			}
		}
		return nil, fmt.Errorf("missing tx for index %d shortid %x", i, sid)
	}
	if shortI != len(h.ShortIDs) {
		return nil, fmt.Errorf("cmpct short id overflow")
	}
	var aux []byte
	var hdr primitives.BlockHeader
	_ = hdr.DecodeWire80(h.Header80[:])
	if isAuxPowVersion(hdr.Version) {
		return nil, fmt.Errorf("cmpct reconstruct: auxpow header without aux blob")
	}
	return SerializeBlockFromTxRaws(h.Header80, aux, txs)
}

// MissingCmpctIndexes returns block tx indexes not covered by prefilled or shortIDToTx matches.
func MissingCmpctIndexes(h *HeaderAndShortIDs, shortIDToTx map[uint64][]byte) []uint64 {
	if h == nil {
		return nil
	}
	nTx := CmpctBlockTxCount(h)
	pref := make(map[uint64]struct{}, len(h.Prefilled))
	for _, pf := range h.Prefilled {
		pref[pf.Index] = struct{}{}
	}
	var missing []uint64
	shortI := 0
	for i := 0; i < nTx; i++ {
		if _, ok := pref[uint64(i)]; ok {
			continue
		}
		if shortI >= len(h.ShortIDs) {
			break
		}
		sid := h.ShortIDs[shortI]
		shortI++
		if _, ok := shortIDToTx[sid]; ok {
			continue
		}
		missing = append(missing, uint64(i))
	}
	return missing
}

// MatchCmpctShortIDsFromMempool maps short IDs to mempool tx bytes (ambiguous matches skipped).
func MatchCmpctShortIDsFromMempool(h *HeaderAndShortIDs, mempoolTxs [][]byte) map[uint64][]byte {
	if h == nil || len(h.ShortIDs) == 0 {
		return nil
	}
	hdr := h.Header80
	nonce := h.Nonce
	want := make(map[uint64]struct{}, len(h.ShortIDs))
	for _, sid := range h.ShortIDs {
		want[sid] = struct{}{}
	}
	candidates := make(map[uint64][][]byte)
	for _, raw := range mempoolTxs {
		tx, err := DeserializeTx(raw)
		if err != nil {
			continue
		}
		sid := CmpctShortTxID(hdr[:], nonce, tx.TxHash())
		if _, ok := want[sid]; !ok {
			continue
		}
		candidates[sid] = append(candidates[sid], raw)
	}
	out := make(map[uint64][]byte)
	for sid, list := range candidates {
		if len(list) == 1 {
			out[sid] = list[0]
		}
	}
	return out
}
