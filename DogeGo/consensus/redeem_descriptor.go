// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/chain"

// P2SHRedeemDescriptor returns a watch descriptor string for a known P2SH redeem script template.
func P2SHRedeemDescriptor(redeem []byte, pubkeyHashAddrVersion byte) (string, bool) {
	if desc, ok := ShMultiDescriptorFromRedeem(redeem); ok {
		return desc, true
	}
	if desc, ok := ShTimelockMultiDescriptorFromRedeem(redeem, opCheckLockTimeVerify, "cltv"); ok {
		return desc, true
	}
	if desc, ok := ShTimelockMultiDescriptorFromRedeem(redeem, opCheckSequenceVerify, "csv"); ok {
		return desc, true
	}
	if desc, ok := ShTimelockPKHDescriptorFromRedeem(redeem, opCheckLockTimeVerify, "cltv", pubkeyHashAddrVersion); ok {
		return desc, true
	}
	if desc, ok := ShTimelockPKHDescriptorFromRedeem(redeem, opCheckSequenceVerify, "csv", pubkeyHashAddrVersion); ok {
		return desc, true
	}
	if len(redeem) == 25 && redeem[0] == 0x76 {
		if addr := chain.PayToPubKeyHashAddress(redeem, pubkeyHashAddrVersion); addr != "" {
			return "sh(pkh(" + addr + "))", true
		}
	}
	return "", false
}

// MultisigRedeemFromP2SH returns multisig params from a bare or CLTV/CSV-wrapped P2SH redeem.
func MultisigRedeemFromP2SH(redeem []byte) (nRequired int, pubkeys [][]byte, ok bool) {
	if n, pubs, err := ParseMultisigRedeemScript(redeem); err == nil {
		return n, pubs, true
	}
	if _, inner, err := parseTimelockMultisigRedeem(redeem, opCheckLockTimeVerify); err == nil {
		if n, pubs, err := ParseMultisigRedeemScript(inner); err == nil {
			return n, pubs, true
		}
	}
	if _, inner, err := parseTimelockMultisigRedeem(redeem, opCheckSequenceVerify); err == nil {
		if n, pubs, err := ParseMultisigRedeemScript(inner); err == nil {
			return n, pubs, true
		}
	}
	return 0, nil, false
}
