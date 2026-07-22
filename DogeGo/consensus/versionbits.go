// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"fmt"

	"dogego/chain"
)

// Version-bits layout (Core versionbits.h).
const (
	VersionBitsTopBits int32 = 0x20000000
	VersionBitsTopMask int32 = -0x20000000 // 0xE0000000 as signed int32 (Core)
)

// ThresholdState is a BIP9 deployment state (Core ThresholdState).
type ThresholdState int

const (
	ThresholdDefined ThresholdState = iota
	ThresholdStarted
	ThresholdLockedIn
	ThresholdActive
	ThresholdFailed
)

func (s ThresholdState) String() string {
	switch s {
	case ThresholdDefined:
		return "defined"
	case ThresholdStarted:
		return "started"
	case ThresholdLockedIn:
		return "locked_in"
	case ThresholdActive:
		return "active"
	case ThresholdFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// BIP9Deployment is one version-bits deployment (Core Consensus::BIP9Deployment).
type BIP9Deployment struct {
	Name      string
	Bit       int
	StartTime int64
	Timeout   int64 // 0 = disabled (never leaves defined via timeout)
	// GBTForce when true uses the bare deployment name in getblocktemplate (Core VersionBitsDeploymentInfo.gbt_force).
	GBTForce bool
}

// BIP9Params groups deployments and activation thresholds for a network.
type BIP9Params struct {
	Period    int
	Threshold int
	Deployments []BIP9Deployment
}

// BIP9ParamsForNetwork returns Core-equivalent BIP9 parameters.
func BIP9ParamsForNetwork(net chain.Network) BIP9Params {
	switch net {
	case chain.MainnetDogecoin:
		return BIP9Params{
			Period:    10080,
			Threshold: 9576,
			Deployments: []BIP9Deployment{
				{Name: "csv", Bit: 0, StartTime: 1462060800, Timeout: 1493596800, GBTForce: true},
				{Name: "segwit", Bit: 1, StartTime: 1479168000, Timeout: 0, GBTForce: true},
			},
		}
	case chain.RebootTestnet:
		return BIP9Params{
			Period:    10080,
			Threshold: 2880,
			Deployments: []BIP9Deployment{
				{Name: "csv", Bit: 0, StartTime: 1456790400, Timeout: 1493596800, GBTForce: true},
				{Name: "segwit", Bit: 1, StartTime: 1462060800, Timeout: 0, GBTForce: true},
			},
		}
	default:
		return BIP9Params{Period: 10080, Threshold: 9576}
	}
}

// BIP9Result is the evaluated deployment at a chain tip.
type BIP9Result struct {
	Status    ThresholdState
	Since     int64
	Bit       int
	StartTime int64
	Timeout   int64
}

// BIP9PeriodStats reports signalling within the deployment period containing tip (Core getdeploymentinfo statistics).
type BIP9PeriodStats struct {
	Period    int
	Threshold int
	Elapsed   int
	Count     int
	Possible  bool
}

// BIP9PeriodStatsAt counts version-bit signals in the current period at tip.
func BIP9PeriodStatsAt(j HeaderChain, tip int64, dep BIP9Deployment, period, threshold int) (BIP9PeriodStats, error) {
	out := BIP9PeriodStats{Period: period, Threshold: threshold}
	if j == nil || tip < 0 || period < 1 {
		return out, fmt.Errorf("version bits: missing chain or params")
	}
	mask := uint32(1) << uint(dep.Bit)
	periodEnd := tip - (tip % int64(period)) + int64(period) - 1
	if periodEnd > tip {
		periodEnd = tip
	}
	periodStart := periodEnd - int64(period) + 1
	if periodStart < 0 {
		periodStart = 0
	}
	out.Elapsed = int(periodEnd - periodStart + 1)
	remaining := int(periodEnd - tip)
	for h := periodStart; h <= periodEnd; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return out, err
		}
		if deploymentSignaled(headerVersion(h80), mask) {
			out.Count++
		}
	}
	out.Possible = out.Count+remaining >= threshold
	return out, nil
}

// BlockBaseVersion extracts the base version without auxpow flag or chain ID (Core CPureBlockHeader::GetBaseVersion).
func BlockBaseVersion(versionLE uint32) int32 {
	return int32(versionLE % 256)
}

// ComputeBlockVersion returns the nVersion for the block after prevHeight (Core ComputeBlockVersion + Dogecoin aux layout).
func ComputeBlockVersion(j HeaderChain, net chain.Network, prevHeight int64) uint32 {
	next := prevHeight + 1
	dc := LookupConsensus(net, next)
	ver := uint32(1)
	if !dc.AllowLegacyBlocks {
		ver |= 1 << 8 // VERSION_AUXPOW
		ver |= uint32(dc.AuxpowChainID) << 16
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
		if r.Status == ThresholdLockedIn || r.Status == ThresholdStarted {
			ver |= uint32(VersionBitsTopBits)
			ver |= uint32(1) << uint(dep.Bit)
		}
	}
	return ver
}

