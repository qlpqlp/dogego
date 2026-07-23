// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

//go:build raccoon_g && cgo

package pqcrypto

/*
#cgo CFLAGS: -I${SRCDIR}/raccoon_g/native -I${SRCDIR}/raccoon_g/shims/include -I${SRCDIR}/raccoon_g/shims/src -Wall -Wno-unused-function
#cgo LDFLAGS: -lgmp -lmpfr -lm
#cgo windows LDFLAGS: -lbcrypt
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include "raccoong.h"
#include "amalgamation.c"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func raccoonBackendName() string         { return "libdogecoin-raccoon_g" }
func raccoonLibdogecoinCompatible() bool { return C.raccoong_is_ready() != 0 }

func raccoonNativeKeygen(seed []byte) (pk, sk []byte, err error) {
	if C.raccoong_is_ready() == 0 {
		return nil, nil, fmt.Errorf("%w: raccoong_is_ready()=false", ErrExperimentalBackend)
	}
	if len(seed) != 32 {
		return nil, nil, fmt.Errorf("raccoon: seed must be 32 bytes")
	}
	pkLen := int(C.raccoong_pk_len())
	skLen := int(C.raccoong_sk_len())
	if pkLen == 0 || skLen == 0 {
		return nil, nil, fmt.Errorf("%w: raccoong sizes unavailable", ErrExperimentalBackend)
	}
	pk = make([]byte, pkLen)
	sk = make([]byte, skLen)
	var seedArr [32]byte
	copy(seedArr[:], seed)
	ok := C.raccoong_keygen_from_seed(
		(*C.uint8_t)(unsafe.Pointer(&seedArr[0])),
		(*C.uint8_t)(unsafe.Pointer(&pk[0])), C.size_t(pkLen),
		(*C.uint8_t)(unsafe.Pointer(&sk[0])), C.size_t(skLen),
	)
	if ok == 0 {
		return nil, nil, fmt.Errorf("raccoon: raccoong_keygen_from_seed failed")
	}
	return pk, sk, nil
}

func raccoonNativeSign(sk, message32 []byte) ([]byte, error) {
	if len(message32) != 32 {
		return nil, errWant32
	}
	if len(sk) == 0 {
		return nil, fmt.Errorf("raccoon: empty signing key")
	}
	maxSig := int(C.raccoong_sig_max_len())
	if maxSig == 0 {
		return nil, fmt.Errorf("%w: raccoong sig size unavailable", ErrExperimentalBackend)
	}
	sig := make([]byte, maxSig)
	sigLen := C.size_t(maxSig)
	ok := C.raccoong_sign(
		(*C.uint8_t)(unsafe.Pointer(&sk[0])), C.size_t(len(sk)),
		(*C.uint8_t)(unsafe.Pointer(&message32[0])), C.size_t(len(message32)),
		(*C.uint8_t)(unsafe.Pointer(&sig[0])), &sigLen,
	)
	if ok == 0 {
		return nil, fmt.Errorf("raccoon: raccoong_sign failed")
	}
	return sig[:sigLen], nil
}

func raccoonNativeVerify(pk, message32, sig []byte) bool {
	if len(message32) != 32 || len(pk) == 0 || len(sig) == 0 {
		return false
	}
	ok := C.raccoong_verify(
		(*C.uint8_t)(unsafe.Pointer(&pk[0])), C.size_t(len(pk)),
		(*C.uint8_t)(unsafe.Pointer(&message32[0])), C.size_t(len(message32)),
		(*C.uint8_t)(unsafe.Pointer(&sig[0])), C.size_t(len(sig)),
	)
	return ok != 0
}
