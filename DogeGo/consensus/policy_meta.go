// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// EffectiveMempoolRelayLimits fills zero fields with Dogecoin Core defaults.
func EffectiveMempoolRelayLimits(l MempoolRelayLimits) MempoolRelayLimits {
	out := l
	if out.LimitAncestorCount <= 0 {
		out.LimitAncestorCount = DefaultMaxMempoolAncestors
	}
	if out.LimitDescendantCount <= 0 {
		out.LimitDescendantCount = DefaultMaxMempoolDescendants
	}
	if out.LimitAncestorSizeKB <= 0 {
		out.LimitAncestorSizeKB = DefaultMaxMempoolAncestorSizeKB
	}
	if out.LimitDescendantSizeKB <= 0 {
		out.LimitDescendantSizeKB = DefaultMaxMempoolDescendantSizeKB
	}
	return out
}

// MempoolPackagePolicyMap returns effective package limits for RPC/UI (Core -limit*).
func MempoolPackagePolicyMap(l MempoolRelayLimits) map[string]any {
	e := EffectiveMempoolRelayLimits(l)
	return map[string]any{
		"limitancestorcount":    e.LimitAncestorCount,
		"limitdescendantcount":  e.LimitDescendantCount,
		"limitancestorsize":     e.LimitAncestorSizeKB,
		"limitdescendantsize":   e.LimitDescendantSizeKB,
	}
}

// StandardPolicyMap returns effective standardness policy for RPC/UI.
func StandardPolicyMap(p StandardPolicy) map[string]any {
	maxCarrier := p.MaxDatacarrierBytes
	if maxCarrier <= 0 {
		maxCarrier = MaxDatacarrierBytes
	}
	hard := p.HardDustLimitKoinu
	if hard <= 0 {
		hard = HardDustLimitKoinu
	}
	return map[string]any{
		"acceptdatacarrier":  p.AcceptDataCarrier,
		"permitbaremultisig": p.AllowBareMultisig,
		"datacarriersize":    maxCarrier,
		"harddustlimit":      hard,
	}
}

// MempoolRelayLimitsFromConfig builds limits from dogecoinconf.json fields (zero = unset).
func MempoolRelayLimitsFromConfig(
	maxTxFee int64,
	ancCount, descCount, ancSizeKB, descSizeKB int,
) MempoolRelayLimits {
	return MempoolRelayLimits{
		MaxTxFeeKoinu:         maxTxFee,
		LimitAncestorCount:    ancCount,
		LimitDescendantCount:  descCount,
		LimitAncestorSizeKB:   ancSizeKB,
		LimitDescendantSizeKB: descSizeKB,
	}
}
