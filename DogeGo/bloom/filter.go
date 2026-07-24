// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package bloom implements Bitcoin Core / BIP37 CBloomFilter (murmur3 + wire encode).
package bloom

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Core protocol.h / bloom.h limits.
const (
	MaxFilterSize = 36000 // MAX_BLOOM_FILTER_SIZE
	MaxHashFuncs  = 50    // MAX_HASH_FUNCS
)

// Update flags (nFlags).
const (
	UpdateNone         = 0 // BLOOM_UPDATE_NONE
	UpdateAll          = 1 // BLOOM_UPDATE_ALL
	UpdateP2PubkeyOnly = 2 // BLOOM_UPDATE_P2PUBKEY_ONLY
)

// Filter is a Core-compatible BIP37 bloom filter.
type Filter struct {
	vData     []byte
	nHashFuncs uint32
	nTweak    uint32
	nFlags    uint8
	empty     bool // true until first insert after construction from NewEmpty
}

// NewEmpty builds a filter sized for nElements at the given false-positive rate.
func NewEmpty(nElements uint32, fpRate float64, nTweak uint32, nFlags uint8) (*Filter, error) {
	if nElements == 0 {
		nElements = 1
	}
	if fpRate <= 0 || fpRate >= 1 {
		fpRate = 0.0001
	}
	nFilterBytes := uint32(-1.0 / math.Ln2 / math.Ln2 * float64(nElements) * math.Log(fpRate) / 8)
	if nFilterBytes > MaxFilterSize {
		nFilterBytes = MaxFilterSize
	}
	if nFilterBytes < 1 {
		nFilterBytes = 1
	}
	nHashFuncs := uint32(float64(nFilterBytes*8) / float64(nElements) * math.Ln2)
	if nHashFuncs > MaxHashFuncs {
		nHashFuncs = MaxHashFuncs
	}
	if nHashFuncs < 1 {
		nHashFuncs = 1
	}
	return &Filter{
		vData:      make([]byte, nFilterBytes),
		nHashFuncs: nHashFuncs,
		nTweak:     nTweak,
		nFlags:     nFlags,
		empty:      true,
	}, nil
}

// NewFromWire builds a filter from deserialized filterload fields.
func NewFromWire(vData []byte, nHashFuncs, nTweak uint32, nFlags uint8) (*Filter, error) {
	if len(vData) > MaxFilterSize {
		return nil, fmt.Errorf("bloom: filter size %d exceeds max %d", len(vData), MaxFilterSize)
	}
	if nHashFuncs > MaxHashFuncs {
		return nil, fmt.Errorf("bloom: nHashFuncs %d exceeds max %d", nHashFuncs, MaxHashFuncs)
	}
	data := append([]byte(nil), vData...)
	empty := true
	for _, b := range data {
		if b != 0 {
			empty = false
			break
		}
	}
	return &Filter{
		vData:      data,
		nHashFuncs: nHashFuncs,
		nTweak:     nTweak,
		nFlags:     nFlags,
		empty:      empty,
	}, nil
}

// Flags returns nFlags.
func (f *Filter) Flags() uint8 {
	if f == nil {
		return 0
	}
	return f.nFlags
}

// IsEmpty reports whether the filter has never had a bit set (or is nil).
func (f *Filter) IsEmpty() bool {
	return f == nil || f.empty
}

func (f *Filter) hash(nHashNum uint32, data []byte) uint32 {
	h := MurmurHash3(nHashNum*0xFBA4C795+f.nTweak, data)
	return h % (uint32(len(f.vData)) * 8)
}

// Insert adds data to the filter.
func (f *Filter) Insert(data []byte) {
	if f == nil || len(f.vData) == 0 {
		return
	}
	for i := uint32(0); i < f.nHashFuncs; i++ {
		bit := f.hash(i, data)
		f.vData[bit/8] |= 1 << (bit % 8)
	}
	f.empty = false
}

// Contains reports whether data may be in the filter.
func (f *Filter) Contains(data []byte) bool {
	if f == nil || len(f.vData) == 0 {
		return false
	}
	if f.empty {
		return false
	}
	for i := uint32(0); i < f.nHashFuncs; i++ {
		bit := f.hash(i, data)
		if f.vData[bit/8]&(1<<(bit%8)) == 0 {
			return false
		}
	}
	return true
}

// InsertOutpoint inserts a serialized COutPoint (txid LE + index LE).
func (f *Filter) InsertOutpoint(txid [32]byte, index uint32) {
	var buf [36]byte
	copy(buf[:32], txid[:])
	binary.LittleEndian.PutUint32(buf[32:], index)
	f.Insert(buf[:])
}

// ContainsOutpoint reports a possible outpoint match.
func (f *Filter) ContainsOutpoint(txid [32]byte, index uint32) bool {
	var buf [36]byte
	copy(buf[:32], txid[:])
	binary.LittleEndian.PutUint32(buf[32:], index)
	return f.Contains(buf[:])
}

// EncodeWire returns filterload payload: compact filter + nHashFuncs + nTweak + nFlags.
func (f *Filter) EncodeWire() ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("bloom: nil filter")
	}
	var out []byte
	out = appendCompactSize(out, uint64(len(f.vData)))
	out = append(out, f.vData...)
	var tmp [9]byte
	binary.LittleEndian.PutUint32(tmp[0:4], f.nHashFuncs)
	binary.LittleEndian.PutUint32(tmp[4:8], f.nTweak)
	tmp[8] = f.nFlags
	out = append(out, tmp[:]...)
	return out, nil
}

func appendCompactSize(b []byte, n uint64) []byte {
	switch {
	case n < 253:
		return append(b, byte(n))
	case n <= 0xffff:
		var tmp [3]byte
		tmp[0] = 253
		binary.LittleEndian.PutUint16(tmp[1:], uint16(n))
		return append(b, tmp[:]...)
	case n <= 0xffffffff:
		var tmp [5]byte
		tmp[0] = 254
		binary.LittleEndian.PutUint32(tmp[1:], uint32(n))
		return append(b, tmp[:]...)
	default:
		var tmp [9]byte
		tmp[0] = 255
		binary.LittleEndian.PutUint64(tmp[1:], n)
		return append(b, tmp[:]...)
	}
}
