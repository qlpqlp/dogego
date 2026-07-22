// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "strings"

// DogeGoSyncPhase classifies operator-visible sync state (Core-style: headers vs chainActive vs bodies).
func DogeGoSyncPhase(nodeMode string, headers, chainActive, contiguousRaw int64, genesisMissing bool) string {
	if strings.ToLower(strings.TrimSpace(nodeMode)) == "spv" || headers < 0 {
		return "caught_up"
	}
	if genesisMissing || (contiguousRaw < 0 && chainActive < 0) {
		return "awaiting_genesis_block"
	}
	if headers > chainActive {
		return "forward_block_ibd"
	}
	if contiguousRaw >= 0 && contiguousRaw < headers {
		return "forward_block_ibd"
	}
	if contiguousRaw < 0 && headers >= 0 {
		return "forward_block_ibd"
	}
	return "block_chain_connected"
}

// HeaderIBDProgress estimates header download vs a peer's advertised chain height (0..1).
func HeaderIBDProgress(localTip, peerStartHeight int64) float64 {
	if localTip < 0 || peerStartHeight <= 0 {
		return 0
	}
	if int64(peerStartHeight) <= localTip {
		return 1
	}
	p := float64(localTip+1) / float64(peerStartHeight+1)
	if p > 1 {
		return 1
	}
	if p < 0 {
		return 0
	}
	return p
}

// BodyIBDOwnsPipeline mirrors node.ShouldPauseHeaderCatchUpForBodyIBD thresholds for RPC/UI
// without importing the node package (Core: block download owns IBD once headers are far ahead).
func BodyIBDOwnsPipeline(headerTip, contiguousRaw int64) bool {
	if headerTip < 500_000 || contiguousRaw < 0 {
		return false
	}
	return headerTip-contiguousRaw > 50_000
}

// EffectiveIBDDisplayProgress picks operator-facing sync % during initial block download.
func EffectiveIBDDisplayProgress(localTip, contiguousRaw, peerStartHeight int64, ibd bool) float64 {
	if !ibd || localTip < 0 {
		return BodyVerificationProgress(localTip, contiguousRaw)
	}
	body := BodyVerificationProgress(localTip, contiguousRaw)
	if BodyIBDOwnsPipeline(localTip, contiguousRaw) {
		return body
	}
	if peerStartHeight > 0 && int64(peerStartHeight) > localTip {
		hdr := HeaderIBDProgress(localTip, peerStartHeight)
		if body < 0.01 {
			return hdr
		}
		if hdr > body {
			return hdr
		}
	}
	return body
}

// HeadersSyncProgress approximates Core header-sync progress when headers run ahead of chainActive.
func HeadersSyncProgress(headers, chainActive int64) float64 {
	if headers < 0 {
		return 1
	}
	if chainActive < 0 {
		chainActive = -1
	}
	if headers <= chainActive {
		return 1
	}
	p := float64(chainActive+1) / float64(headers+1)
	if p > 1 {
		return 1
	}
	if p < 0 {
		return 0
	}
	return p
}

// BlocksBehindHeaders is the operator lag metric: max(header tip − chainActive, header tip − contiguous bodies).
func BlocksBehindHeaders(headers, chainActive, contiguousRaw int64) int64 {
	if headers < 0 {
		return 0
	}
	behind := int64(0)
	if chainActive >= 0 && headers > chainActive {
		behind = headers - chainActive
	}
	if contiguousRaw >= 0 {
		if gap := headers - contiguousRaw; gap > behind {
			behind = gap
		}
	} else if headers >= 0 {
		gap := headers + 1
		if gap > behind {
			behind = gap
		}
	}
	if behind < 0 {
		return 0
	}
	return behind
}