// EvaluateBIP9AtTip computes deployment state at the header journal tip (Core VersionBitsState at pindexBestHeader).
func EvaluateBIP9AtTip(j HeaderChain, net chain.Network, dep BIP9Deployment, period, threshold int) (BIP9Result, error) {
	if j == nil {
		return EvaluateBIP9AtHeight(nil, -1, net, dep, period, threshold)
	}
	tip, err := j.TipHeight()
	if err != nil {
		return BIP9Result{}, err
	}
	return EvaluateBIP9AtHeight(j, tip, net, dep, period, threshold)
}

// EvaluateBIP9AtHeight computes deployment state at a specific height (Core VersionBitsState at pindex).
func EvaluateBIP9AtHeight(j HeaderChain, tip int64, net chain.Network, dep BIP9Deployment, period, threshold int) (BIP9Result, error) {
	res := BIP9Result{
		Status:    ThresholdDefined,
		Since:     0,
		Bit:       dep.Bit,
		StartTime: dep.StartTime,
		Timeout:   dep.Timeout,
	}
	if dep.Timeout == 0 {
		return res, nil
	}
	if j == nil || period < 1 || threshold < 1 {
		return res, fmt.Errorf("version bits: missing chain or params")
	}
	if tip < 0 {
		return res, nil
	}
	mask := uint32(1) << uint(dep.Bit)
	state := computeThresholdStateForward(j, tip, dep, period, threshold, mask)
	res.Status = state
	res.Since = stateSinceHeightForward(j, tip, dep, period, threshold, mask, state)
	return res, nil
}

func headerVersion(h80 []byte) uint32 {
	if len(h80) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(h80[0:4])
}

func deploymentSignaled(version uint32, mask uint32) bool {
	v := int32(version)
	return (v&VersionBitsTopMask) == VersionBitsTopBits && (version&mask) != 0
}

func alignPrevToPeriodEnd(prev int64, period int) int64 {
	if prev < 0 {
		return -1
	}
	return prev - ((prev + 1) % int64(period))
}

func computeThresholdStateForward(j HeaderChain, tip int64, dep BIP9Deployment, period, threshold int, mask uint32) ThresholdState {
	state := ThresholdDefined
	if tip < 0 {
		return state
	}
	for periodEnd := int64(period - 1); periodEnd <= tip; periodEnd += int64(period) {
		prev := alignPrevToPeriodEnd(periodEnd, period)
		state = thresholdStateNext(j, prev, state, dep, period, threshold, mask)
		if state == ThresholdActive || state == ThresholdFailed {
			break
		}
	}
	return state
}

func thresholdStateNext(j HeaderChain, prev int64, state ThresholdState, dep BIP9Deployment, period, threshold int, mask uint32) ThresholdState {
	mtp, err := MedianTimePastAt(j, prev)
	if err != nil {
		return state
	}
	switch state {
	case ThresholdDefined:
		if dep.Timeout > 0 && mtp >= dep.Timeout {
			return ThresholdFailed
		}
		if mtp >= dep.StartTime {
			return ThresholdStarted
		}
		return ThresholdDefined
	case ThresholdStarted:
		if dep.Timeout > 0 && mtp >= dep.Timeout {
			return ThresholdFailed
		}
		count := 0
		for i := 0; i < period; i++ {
			h := prev - int64(i)
			if h < 0 {
				break
			}
			h80, err := j.ReadHeaderAt(h)
			if err != nil {
				break
			}
			if deploymentSignaled(headerVersion(h80), mask) {
				count++
			}
		}
		if count >= threshold {
			return ThresholdLockedIn
		}
		return ThresholdStarted
	case ThresholdLockedIn:
		return ThresholdActive
	case ThresholdFailed, ThresholdActive:
		return state
	default:
		return state
	}
}

func stateSinceHeightForward(j HeaderChain, tip int64, dep BIP9Deployment, period, threshold int, mask uint32, final ThresholdState) int64 {
	if final == ThresholdDefined {
		return 0
	}
	since := int64(0)
	state := ThresholdDefined
	for periodEnd := int64(period - 1); periodEnd <= tip; periodEnd += int64(period) {
		prev := alignPrevToPeriodEnd(periodEnd, period)
		next := thresholdStateNext(j, prev, state, dep, period, threshold, mask)
		if next != state {
			since = prev + 1
		}
		state = next
		if state == final {
			break
		}
	}
	return since
}
