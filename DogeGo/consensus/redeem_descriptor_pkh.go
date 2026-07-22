// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"strconv"

	"dogego/chain"
)

// ShTimelockPKHDescriptorFromRedeem builds sh(cltv(N)pkh(...)) or sh(csv(N)pkh(...)) from a P2SH redeem.
func ShTimelockPKHDescriptorFromRedeem(redeem []byte, timeOpcode byte, tag string, pubkeyHashAddrVersion byte) (string, bool) {
	var lock int64
	var inner []byte
	var err error
	switch timeOpcode {
	case opCheckLockTimeVerify:
		lock, inner, err = parseTimelockDropRedeem(redeem, opCheckLockTimeVerify)
	case opCheckSequenceVerify:
		lock, inner, err = parseTimelockDropRedeem(redeem, opCheckSequenceVerify)
	default:
		return "", false
	}
	if err != nil {
		return "", false
	}
	addr := chain.PayToPubKeyHashAddress(inner, pubkeyHashAddrVersion)
	if addr == "" {
		return "", false
	}
	return "sh(" + tag + "(" + strconv.FormatInt(lock, 10) + ")pkh(" + addr + "))", true
}
