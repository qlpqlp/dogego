// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package clock provides process-wide time used for consensus checks (mockable via setmocktime RPC).
package clock

import (
	"sync/atomic"
	"time"
)

var mockUnix atomic.Int64

// SetMockUnix sets mock unix time; 0 disables mock time (real clock).
func SetMockUnix(ts int64) {
	mockUnix.Store(ts)
}

// MockUnix returns the mock timestamp, or 0 if disabled.
func MockUnix() int64 {
	return mockUnix.Load()
}

// UnixNow returns mock time when set, otherwise time.Now().Unix().
func UnixNow() int64 {
	if m := mockUnix.Load(); m != 0 {
		return m
	}
	return time.Now().Unix()
}
