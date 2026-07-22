// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// isValidSignatureEncoding implements Core IsValidSignatureEncoding (BIP66), including sighash byte.
func isValidSignatureEncoding(sig []byte) bool {
	if len(sig) < 9 || len(sig) > 73 {
		return false
	}
	if sig[0] != 0x30 {
		return false
	}
	if sig[1] != byte(len(sig)-3) {
		return false
	}
	lenR := int(sig[3])
	if 5+lenR >= len(sig) {
		return false
	}
	lenS := int(sig[5+lenR])
	if lenR+lenS+7 != len(sig) {
		return false
	}
	if sig[2] != 0x02 || lenR == 0 || sig[4]&0x80 != 0 {
		return false
	}
	if lenR > 1 && sig[4] == 0x00 && sig[5]&0x80 == 0 {
		return false
	}
	if sig[lenR+4] != 0x02 || lenS == 0 || sig[lenR+6]&0x80 != 0 {
		return false
	}
	if lenS > 1 && sig[lenR+6] == 0x00 && sig[lenR+7]&0x80 == 0 {
		return false
	}
	return true
}
