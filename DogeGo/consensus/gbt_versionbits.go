// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/chain"
)

// GBTVersionBitsResult holds getblocktemplate version-bits fields (Dogecoin Core mining.cpp).
type GBTVersionBitsResult struct {
	VBAvailable map[string]int
	Rules       []string
}

// GBTVBName returns the deployment name for GBT vbavailable/rules (Core gbt_vb_name).
func GBTVBName(dep BIP9Deployment) string {
	if dep.GBTForce {
		return dep.Name
	}
	return "!" + dep.Name
}

// GBTVersionBits evaluates BIP9 deployments for getblocktemplate (Core getblocktemplate loop).
// clientRules lists rule names the miner claims to support (from template request "rules").
// Dogecoin Core always sets vbrequired to 0; use GBTVersionBitsResult only for vbavailable/rules.
func GBTVersionBits(j HeaderChain, net chain.Network, prevHeight int64, clientRules map[string]struct{}) (GBTVersionBitsResult, error) {
	res := GBTVersionBitsResult{VBAvailable: map[string]int{}}
	if j == nil {
		return res, nil
	}
	p := BIP9ParamsForNetwork(net)
	for _, dep := range p.Deployments {
		if dep.Timeout == 0 {
			continue
		}
		r, err := EvaluateBIP9AtTip(j, net, dep, p.Period, p.Threshold)
		if err != nil {
			continue
		}
		vbName := GBTVBName(dep)
		switch r.Status {
		case ThresholdDefined, ThresholdFailed:
			continue
		case ThresholdLockedIn, ThresholdStarted:
			res.VBAvailable[vbName] = dep.Bit
		case ThresholdActive:
			res.Rules = append(res.Rules, vbName)
			if !dep.GBTForce {
				if _, ok := clientRules[dep.Name]; !ok {
					return res, fmt.Errorf("support for '%s' rule requires explicit client support", dep.Name)
				}
			}
		}
	}
	return res, nil
}

// GBTBlockVersion returns nVersion for getblocktemplate (Core CreateNewBlock + client rules masking).
// Signaling bits are included only for STARTED/LOCKED_IN deployments the client supports (or GBTForce).
func GBTBlockVersion(j HeaderChain, net chain.Network, prevHeight int64, clientRules map[string]struct{}) uint32 {
	next := prevHeight + 1
	dc := LookupConsensus(net, next)
	ver := uint32(1)
	if !dc.AllowLegacyBlocks {
		ver |= 1 << 8 // VERSION_AUXPOW
		ver |= uint32(dc.AuxpowChainID) << 16
	}
	if j == nil {
		return ver
	}
	p := BIP9ParamsForNetwork(net)
	for _, dep := range p.Deployments {
		if dep.Timeout == 0 {
			continue
		}
		r, err := EvaluateBIP9AtTip(j, net, dep, p.Period, p.Threshold)
		if err != nil {
			continue
		}
		if r.Status != ThresholdLockedIn && r.Status != ThresholdStarted {
			continue
		}
		if !dep.GBTForce {
			if _, ok := clientRules[dep.Name]; !ok {
				continue
			}
		}
		ver |= uint32(VersionBitsTopBits)
		ver |= uint32(1) << uint(dep.Bit)
	}
	return ver
}
