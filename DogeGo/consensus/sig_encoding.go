// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/chain"
	"dogego/wire"
)

// Script policy flags beyond height-activated consensus soft forks.
const (
	// ScriptVerifyLowS requires low-S ECDSA signatures (BIP62, standard mempool policy).
	ScriptVerifyLowS ScriptVerifyFlags = 1 << 3
	// ScriptVerifyNullFail requires empty signature pushes on failed CHECK(MULTI)SIG (BIP146 segment).
	ScriptVerifyNullFail ScriptVerifyFlags = 1 << 14
	// ScriptVerifyStrictEnc requires a defined base sighash type byte.
	ScriptVerifyStrictEnc ScriptVerifyFlags = 1 << 1
	// ScriptVerifyNullDummy requires an empty CHECKMULTISIG dummy push (BIP147; mempool standard only on DogeGo).
	ScriptVerifyNullDummy ScriptVerifyFlags = 1 << 4
)

// secp256k1HalfOrder is N/2 for low-S checks (OpenSSL / Core CPubKey::CheckLowS).
var secp256k1HalfOrder = []byte{
	0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x5d, 0x57, 0x6e, 0x73, 0x57, 0xa4, 0x50, 0x1d,
}

// ScriptFlagsForMempool returns height-dependent consensus flags plus standard relay policy.
func ScriptFlagsForMempool(spendHeight int64, net chain.Network, journal HeaderChain) ScriptVerifyFlags {
	return ScriptFlagsForChain(spendHeight, net, journal) |
		ScriptVerifyLowS | ScriptVerifyNullFail | ScriptVerifyStrictEnc | ScriptVerifyMinimalData |
		ScriptVerifyNullDummy | ScriptVerifyDiscourageUpgradableNops
}

func checkSignatureEncoding(sig []byte, flags ScriptVerifyFlags) error {
	if len(sig) == 0 {
		return nil
	}
	if flags&(ScriptVerifyDERSig|ScriptVerifyLowS|ScriptVerifyStrictEnc) != 0 {
		if !isValidSignatureEncoding(sig) {
			return fmt.Errorf("script-verify: non-DER signature")
		}
	}
	if flags&ScriptVerifyLowS != 0 {
		if !isLowDERSignature(sig) {
			return fmt.Errorf("script-verify: high-S signature (BIP62)")
		}
	}
	if flags&ScriptVerifyStrictEnc != 0 {
		if !isDefinedHashTypeSignature(sig) {
			return fmt.Errorf("script-verify: undefined signature hashtype")
		}
	}
	return nil
}

func isLowDERSignature(sig []byte) bool {
	if len(sig) == 0 {
		return true
	}
	der := sig[:len(sig)-1]
	sBytes, ok := derSignatureS(der)
	if !ok {
		return false
	}
	return compareBigEndian(sBytes, secp256k1HalfOrder) <= 0
}

func isDefinedHashTypeSignature(sig []byte) bool {
	if len(sig) == 0 {
		return false
	}
	base := sig[len(sig)-1] &^ byte(wire.SigHashAnyoneCanPay)
	return base >= byte(wire.SigHashAll) && base <= byte(wire.SigHashSingle)
}

func derSignatureS(der []byte) ([]byte, bool) {
	if len(der) < 8 || der[0] != 0x30 {
		return nil, false
	}
	lenR := int(der[3])
	if 4+lenR >= len(der) || der[2] != 0x02 {
		return nil, false
	}
	off := 4 + lenR
	if off+2 > len(der) || der[off] != 0x02 {
		return nil, false
	}
	lenS := int(der[off+1])
	start := off + 2
	if start+lenS > len(der) {
		return nil, false
	}
	return der[start : start+lenS], true
}

// compareBigEndian compares two unsigned big-endian magnitudes (-1/0/1).
func compareBigEndian(a, b []byte) int {
	for len(a) > 0 && a[0] == 0 {
		a = a[1:]
	}
	for len(b) > 0 && b[0] == 0 {
		b = b[1:]
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
