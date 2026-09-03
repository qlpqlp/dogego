// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package consensus

// Doginal lock redeem (apezord/doginals):
//   <pubkey> OP_CHECKSIGVERIFY OP_DROP{n} OP_TRUE
// Unlock scriptSig prepends inscription pushdatas, then <sig> <redeemScript>.

const (
	opCheckSigVerify = 0xad
	opDrop           = 0x75
	opTrue           = 0x51
)

// ParseDoginalLockRedeem extracts the controlling pubkey and DROP count.
func ParseDoginalLockRedeem(redeem []byte) (pubkey []byte, drops int, ok bool) {
	if len(redeem) < 4 {
		return nil, 0, false
	}
	op := redeem[0]
	var pubLen int
	switch op {
	case 0x21:
		pubLen = 33
	case 0x41:
		pubLen = 65
	default:
		return nil, 0, false
	}
	if len(redeem) < 1+pubLen+1 {
		return nil, 0, false
	}
	pub := redeem[1 : 1+pubLen]
	i := 1 + pubLen
	if redeem[i] != opCheckSigVerify {
		return nil, 0, false
	}
	i++
	for i < len(redeem) && redeem[i] == opDrop {
		drops++
		i++
	}
	if i != len(redeem)-1 || redeem[i] != opTrue {
		return nil, 0, false
	}
	if drops < 1 {
		return nil, 0, false
	}
	return append([]byte(nil), pub...), drops, true
}

// IsDoginalLockRedeem reports whether redeem matches the apezord doginal lock template.
func IsDoginalLockRedeem(redeem []byte) bool {
	_, _, ok := ParseDoginalLockRedeem(redeem)
	return ok
}
