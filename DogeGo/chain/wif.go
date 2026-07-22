// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
)

// DecodeWIF decodes a wallet-import-format private key (Base58Check) for the expected version byte
// (SECRET_KEY in src/chainparams.cpp). Returns 32-byte secp256k1 secret scalar and whether the key is compressed.
func DecodeWIF(s string, version byte) (secret32 []byte, compressed bool, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false, fmt.Errorf("empty WIF")
	}
	raw, err := base58Decode(s)
	if err != nil {
		return nil, false, err
	}
	if len(raw) < 5 {
		return nil, false, fmt.Errorf("WIF too short")
	}
	payload := raw[:len(raw)-4]
	cs := raw[len(raw)-4:]
	h := sha256.Sum256(payload)
	h2 := sha256.Sum256(h[:])
	if !bytes.Equal(cs, h2[:4]) {
		return nil, false, fmt.Errorf("WIF checksum mismatch")
	}
	switch len(payload) {
	case 34:
		if payload[33] != 0x01 {
			return nil, false, fmt.Errorf("WIF invalid suffix")
		}
		compressed = true
		payload = payload[:33]
		fallthrough
	case 33:
		if payload[0] != version {
			return nil, false, fmt.Errorf("WIF version mismatch")
		}
		return append([]byte(nil), payload[1:33]...), compressed, nil
	default:
		return nil, false, fmt.Errorf("WIF unexpected length %d", len(payload))
	}
}

// EncodeWIF encodes a 32-byte secp256k1 secret as Base58Check WIF for the given SECRET_KEY version byte.
func EncodeWIF(secret32 []byte, version byte, compressed bool) (string, error) {
	if len(secret32) != 32 {
		return "", fmt.Errorf("secret must be 32 bytes")
	}
	var payload []byte
	payload = append(payload, version)
	payload = append(payload, secret32...)
	if compressed {
		payload = append(payload, 0x01)
	}
	h := sha256.Sum256(payload)
	h2 := sha256.Sum256(h[:])
	full := append(payload, h2[0:4]...)
	return base58Encode(full), nil
}
