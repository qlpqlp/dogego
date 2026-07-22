// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"strconv"
	"strings"
)

// ShMultiDescriptorFromRedeem builds a Core-style sh(multi(N,hex,...)) string from a standard multisig redeem script.
func ShMultiDescriptorFromRedeem(redeem []byte) (string, bool) {
	nReq, pubs, err := ParseMultisigRedeemScript(redeem)
	if err != nil || nReq < 1 || len(pubs) == 0 {
		return "", false
	}
	parts := make([]string, 0, 1+len(pubs))
	parts = append(parts, strconv.Itoa(nReq))
	for _, pk := range pubs {
		parts = append(parts, hex.EncodeToString(pk))
	}
	return "sh(multi(" + strings.Join(parts, ",") + "))", true
}

// MultiDescriptorFromRedeem builds Core-style multi(N,hex,...) for bare multisig scriptPubKeys.
func MultiDescriptorFromRedeem(redeem []byte) (string, bool) {
	nReq, pubs, err := ParseMultisigRedeemScript(redeem)
	if err != nil || nReq < 1 || len(pubs) == 0 {
		return "", false
	}
	parts := make([]string, 0, 1+len(pubs))
	parts = append(parts, strconv.Itoa(nReq))
	for _, pk := range pubs {
		parts = append(parts, hex.EncodeToString(pk))
	}
	return "multi(" + strings.Join(parts, ",") + ")", true
}

// ShTimelockMultiDescriptorFromRedeem builds sh(cltv(N)multi(...)) or sh(csv(N)multi(...)) from a P2SH redeem.
func ShTimelockMultiDescriptorFromRedeem(redeem []byte, timeOpcode byte, tag string) (string, bool) {
	var lock int64
	var inner []byte
	var err error
	switch timeOpcode {
	case opCheckLockTimeVerify:
		lock, inner, err = parseTimelockMultisigRedeem(redeem, opCheckLockTimeVerify)
	case opCheckSequenceVerify:
		lock, inner, err = parseTimelockMultisigRedeem(redeem, opCheckSequenceVerify)
	default:
		return "", false
	}
	if err != nil {
		return "", false
	}
	multi, ok := MultiDescriptorFromRedeem(inner)
	if !ok {
		return "", false
	}
	return "sh(" + tag + "(" + strconv.FormatInt(lock, 10) + ")" + multi + ")", true
}
