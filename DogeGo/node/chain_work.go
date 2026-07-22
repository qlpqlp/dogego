// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func incomingChainWork(decoded []wire.DecodedHeader) (*big.Int, error) {
	sum := big.NewInt(0)
	for _, d := range decoded {
		bits := binary.LittleEndian.Uint32(d.Header80[72:76])
		w, err := pow.BlockProofFromBits(bits)
		if err != nil {
			return nil, fmt.Errorf("incoming chain work: %w", err)
		}
		sum.Add(sum, w)
	}
	return sum, nil
}

func journalChainWork(j *store.HeaderJournal, from, to int64) (*big.Int, error) {
	if j == nil {
		return nil, fmt.Errorf("nil journal")
	}
	if from > to {
		return big.NewInt(0), nil
	}
	sum := big.NewInt(0)
	for h := from; h <= to; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return nil, err
		}
		bits := binary.LittleEndian.Uint32(h80[72:76])
		w, err := pow.BlockProofFromBits(bits)
		if err != nil {
			return nil, err
		}
		sum.Add(sum, w)
	}
	return sum, nil
}
