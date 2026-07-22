// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import "time"

// Core net_processing.h ORPHAN_TX_EXPIRE_TIME / ORPHAN_TX_EXPIRE_INTERVAL.
const (
	OrphanTxExpireTime     = 20 * 60 // seconds
	OrphanTxExpireInterval = 5 * 60
)

// PruneExpired removes orphans older than OrphanTxExpireTime (Core mapOrphanTransactions sweep).
func (o *OrphanPool) PruneExpired() int {
	if o == nil {
		return 0
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.expiresAt) == 0 {
		return 0
	}
	now := time.Now().Unix()
	var drop []string
	for id, exp := range o.expiresAt {
		if exp > 0 && exp <= now {
			drop = append(drop, id)
		}
	}
	for _, id := range drop {
		o.removeLocked(id)
	}
	return len(drop)
}
