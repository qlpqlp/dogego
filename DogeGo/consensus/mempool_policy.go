// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// MempoolRelayLimits holds configurable mempool admission limits (Core -maxtxfee, -limit*).
type MempoolRelayLimits struct {
	MaxTxFeeKoinu         int64
	LimitAncestorCount    int
	LimitDescendantCount  int
	LimitAncestorSizeKB   int
	LimitDescendantSizeKB int
}

// Apply copies non-zero limits onto admission policy.
func (l MempoolRelayLimits) Apply(adm *MempoolAdmission) {
	if adm == nil {
		return
	}
	if l.MaxTxFeeKoinu > 0 {
		adm.MaxAbsurdFeeKoinu = l.MaxTxFeeKoinu
	}
	if l.LimitAncestorCount > 0 {
		adm.MaxMempoolAncestors = l.LimitAncestorCount
	}
	if l.LimitDescendantCount > 0 {
		adm.MaxMempoolDescendants = l.LimitDescendantCount
	}
	if l.LimitAncestorSizeKB > 0 {
		adm.MaxMempoolAncestorSizeKB = l.LimitAncestorSizeKB
	}
	if l.LimitDescendantSizeKB > 0 {
		adm.MaxMempoolDescendantSizeKB = l.LimitDescendantSizeKB
	}
}
