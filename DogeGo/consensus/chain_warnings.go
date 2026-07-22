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

const versionBitsLastOldBlockVersion = 4

// ChainWarnings returns Core-style operational warnings from the header chain (version bits, unexpected versions).
func ChainWarnings(j HeaderChain, net chain.Network) []string {
	if j == nil {
		return nil
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 0 {
		return nil
	}
	var out []string
	out = append(out, unknownVersionBitWarnings(j, net, tip)...)
	out = append(out, unexpectedBlockVersionWarning(j, net, tip)...)
	return out
}

func unknownVersionBitWarnings(j HeaderChain, net chain.Network, tip int64) []string {
	h80, err := j.ReadHeaderAt(tip)
	if err != nil {
		return nil
	}
	ver := headerVersion(h80)
	known := knownDeploymentBits(net)
	var out []string
	for bit := 0; bit < 32; bit++ {
		mask := uint32(1) << uint(bit)
		if !deploymentSignaled(ver, mask) {
			continue
		}
		if known[bit] {
			continue
		}
		out = append(out, fmt.Sprintf("unknown new rules activated (versionbit %d)", bit))
	}
	return out
}

func knownDeploymentBits(net chain.Network) map[int]bool {
	p := BIP9ParamsForNetwork(net)
	m := make(map[int]bool, len(p.Deployments))
	for _, d := range p.Deployments {
		m[d.Bit] = true
	}
	return m
}

func unexpectedBlockVersionWarning(j HeaderChain, net chain.Network, tip int64) []string {
	nUnexpected := 0
	for i := 0; i < 100; i++ {
		h := tip - int64(i)
		if h < 0 {
			break
		}
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			break
		}
		ver := headerVersion(h80)
		if BlockBaseVersion(ver) <= versionBitsLastOldBlockVersion {
			continue
		}
		var prev int64
		if h > 0 {
			prev = h - 1
		}
		exp := ComputeBlockVersion(j, net, prev)
		if int32(ver)&^int32(exp) != 0 {
			nUnexpected++
		}
	}
	if nUnexpected == 0 {
		return nil
	}
	msg := fmt.Sprintf("%d of last 100 blocks have unexpected version", nUnexpected)
	if nUnexpected > 50 {
		return []string{msg + "; possible unknown rules in effect"}
	}
	return []string{msg}
}
