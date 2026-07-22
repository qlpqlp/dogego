// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package secp256k1 provides DogeGo's secp256k1 key types and helpers for
// Dogecoin script verification, wallet signing, and BIP32 derivation.
//
// Implementation is pure Go (no btcsuite dependency).
package secp256k1

import (
	decred "github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// Types mirror the scalar and point types used across DogeGo.
type (
	PrivateKey = decred.PrivateKey
	PublicKey  = decred.PublicKey
	ModNScalar = decred.ModNScalar
)

// PrivKeyFromBytes interprets b as a private scalar and returns the key pair.
// Returns nil keys when the scalar is zero (invalid for wallet/BIP32 use).
func PrivKeyFromBytes(b []byte) (*PrivateKey, *PublicKey) {
	if len(b) == 0 {
		return nil, nil
	}
	priv := decred.PrivKeyFromBytes(b)
	if priv == nil || priv.Key.IsZero() {
		return nil, nil
	}
	return priv, priv.PubKey()
}

// ParsePubKey parses a compressed, uncompressed, or hybrid public key.
func ParsePubKey(b []byte) (*PublicKey, error) {
	return decred.ParsePubKey(b)
}

// NewPrivateKey generates a random valid private key.
func NewPrivateKey() (*PrivateKey, error) {
	return decred.GeneratePrivateKey()
}
