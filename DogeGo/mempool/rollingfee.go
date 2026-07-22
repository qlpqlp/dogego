// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"math"
	"time"
)

// RollingFeeHalflifeSeconds matches Core CTxMemPool::ROLLING_FEE_HALFLIFE (12 hours).
const RollingFeeHalflifeSeconds = 60 * 60 * 12

// NoteBlockFound resets rolling-fee decay after a block is connected (Core blockSinceLastRollingFeeBump).
func (p *Pool) NoteBlockFound() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.blockSinceLastRollingFeeBump = true
	p.mu.Unlock()
}

// MinRelayFeePerKB returns the mempool rolling minimum relay feerate in koinu per kB (Core GetMinFee).
func (p *Pool) MinRelayFeePerKB() uint64 {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.blockSinceLastRollingFeeBump || p.rollingMinFeePerKB <= 0 {
		return uint64(p.rollingMinFeePerKB)
	}
	incr := float64(p.incrementalRelayFeePerKB)
	if incr < 1 {
		incr = defaultIncrementalRelayFeePerKB
	}
	now := time.Now().Unix()
	if now > p.lastRollingFeeUpdate+10 {
		halflife := float64(RollingFeeHalflifeSeconds)
		n := len(p.raw)
		if p.max > 0 {
			if n < p.max/4 {
				halflife /= 4
			} else if n < p.max/2 {
				halflife /= 2
			}
		}
		elapsed := float64(now - p.lastRollingFeeUpdate)
		p.rollingMinFeePerKB = p.rollingMinFeePerKB / math.Pow(2, elapsed/halflife)
		p.lastRollingFeeUpdate = now
		if p.rollingMinFeePerKB < incr/2 {
			p.rollingMinFeePerKB = 0
			return 0
		}
	}
	rate := p.rollingMinFeePerKB
	if rate < incr {
		rate = incr
	}
	return uint64(rate)
}

func (p *Pool) trackPackageRemoved(feeKoinu int64, pkgSize int) {
	if p == nil || pkgSize <= 0 || feeKoinu < 0 {
		return
	}
	incr := float64(p.incrementalRelayFeePerKB)
	if incr < 1 {
		incr = defaultIncrementalRelayFeePerKB
	}
	rate := float64(feeKoinu)*1000/float64(pkgSize) + incr
	p.mu.Lock()
	defer p.mu.Unlock()
	if rate > p.rollingMinFeePerKB {
		p.rollingMinFeePerKB = rate
		p.blockSinceLastRollingFeeBump = false
		if p.lastRollingFeeUpdate == 0 {
			p.lastRollingFeeUpdate = time.Now().Unix()
		}
	}
}
