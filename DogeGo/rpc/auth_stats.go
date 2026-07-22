// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "sync/atomic"

var rpcAuthFailures uint64

// RecordRPCAuthFailure increments the Core getrpcinfo authentication_failures counter.
func RecordRPCAuthFailure() {
	atomic.AddUint64(&rpcAuthFailures, 1)
}

// RPCAuthFailures returns failed HTTP Basic attempts since process start.
func RPCAuthFailures() uint64 {
	return atomic.LoadUint64(&rpcAuthFailures)
}
