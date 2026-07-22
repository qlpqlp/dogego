// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package ecdsa provides DER and compact ECDSA signing and verification for DogeGo.
package ecdsa

import (
	"dogego/secp256k1"

	decredECDSA "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// Signature is a canonical low-S ECDSA signature.
type Signature = decredECDSA.Signature

// Sign produces a DER-encoded ECDSA signature for hash using RFC6979.
func Sign(key *secp256k1.PrivateKey, hash []byte) *Signature {
	return decredECDSA.Sign(key, hash)
}

// SignCompact returns a recoverable compact signature (Bitcoin message format).
func SignCompact(key *secp256k1.PrivateKey, hash []byte, isCompressedKey bool) []byte {
	return decredECDSA.SignCompact(key, hash, isCompressedKey)
}

// ParseDERSignature parses a strict DER ECDSA signature (BIP66+).
func ParseDERSignature(sig []byte) (*Signature, error) {
	return decredECDSA.ParseDERSignature(sig)
}

// NewSignature builds a signature from big-endian r and s scalars.
func NewSignature(r, s *secp256k1.ModNScalar) *Signature {
	return decredECDSA.NewSignature(r, s)
}

// RecoverCompact recovers the public key from a compact signature and message hash.
func RecoverCompact(signature, hash []byte) (*secp256k1.PublicKey, bool, error) {
	return decredECDSA.RecoverCompact(signature, hash)
}
