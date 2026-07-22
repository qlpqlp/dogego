// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package applog

import "sync/atomic"

// sink is set by the node process so packages (rpc, node) can emit lines without threading *Ring everywhere.
var sink atomic.Pointer[Ring]

// Register sets the global log sink (nil disables Line). Call from node.Run; defer Register(nil) on shutdown if desired.
func Register(r *Ring) {
	sink.Store(r)
}

// Line appends one log line to the registered sink, if any.
func Line(category, message string) {
	if r := sink.Load(); r != nil {
		r.Add(category, message)
	}
}
