// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"dogego/pow"
	"dogego/store"
)

// CumulativeChainWorkBig returns total chain work for heights 0..through (inclusive). through < 0 yields 0.
func CumulativeChainWorkBig(j HeaderJournal, through int64) (*big.Int, error) {
	return cumulativeChainworkBig(j, through)
}

// cumulativeChainworkBig returns total chain work for heights 0..through (inclusive). through < 0 yields 0.
func cumulativeChainworkBig(j HeaderJournal, through int64) (*big.Int, error) {
	sum := new(big.Int)
	if through < 0 {
		return sum, nil
	}
	if sj, ok := j.(*store.HeaderJournal); ok {
		raw, err := sj.ReadHeadersThrough(through)
		if err != nil {
			return nil, err
		}
		for off := 0; off < len(raw); off += 80 {
			bits := binary.LittleEndian.Uint32(raw[off+72 : off+76])
			w, err := pow.BlockProofFromBits(bits)
			if err != nil {
				return nil, err
			}
			sum.Add(sum, w)
		}
		return sum, nil
	}
	for h := int64(0); h <= through; h++ {
		buf, err := j.ReadHeaderAt(h)
		if err != nil {
			return nil, err
		}
		bits := binary.LittleEndian.Uint32(buf[72:76])
		w, err := pow.BlockProofFromBits(bits)
		if err != nil {
			return nil, err
		}
		sum.Add(sum, w)
	}
	return sum, nil
}

// cumulativeChainworkHex returns total chain work through height through (inclusive), as lowercase hex.
func cumulativeChainworkHex(j HeaderJournal, through int64) (string, error) {
	if through < 0 {
		return "0", fmt.Errorf("negative through height %d", through)
	}
	sum, err := cumulativeChainworkBig(j, through)
	if err != nil {
		return "", err
	}
	return pow.ChainworkHex(sum), nil
}
