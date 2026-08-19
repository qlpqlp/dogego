// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pqcrypto

import (
	"crypto/rand"
	"fmt"

	"dogego/pqcrypto/raccoon_g"
)

// Raccoon-G-44 wire sizes â€” byte-exact with libdogecoin thrc.h / raccoong_*_len
// (dogecoinfoundation/libdogecoin@0.1.5-dev/src/raccoon_g) and Core green PR #8.
const (
	raccoonPKLen  = raccoon_g.PKBytes  // 16144
	raccoonSKLen  = raccoon_g.SKBytes  // 32272
	raccoonSigLen = raccoon_g.SigBytes // 20768
)

// RaccoonG44 is Raccoon-G-44 via the Dogecoin Foundation in-tree C port
// (pqcrypto/raccoon_g/native â€” same sources as libdogecoin src/raccoon_g and
// https://github.com/dogecoinfoundation/dogecoin/pull/8).
//
// The Foundation in-tree port was developed by Ed Tubbs
// (https://github.com/edtubbs Â· https://x.com/EdTubbs). See docs/CREDITS.md.
// There is no Go placeholder: crypto requires CGO_ENABLED=1 -tags raccoon_g
// (libgmp + libmpfr). See pqcrypto/raccoon_g/BUILD.md.
type RaccoonG44 struct{}

func (RaccoonG44) Name() string                { return "raccoon-g-44" }
func (RaccoonG44) OPReturnTag() string         { return "RCG4" }
func (RaccoonG44) CarrierTag8() string         { return "RCG4FULL" }
func (RaccoonG44) PartTotal() int              { return 24 }
func (RaccoonG44) Backend() string             { return raccoonBackendName() }
func (RaccoonG44) LibdogecoinCompatible() bool { return raccoonLibdogecoinCompatible() }
func (RaccoonG44) UpstreamRef() string         { return raccoon_g.UpstreamRef }
func (RaccoonG44) UpstreamAuthor() string      { return raccoon_g.UpstreamAuthor }

// Available reports whether the in-tree raccoong_* backend is linked and ready.
func (RaccoonG44) Available() bool { return raccoonLibdogecoinCompatible() }

func (RaccoonG44) GenerateKey(seed []byte) (pk, sk []byte, err error) {
	if len(seed) == 0 {
		seed = make([]byte, 32)
		if _, err := rand.Read(seed); err != nil {
			return nil, nil, err
		}
	}
	if len(seed) != 32 {
		return nil, nil, fmt.Errorf("raccoon: seed must be 32 bytes")
	}
	return raccoonNativeKeygen(seed)
}

func (RaccoonG44) Sign(sk, message32 []byte) (sig []byte, err error) {
	return raccoonNativeSign(sk, message32)
}

func (RaccoonG44) Verify(pk, message32, sig []byte) bool {
	return raccoonNativeVerify(pk, message32, sig)
}

func (RaccoonG44) Commit(pk, sig []byte) [32]byte { return Commit(pk, sig) }
