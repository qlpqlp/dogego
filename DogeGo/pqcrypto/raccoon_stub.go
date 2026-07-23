// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

//go:build !raccoon_g || !cgo

package pqcrypto

import "fmt"

func raccoonBackendName() string         { return "unavailable" }
func raccoonLibdogecoinCompatible() bool { return false }

func raccoonNativeKeygen(seed []byte) (pk, sk []byte, err error) {
	return nil, nil, fmt.Errorf("%w: Raccoon-G-44 requires CGO_ENABLED=1 -tags raccoon_g (libgmp+libmpfr); see pqcrypto/raccoon_g/BUILD.md", ErrExperimentalBackend)
}

func raccoonNativeSign(sk, message32 []byte) ([]byte, error) {
	return nil, fmt.Errorf("%w: Raccoon-G-44 in-tree backend not linked (CGO_ENABLED=1 -tags raccoon_g)", ErrExperimentalBackend)
}

func raccoonNativeVerify(pk, message32, sig []byte) bool { return false }
