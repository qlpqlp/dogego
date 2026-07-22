// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"
)

// verifyECDSASignatureLax verifies using Core ecdsa_signature_parse_der_lax (pre-BIP66).
func verifyECDSASignatureLax(sigDER, digest []byte, pub *secp256k1.PublicKey) bool {
	compact, ok := derLaxToCompact(sigDER)
	if !ok {
		return false
	}
	var r, s secp256k1.ModNScalar
	if r.SetByteSlice(compact[:32]) || s.SetByteSlice(compact[32:]) {
		return false
	}
	sig := ecdsa.NewSignature(&r, &s)
	return sig.Verify(digest, pub)
}

// verifyECDSASignatureStrict uses strict DER (BIP66+).
func verifyECDSASignatureStrict(sigDER, digest []byte, pub *secp256k1.PublicKey) bool {
	sig, err := ecdsa.ParseDERSignature(sigDER)
	if err != nil {
		return false
	}
	return sig.Verify(digest, pub)
}
