// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

// Core policy defaults (policy.h / validation.h).
const (
	DefaultMaxMempoolMB      = 300
	DefaultMempoolExpiryHours = 24
)

// MaxMempoolBytes returns the byte cap for maxmempool in megabytes (Core uses n * 1_000_000).
func MaxMempoolBytes(mb int) int {
	if mb <= 0 {
		mb = DefaultMaxMempoolMB
	}
	return mb * 1_000_000
}

// MempoolExpirySeconds returns expiry duration in seconds (Core -mempoolexpiry hours).
func MempoolExpirySeconds(hours int) int64 {
	if hours <= 0 {
		hours = DefaultMempoolExpiryHours
	}
	return int64(hours) * 3600
}
