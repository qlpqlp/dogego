// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package pqcrypto provides Phase-1 post-quantum sign/verify backends for
// DogeGo verifier-side PQ carrier workflows (not consensus-enforced).
// Falcon-512 and Dilithium2 use pure-Go libraries. Raccoon-G-44 is the
// Foundation in-tree C port under pqcrypto/raccoon_g (by Ed Tubbs;
// CGO_ENABLED=1 -tags raccoon_g). See docs/CREDITS.md.
package pqcrypto

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

// ErrUnknownScheme is returned for unsupported PQ tags.
var ErrUnknownScheme = errors.New("pqcrypto: unknown scheme")

// ErrExperimentalBackend is returned when a backend is not compiled in.
var ErrExperimentalBackend = errors.New("pqcrypto: experimental backend unavailable")

// Scheme is a Phase-1 PQ algorithm backend.
type Scheme interface {
	Name() string
	OPReturnTag() string
	CarrierTag8() string
	GenerateKey(seed []byte) (pk, sk []byte, err error)
	Sign(sk, message32 []byte) (sig []byte, err error)
	Verify(pk, message32, sig []byte) bool
	Commit(pk, sig []byte) [32]byte
	PartTotal() int
	// Backend names the crypto implementation (e.g. circl, go-fn-dsa, libdogecoin-raccoon_g).
	Backend() string
	// LibdogecoinCompatible reports byte-compatibility with Foundation libdogecoin PQC APIs.
	LibdogecoinCompatible() bool
}

// Commit computes SHA256(pk || sig).
func Commit(pk, sig []byte) [32]byte {
	h := sha256.New()
	_, _ = h.Write(pk)
	_, _ = h.Write(sig)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// ByOPReturnTag returns a scheme for FLC1/DIL2/RCG4.
func ByOPReturnTag(tag string) (Scheme, error) {
	switch tag {
	case "FLC1":
		return Falcon512{}, nil
	case "DIL2":
		return Dilithium2{}, nil
	case "RCG4":
		return RaccoonG44{}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownScheme, tag)
	}
}

// ByCarrierTag8 returns a scheme for FLC1FULL/DIL2FULL/RCG4FULL.
func ByCarrierTag8(tag8 string) (Scheme, error) {
	switch tag8 {
	case "FLC1FULL":
		return Falcon512{}, nil
	case "DIL2FULL":
		return Dilithium2{}, nil
	case "RCG4FULL":
		return RaccoonG44{}, nil
	default:
		return nil, fmt.Errorf("%w: carrier tag8 %q", ErrUnknownScheme, tag8)
	}
}

// AllSchemes returns registered Phase-1 backends.
func AllSchemes() []Scheme {
	return []Scheme{Falcon512{}, Dilithium2{}, RaccoonG44{}}
}

// DeriveSeed hashes arbitrary material into a 32-byte PQ key seed.
func DeriveSeed(material []byte, label string) [32]byte {
	h := sha256.New()
	_, _ = h.Write([]byte(label))
	_, _ = h.Write(material)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
