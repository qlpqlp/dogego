// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ecdsa_test

import (
	"crypto/sha256"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"
)

func TestSignVerifyDER(t *testing.T) {
	priv, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	msg := sha256.Sum256([]byte("such verify"))
	sig := ecdsa.Sign(priv, msg[:])
	der := sig.Serialize()
	parsed, err := ecdsa.ParseDERSignature(der)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.Verify(msg[:], priv.PubKey()) {
		t.Fatal("verify failed")
	}
}
