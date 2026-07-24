// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

// P2P NODE_* service bits (Bitcoin Core protocol.h / BIP157).
const (
	ServiceNetwork        uint64 = 1 << 0 // NODE_NETWORK
	ServiceGetUTXO        uint64 = 1 << 1 // NODE_GETUTXO
	ServiceBloom          uint64 = 1 << 2 // NODE_BLOOM
	ServiceWitness        uint64 = 1 << 3 // NODE_WITNESS (not used on Dogecoin)
	ServiceCompactFilters uint64 = 1 << 6 // NODE_COMPACT_FILTERS (BIP157)
	ServiceNetworkLimited uint64 = 1 << 10
	// ServiceDogeGoRelayCGNAT marks a public DogeGo QUIC reachability relay (rdogego).
	ServiceDogeGoRelayCGNAT uint64 = 1 << 29 // NODE_DOGEGO_RELAY_CGNAT
)

// EffectiveP2PServices returns service bits advertised on version/addr for this run.
func EffectiveP2PServices(p Params, advertiseCompactFilters, advertiseRelayCGNAT, advertiseBloom bool) uint64 {
	s := p.NodeNetwork
	if advertiseCompactFilters {
		s |= ServiceCompactFilters
	}
	if advertiseRelayCGNAT {
		s |= ServiceDogeGoRelayCGNAT
	}
	if advertiseBloom {
		s |= ServiceBloom
	}
	return s
}

// HasDogeGoRelayCGNAT reports NODE_DOGEGO_RELAY_CGNAT on a peer version/addr.
func HasDogeGoRelayCGNAT(services uint64) bool {
	return services&ServiceDogeGoRelayCGNAT != 0
}

// HasFullBlockRelay reports NODE_NETWORK (historical blocks), not limited-relay only.
func HasFullBlockRelay(services uint64) bool {
	return services&ServiceNetwork != 0
}

// limitedPeerPruneBase is how far below a peer's start height BIP159 limited relay reaches
// (288 block buffer + Core BLOCK_DOWNLOAD_WINDOW).
func limitedPeerPruneBase(peerStartHeight int32) int64 {
	if peerStartHeight <= 0 {
		return 0
	}
	const limitedKeep = 288 + 1024
	base := int64(peerStartHeight) - limitedKeep
	if base < 0 {
		return 0
	}
	return base
}

// PeerLikelyHasBlock estimates whether a peer can serve wantHeight (Core pruned / BIP159 limited peers).
// NODE_NETWORK_LIMITED is checked before NODE_NETWORK: many pruned peers set both bits but cannot serve ancient blocks.
func PeerLikelyHasBlock(services uint64, peerStartHeight int32, wantHeight int64) bool {
	if wantHeight < 0 {
		return true
	}
	if services&ServiceNetworkLimited != 0 {
		if peerStartHeight <= 0 {
			return false
		}
		return wantHeight >= limitedPeerPruneBase(peerStartHeight)
	}
	if HasFullBlockRelay(services) {
		return true
	}
	return true // unknown service bits - do not exclude
}
