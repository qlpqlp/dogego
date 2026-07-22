// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// PubKeyHash160 returns RIPEMD160(SHA256(pubKey)) for wallet-anchored live probes.
func PubKeyHash160(pubCompressed []byte) [20]byte {
	return hash160(pubCompressed)
}
