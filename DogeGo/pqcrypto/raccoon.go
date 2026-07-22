// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pqcrypto

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/sha3"
)

// Raccoon-G-44 BIP profile sizes (lattice-hd-wallets wire format).
const (
	raccoonPKLen  = 16144
	raccoonSigLen = 20768
)

// RaccoonG44 implements Raccoon-G-44 carrier metadata and an experimental
// deterministic sign/verify backend for DogeGo Phase-1 tooling. It uses the BIP
// message binding mu = SHAKE256(SHAKE256(pk).read(32) || sighash32).read(32).
// This is NOT byte-compatible with libdogecoin lattice Raccoon-G-44 signatures.
type RaccoonG44 struct{}

func (RaccoonG44) Name() string        { return "raccoon-g-44" }
func (RaccoonG44) OPReturnTag() string { return "RCG4" }
func (RaccoonG44) CarrierTag8() string { return "RCG4FULL" }
func (RaccoonG44) PartTotal() int      { return 24 }

func (RaccoonG44) GenerateKey(seed []byte) (pk, sk []byte, err error) {
	if len(seed) == 0 {
		seed = make([]byte, 32)
		if _, err := rand.Read(seed); err != nil {
			return nil, nil, err
		}
	}
	h := sha256.Sum256(append([]byte("dogego/raccoon-g44/sk/v1/"), seed...))
	sk = append([]byte(nil), h[:]...)
	root := sha256.Sum256(sk)
	pk = raccoonExpandWire(root[:], raccoonPKLen)
	copy(pk[:32], root[:])
	return pk, sk, nil
}

func (RaccoonG44) Sign(sk, message32 []byte) (sig []byte, err error) {
	if len(message32) != 32 {
		return nil, errWant32
	}
	if len(sk) == 0 {
		return nil, fmt.Errorf("raccoon: empty signing key")
	}
	root := sha256.Sum256(sk)
	pk := raccoonExpandWire(root[:], raccoonPKLen)
	copy(pk[:32], root[:])
	mu, err := raccoonMessageMu(pk, message32)
	if err != nil {
		return nil, err
	}
	return raccoonExpandSig(root[:], mu), nil
}

func (RaccoonG44) Verify(pk, message32, sig []byte) bool {
	if len(pk) != raccoonPKLen || len(sig) != raccoonSigLen || len(message32) != 32 {
		return false
	}
	mu, err := raccoonMessageMu(pk, message32)
	if err != nil {
		return false
	}
	want := raccoonExpandSig(pk[:32], mu)
	return string(sig) == string(want)
}

func (RaccoonG44) Commit(pk, sig []byte) [32]byte { return Commit(pk, sig) }

func raccoonExpandWire(root []byte, n int) []byte {
	out := make([]byte, n)
	off := 0
	ctr := 0
	for off < n {
		h := sha256.New()
		_, _ = h.Write([]byte("dogego/raccoon-g44/wire/"))
		_, _ = h.Write(root)
		_, _ = h.Write([]byte{byte(ctr), byte(ctr >> 8), byte(ctr >> 16), byte(ctr >> 24)})
		block := h.Sum(nil)
		ctr++
		copied := copy(out[off:], block)
		off += copied
	}
	return out
}

func raccoonExpandSig(root, mu []byte) []byte {
	return raccoonExpandWire(append(append([]byte("sig/"), root...), mu...), raccoonSigLen)
}

func raccoonMessageMu(pk, sighash32 []byte) ([]byte, error) {
	if len(sighash32) != 32 {
		return nil, fmt.Errorf("raccoon: sighash32 must be 32 bytes")
	}
	h1 := sha3.NewShake256()
	_, _ = h1.Write(pk)
	pkHash := make([]byte, 32)
	if _, err := h1.Read(pkHash); err != nil {
		return nil, err
	}
	h2 := sha3.NewShake256()
	_, _ = h2.Write(pkHash)
	_, _ = h2.Write(sighash32)
	mu := make([]byte, 32)
	if _, err := h2.Read(mu); err != nil {
		return nil, err
	}
	return mu, nil
}
