// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pqcrypto

import (
	"crypto/rand"

	"github.com/cloudflare/circl/sign/dilithium/mode2"
)

// Dilithium2 implements CRYSTALS-Dilithium2 (ML-DSA-44 compatible sizes).
type Dilithium2 struct{}

func (Dilithium2) Name() string        { return "dilithium2" }
func (Dilithium2) OPReturnTag() string { return "DIL2" }
func (Dilithium2) CarrierTag8() string { return "DIL2FULL" }
func (Dilithium2) PartTotal() int      { return 3 }

func (Dilithium2) GenerateKey(seed []byte) (pk, sk []byte, err error) {
	if len(seed) == 32 {
		var s [mode2.SeedSize]byte
		copy(s[:], seed)
		pub, priv := mode2.NewKeyFromSeed(&s)
		return pub.Bytes(), priv.Bytes(), nil
	}
	pub, priv, err := mode2.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return pub.Bytes(), priv.Bytes(), nil
}

func (Dilithium2) Sign(sk, message32 []byte) (sig []byte, err error) {
	if len(message32) != 32 {
		return nil, errWant32
	}
	priv, err := mode2SchemeUnmarshalPrivate(sk)
	if err != nil {
		return nil, err
	}
	sig = make([]byte, mode2.Scheme().SignatureSize())
	mode2.SignTo(priv, message32, sig)
	return sig, nil
}

func (Dilithium2) Verify(pk, message32, sig []byte) bool {
	if len(message32) != 32 {
		return false
	}
	pub, err := mode2SchemeUnmarshalPublic(pk)
	if err != nil {
		return false
	}
	return mode2.Verify(pub, message32, sig)
}

func (Dilithium2) Commit(pk, sig []byte) [32]byte { return Commit(pk, sig) }

func mode2SchemeUnmarshalPrivate(sk []byte) (*mode2.PrivateKey, error) {
	priv := new(mode2.PrivateKey)
	if err := priv.UnmarshalBinary(sk); err != nil {
		return nil, err
	}
	return priv, nil
}

func mode2SchemeUnmarshalPublic(pk []byte) (*mode2.PublicKey, error) {
	pub := new(mode2.PublicKey)
	if err := pub.UnmarshalBinary(pk); err != nil {
		return nil, err
	}
	return pub, nil
}
