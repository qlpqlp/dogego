// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sort"

	"dogego/wire"
)

// DecodeBasicFilterHashed returns sorted Golomb-Rice hashed keys from a BIP158 basic filter.
func DecodeBasicFilterHashed(encoded []byte) ([]uint64, error) {
	if len(encoded) == 0 {
		return nil, nil
	}
	r := bytes.NewReader(encoded)
	n, err := wire.ReadCompactSize(r)
	if err != nil {
		return nil, err
	}
	bitData, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	br := newBitReader(bitData)
	keys := make([]uint64, 0, n)
	var last uint64
	for i := uint64(0); i < n; i++ {
		delta, err := br.readGolombRice(BasicFilterP)
		if err != nil {
			return nil, err
		}
		v := delta
		if i > 0 {
			v = last + delta
		}
		keys = append(keys, v)
		last = v
	}
	return keys, nil
}

// BasicFilterMayContainScript reports whether script may appear in the basic filter (BIP158; false positives possible).
func BasicFilterMayContainScript(blockHashLE [32]byte, encoded []byte, script []byte) (bool, error) {
	if len(script) == 0 || (len(script) > 0 && script[0] == 0x6a) {
		return false, nil
	}
	keys, err := DecodeBasicFilterHashed(encoded)
	if err != nil || len(keys) == 0 {
		return false, err
	}
	k0 := binary.LittleEndian.Uint64(blockHashLE[0:8])
	k1 := binary.LittleEndian.Uint64(blockHashLE[8:16])
	h := SipHash24(k0, k1, script)
	target := fastRange64(h, uint64(len(keys))*BasicFilterM)
	i := sort.Search(len(keys), func(i int) bool { return keys[i] >= target })
	return i < len(keys) && keys[i] == target, nil
}

type bitReader struct {
	data  []byte
	pos   int
	bit   uint8
	avail int // bits left in current byte (0 = load next)
}

func newBitReader(data []byte) *bitReader {
	return &bitReader{data: data}
}

func (r *bitReader) readBit() (bool, error) {
	if r.avail == 0 {
		if r.pos >= len(r.data) {
			return false, errors.New("gcs bitstream underrun")
		}
		r.bit = r.data[r.pos]
		r.pos++
		r.avail = 8
	}
	b := (r.bit >> (r.avail - 1)) & 1
	r.avail--
	return b == 1, nil
}

func (r *bitReader) readBits(n int) (uint64, error) {
	var v uint64
	for i := 0; i < n; i++ {
		b, err := r.readBit()
		if err != nil {
			return 0, err
		}
		if b {
			v |= 1 << uint(n-1-i)
		}
	}
	return v, nil
}

func (r *bitReader) readGolombRice(p int) (uint64, error) {
	var quotient uint64
	for {
		b, err := r.readBit()
		if err != nil {
			return 0, err
		}
		if !b {
			break
		}
		quotient++
	}
	remainder, err := r.readBits(p)
	if err != nil {
		return 0, err
	}
	return (quotient << uint(p)) | remainder, nil
}
