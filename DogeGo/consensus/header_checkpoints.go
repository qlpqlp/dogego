// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"strings"
	"sync/atomic"

	"dogego/chain"
	"dogego/pow"
)

// headerCheckpointsEnabled mirrors Core -checkpoints (default true).
var headerCheckpointsEnabled atomic.Bool

func init() {
	headerCheckpointsEnabled.Store(true)
}

// SetHeaderCheckpointsEnabled toggles Core mapCheckpoints hash checks during header validation.
func SetHeaderCheckpointsEnabled(on bool) {
	headerCheckpointsEnabled.Store(on)
}

// HeaderCheckpointsEnabled reports whether checkpoint hash checks run.
func HeaderCheckpointsEnabled() bool {
	return headerCheckpointsEnabled.Load()
}

func checkHeaderCheckpoint(net chain.Network, height int64, header80 []byte) error {
	if !headerCheckpointsEnabled.Load() {
		return nil
	}
	want, ok := chain.CheckpointHashAt(net, height)
	if !ok {
		return nil
	}
	got := strings.ToLower(pow.BlockHashHex(header80))
	want = strings.ToLower(strings.TrimPrefix(want, "0x"))
	if got != want {
		return fmt.Errorf("header at height %d: checkpoint hash mismatch (got %s… want %s…) - wrong chain or corrupt headers.bin; delete headers.bin or use -network testnet",
			height, got[:12], want[:12])
	}
	return nil
}
