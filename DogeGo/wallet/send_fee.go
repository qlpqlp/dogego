// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import "strings"

// LookupSendFee returns the persisted network fee for a confirmed wallet send (from block scan / live index).
func (w *Disk) LookupSendFee(txid string) (int64, bool) {
	if w == nil {
		return 0, false
	}
	txid = strings.ToLower(strings.TrimSpace(txid))
	if txid == "" {
		return 0, false
	}
	for _, r := range w.ListScannedTx() {
		if r.Category != "send" || !strings.EqualFold(r.TxID, txid) {
			continue
		}
		if r.FeeKoinu > 0 {
			return r.FeeKoinu, true
		}
	}
	return 0, false
}
