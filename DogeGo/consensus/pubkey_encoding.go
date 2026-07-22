// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/secp256k1"

func isCompressedOrUncompressedPubKey(pub []byte) bool {
	if len(pub) == 33 {
		return pub[0] == 0x02 || pub[0] == 0x03
	}
	if len(pub) == 65 {
		return pub[0] == 0x04
	}
	return false
}

func checkPubKeyEncoding(pub []byte, flags ScriptVerifyFlags) ScriptError {
	if flags&ScriptVerifyStrictEnc == 0 {
		return ScriptErrOK
	}
	if !isCompressedOrUncompressedPubKey(pub) {
		return ScriptErrPubKeyType
	}
	if _, err := secp256k1.ParsePubKey(pub); err != nil {
		return ScriptErrPubKeyType
	}
	return ScriptErrOK
}
