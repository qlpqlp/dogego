// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// derLaxToCompact implements Core ecdsa_signature_parse_der_lax (pubkey.cpp / secp256k1 contrib).
// It accepts non-canonical DER padding for pre-BIP66 signatures when SCRIPT_VERIFY_DERSIG is off.
func derLaxToCompact(der []byte) ([]byte, bool) {
	input := der
	inputlen := len(der)
	var pos int
	var rpos, rlen, spos, slen int
	tmpsig := make([]byte, 64)

	if pos == inputlen || input[pos] != 0x30 {
		return nil, false
	}
	pos++

	if pos == inputlen {
		return nil, false
	}
	lenbyte := int(input[pos])
	pos++
	if lenbyte&0x80 != 0 {
		lenbyte -= 0x80
		if pos+lenbyte > inputlen {
			return nil, false
		}
		pos += lenbyte
	}

	if pos == inputlen || input[pos] != 0x02 {
		return nil, false
	}
	pos++

	if pos == inputlen {
		return nil, false
	}
	lenbyte = int(input[pos])
	pos++
	if lenbyte&0x80 != 0 {
		lenbyte -= 0x80
		if pos+lenbyte > inputlen {
			return nil, false
		}
		for lenbyte > 0 && pos < inputlen && input[pos] == 0 {
			pos++
			lenbyte--
		}
		if lenbyte >= 8 { // Core: lenbyte >= sizeof(size_t)
			return nil, false
		}
		rlen = 0
		for lenbyte > 0 {
			if pos >= inputlen {
				return nil, false
			}
			rlen = (rlen << 8) + int(input[pos])
			pos++
			lenbyte--
		}
	} else {
		rlen = lenbyte
	}
	if rlen > inputlen-pos {
		return nil, false
	}
	rpos = pos
	pos += rlen

	if pos == inputlen || input[pos] != 0x02 {
		return nil, false
	}
	pos++

	if pos == inputlen {
		return nil, false
	}
	lenbyte = int(input[pos])
	pos++
	if lenbyte&0x80 != 0 {
		lenbyte -= 0x80
		if pos+lenbyte > inputlen {
			return nil, false
		}
		for lenbyte > 0 && pos < inputlen && input[pos] == 0 {
			pos++
			lenbyte--
		}
		if lenbyte >= 8 { // Core: lenbyte >= sizeof(size_t)
			return nil, false
		}
		slen = 0
		for lenbyte > 0 {
			if pos >= inputlen {
				return nil, false
			}
			slen = (slen << 8) + int(input[pos])
			pos++
			lenbyte--
		}
	} else {
		slen = lenbyte
	}
	if slen > inputlen-pos {
		return nil, false
	}
	spos = pos

	overflow := false
	for rlen > 0 && input[rpos] == 0 {
		rlen--
		rpos++
	}
	if rlen > 32 {
		overflow = true
	} else if rlen > 0 {
		copy(tmpsig[32-rlen:32], input[rpos:rpos+rlen])
	}

	for slen > 0 && input[spos] == 0 {
		slen--
		spos++
	}
	if slen > 32 {
		overflow = true
	} else if slen > 0 {
		copy(tmpsig[64-slen:64], input[spos:spos+slen])
	}

	if overflow {
		return nil, false
	}
	return tmpsig, true
}
