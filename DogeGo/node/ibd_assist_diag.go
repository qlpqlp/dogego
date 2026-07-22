// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "sync"

var ibdAssistDiag struct {
	mu     sync.RWMutex
	poolFn func() *BlockAssistCandidates
	reg    *AssistPeerRegistry
}

// SetIBDAssistDiagnostics wires live assist pool/registry for RPC progress snapshots.
func SetIBDAssistDiagnostics(poolFn func() *BlockAssistCandidates, reg *AssistPeerRegistry) {
	ibdAssistDiag.mu.Lock()
	ibdAssistDiag.poolFn = poolFn
	ibdAssistDiag.reg = reg
	ibdAssistDiag.mu.Unlock()
}

func enrichAssistDiagnosticsAuto(snap map[string]interface{}) {
	ibdAssistDiag.mu.RLock()
	poolFn := ibdAssistDiag.poolFn
	reg := ibdAssistDiag.reg
	ibdAssistDiag.mu.RUnlock()
	var pool *BlockAssistCandidates
	if poolFn != nil {
		pool = poolFn()
	}
	enrichAssistDiagnostics(snap, pool, reg)
}
