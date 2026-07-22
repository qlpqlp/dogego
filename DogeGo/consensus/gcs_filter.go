// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"sort"

	"dogego/wire"
)

// BIP158 basic filter constants (Bitcoin Core blockfilter.cpp).
const (
	BasicFilterP = 19
	BasicFilterM = 784931
)

// BuildBasicGCSFilter constructs a BIP158 basic filter for block outputs and spent prevout scripts.
func BuildBasicGCSFilter(blockHashLE [32]byte, outputScripts, inputScripts [][]byte) []byte {
	seen := make(map[string]struct{})
	add := func(script []byte) {
		if len(script) == 0 {
			return
		}
		if len(script) > 0 && script[0] == 0x6a { // OP_RETURN
			return
		}
		seen[string(script)] = struct{}{}
	}
	for _, s := range outputScripts {
		add(s)
	}
	for _, s := range inputScripts {
		add(s)
	}
	if len(seen) == 0 {
		var out bytes.Buffer
		_ = wire.WriteCompactSize(&out, 0)
		return out.Bytes()
	}
	keys := make([][]byte, 0, len(seen))
	for k := range seen {
		keys = append(keys, []byte(k))
	}
	k0 := binary.LittleEndian.Uint64(blockHashLE[0:8])
	k1 := binary.LittleEndian.Uint64(blockHashLE[8:16])
	N := uint64(len(keys))
	F := N * BasicFilterM

	hashed := make([]uint64, N)
	for i, el := range keys {
		h := SipHash24(k0, k1, el)
		hashed[i] = fastRange64(h, F)
	}
	sort.Slice(hashed, func(i, j int) bool { return hashed[i] < hashed[j] })

	var enc bytes.Buffer
	_ = wire.WriteCompactSize(&enc, N)
	bw := &bitWriter{}
	var last uint64
	for i, v := range hashed {
		delta := v - last
		if i == 0 {
			delta = v
		}
		golombRiceEncode(bw, BasicFilterP, delta)
		last = v
	}
	enc.Write(bw.flush())
	return enc.Bytes()
}

func fastRange64(x, f uint64) uint64 {
	hi, _ := bits.Mul64(x, f)
	return hi
}

func golombRiceEncode(w *bitWriter, p int, v uint64) {
	quotient := v >> uint(p)
	remainder := v & ((uint64(1) << uint(p)) - 1)
	for i := uint64(0); i < quotient; i++ {
		w.writeBit(true)
	}
	w.writeBit(false)
	w.writeBits(remainder, p)
}

// BasicFilterElementCount returns unique script elements that would be included in a basic filter.
func BasicFilterElementCount(outputScripts, inputScripts [][]byte) int {
	seen := make(map[string]struct{})
	add := func(script []byte) {
		if len(script) == 0 || (len(script) > 0 && script[0] == 0x6a) {
			return
		}
		seen[string(script)] = struct{}{}
	}
	for _, s := range outputScripts {
		add(s)
	}
	for _, s := range inputScripts {
		add(s)
	}
	return len(seen)
}
