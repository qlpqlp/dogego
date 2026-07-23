// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pqcrypto

import (
	"crypto"
	"crypto/rand"

	"github.com/pornin/go-fn-dsa/fndsa"
)

const falconLogN = 9 // degree 512

// Falcon512 implements Falcon-512 / FN-DSA degree-512 via go-fn-dsa.
type Falcon512 struct{}

func (Falcon512) Name() string                { return "falcon-512" }
func (Falcon512) OPReturnTag() string         { return "FLC1" }
func (Falcon512) CarrierTag8() string         { return "FLC1FULL" }
func (Falcon512) PartTotal() int              { return 2 } // pk||sig may exceed one 3×520-byte part
func (Falcon512) Backend() string             { return "go-fn-dsa" }
func (Falcon512) LibdogecoinCompatible() bool { return false }

func (Falcon512) GenerateKey(seed []byte) (pk, sk []byte, err error) {
	_ = seed
	sk, pk, err = fndsa.KeyGen(falconLogN, rand.Reader)
	return pk, sk, err
}

func (Falcon512) Sign(sk, message32 []byte) (sig []byte, err error) {
	if len(message32) != 32 {
		return nil, errWant32
	}
	return fndsa.Sign(rand.Reader, sk, fndsa.DomainContext(""), crypto.Hash(0), message32)
}

func (Falcon512) Verify(pk, message32, sig []byte) bool {
	if len(message32) != 32 {
		return false
	}
	return fndsa.Verify(pk, fndsa.DomainContext(""), crypto.Hash(0), message32, sig)
}

func (Falcon512) Commit(pk, sig []byte) [32]byte { return Commit(pk, sig) }
