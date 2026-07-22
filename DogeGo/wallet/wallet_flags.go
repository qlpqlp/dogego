// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

// AvoidReuse reports the wallet avoid_reuse flag (Core wallet flag subset).
func (w *Disk) AvoidReuse() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.avoidReuse
}

// SetAvoidReuse persists the avoid_reuse wallet flag.
func (w *Disk) SetAvoidReuse(v bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.avoidReuse = v
	w.rebuildUsedRecvScriptsLocked()
	return w.saveLocked()
}

// PqCommitmentsEnabled reports whether wallet sends may attach OP_RETURN PQ commitments.
func (w *Disk) PqCommitmentsEnabled() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pqCommitmentsEnabled
}

// SetPqCommitmentsEnabled persists the pq_commitments_enabled wallet flag.
func (w *Disk) SetPqCommitmentsEnabled(v bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pqCommitmentsEnabled = v
	if v {
		w.pqCarrierEnabled = true
	}
	return w.saveLocked()
}
