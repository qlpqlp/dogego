// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package bloom

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"dogego/wire"
)

// ParseFilterLoad decodes a BIP37 filterload payload.
func ParseFilterLoad(payload []byte) (*Filter, error) {
	r := bytes.NewReader(payload)
	n, err := wire.ReadCompactSize(r)
	if err != nil {
		return nil, fmt.Errorf("filterload: %w", err)
	}
	if n > MaxFilterSize {
		return nil, fmt.Errorf("filterload: size %d exceeds max", n)
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("filterload data: %w", err)
	}
	var nHashFuncs, nTweak uint32
	var nFlags uint8
	if err := binary.Read(r, binary.LittleEndian, &nHashFuncs); err != nil {
		return nil, fmt.Errorf("filterload nHashFuncs: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &nTweak); err != nil {
		return nil, fmt.Errorf("filterload nTweak: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &nFlags); err != nil {
		return nil, fmt.Errorf("filterload nFlags: %w", err)
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("filterload: trailing %d bytes", r.Len())
	}
	return NewFromWire(data, nHashFuncs, nTweak, nFlags)
}

// EncodeFilterAdd builds a BIP37 filteradd payload.
func EncodeFilterAdd(data []byte) []byte {
	var out []byte
	out = appendCompactSize(out, uint64(len(data)))
	return append(out, data...)
}

// ParseFilterAdd returns the data bytes from a filteradd payload.
func ParseFilterAdd(payload []byte) ([]byte, error) {
	r := bytes.NewReader(payload)
	n, err := wire.ReadCompactSize(r)
	if err != nil {
		return nil, fmt.Errorf("filteradd: %w", err)
	}
	if n > MaxFilterSize {
		return nil, fmt.Errorf("filteradd: size %d exceeds max", n)
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("filteradd data: %w", err)
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("filteradd: trailing %d bytes", r.Len())
	}
	return data, nil
}

// MatchRelevantTx reports whether tx matches the bloom filter (Core IsRelevantAndUpdate subset without P2PUBKEY script classification).
// When update flags request it, matched outpoints are inserted into the filter.
func MatchRelevantTx(f *Filter, tx *wire.Tx) bool {
	if f == nil || tx == nil {
		return false
	}
	found := false
	txid := tx.TxHash()
	if f.Contains(txid[:]) {
		found = true
	}
	for i := range tx.Vout {
		if f.Contains(tx.Vout[i].PkScript) {
			found = true
			switch f.nFlags & 3 {
			case UpdateAll:
				f.InsertOutpoint(txid, uint32(i))
			case UpdateP2PubkeyOnly:
				if isProbablyP2PKOrMulti(tx.Vout[i].PkScript) {
					f.InsertOutpoint(txid, uint32(i))
				}
			}
		}
	}
	for i := range tx.Vin {
		if f.ContainsOutpoint(tx.Vin[i].PrevHash, tx.Vin[i].PrevIdx) {
			found = true
		}
		if len(tx.Vin[i].Script) > 0 && f.Contains(tx.Vin[i].Script) {
			found = true
		}
	}
	return found
}

// isProbablyP2PKOrMulti is a lightweight check for BLOOM_UPDATE_P2PUBKEY_ONLY
// (push pubkey OP_CHECKSIG, or bare multisig ending in OP_CHECKMULTISIG).
func isProbablyP2PKOrMulti(script []byte) bool {
	if len(script) == 0 {
		return false
	}
	last := script[len(script)-1]
	if last == 0xac { // OP_CHECKSIG
		return true
	}
	if last == 0xae { // OP_CHECKMULTISIG
		return true
	}
	return false
}
