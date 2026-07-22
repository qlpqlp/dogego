// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/big"

	"dogego/chain"
	"dogego/consensus"
	"dogego/store"
)

func headerTimeSpanInclusive(j HeaderJournal, lo, hi int64) (minT, maxT int64, err error) {
	if lo > hi {
		return 0, 0, fmt.Errorf("bad height span %d..%d", lo, hi)
	}
	for h := lo; h <= hi; h++ {
		buf, err := j.ReadHeaderAt(h)
		if err != nil {
			return 0, 0, err
		}
		if len(buf) < 72 {
			return 0, 0, fmt.Errorf("header at %d: short buffer", h)
		}
		t := int64(binary.LittleEndian.Uint32(buf[68:72]))
		if h == lo {
			minT, maxT = t, t
			continue
		}
		if t < minT {
			minT = t
		}
		if t > maxT {
			maxT = t
		}
	}
	return minT, maxT, nil
}

func parseRPCInt64Param(msg json.RawMessage) (int64, bool) {
	var v float64
	if err := json.Unmarshal(msg, &v); err != nil {
		return 0, false
	}
	if v < float64(math.MinInt64) || v > float64(math.MaxInt64) || v != float64(int64(v)) {
		return 0, false
	}
	return int64(v), true
}

// execGetNetworkHashPS estimates network hashes per second from chainwork and header timestamps (Core getnetworkhashps).
func execGetNetworkHashPS(j HeaderJournal, raw *store.RawBlockStore, paths *DataPaths, net chain.Network, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 2 {
		return nil, -8, "getnetworkhashps: too many arguments"
	}

	tip, _, _ := activeChainFromJournal(j, raw, paths)
	if tip < 1 {
		return float64(0), 0, ""
	}

	lookup := int64(120)
	if len(params) > 0 {
		v, ok := parseRPCInt64Param(params[0])
		if !ok {
			return nil, -8, "getnetworkhashps: nblocks must be integer"
		}
		lookup = v
	}

	height := int64(-1)
	if len(params) > 1 {
		v, ok := parseRPCInt64Param(params[1])
		if !ok {
			return nil, -8, "getnetworkhashps: height must be integer"
		}
		height = v
	}

	end := tip
	if height >= 0 && height < tip {
		end = height
	}

	if lookup <= 0 {
		adj := consensus.LookupConsensus(net, end).DifficultyAdjustmentBlocks()
		lookup = end%adj + 1
	}
	if lookup > end {
		lookup = end
	}

	workEnd, err := cumulativeChainworkBig(j, end)
	if err != nil {
		return nil, -1, err.Error()
	}
	workBase, err := cumulativeChainworkBig(j, end-lookup)
	if err != nil {
		return nil, -1, err.Error()
	}
	var workDiff big.Int
	workDiff.Sub(workEnd, workBase)

	lowH := end - lookup
	minT, maxT, err := headerTimeSpanInclusive(j, lowH, end)
	if err != nil {
		return nil, -1, err.Error()
	}
	if minT == maxT {
		return float64(0), 0, ""
	}

	dt := maxT - minT
	if dt <= 0 {
		return float64(0), 0, ""
	}

	rat := new(big.Rat).SetFrac(&workDiff, big.NewInt(dt))
	f, _ := rat.Float64()
	return f, 0, ""
}
