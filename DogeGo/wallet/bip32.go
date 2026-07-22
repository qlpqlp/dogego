// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"

	"dogego/secp256k1"
	"golang.org/x/crypto/ripemd160"
)

const bip32SeedKey = "Bitcoin seed"

// extendedKey holds a BIP32 private node (32-byte key + 32-byte chain code).
type extendedKey struct {
	key       *secp256k1.PrivateKey
	chainCode [32]byte
}

func masterKeyFromSeed(seed []byte) (*extendedKey, error) {
	if len(seed) < 16 || len(seed) > 64 {
		return nil, fmt.Errorf("seed length %d out of range", len(seed))
	}
	mac := hmac.New(sha512.New, []byte(bip32SeedKey))
	_, _ = mac.Write(seed)
	I := mac.Sum(nil)
	IL, IR := I[:32], I[32:]
	priv, pub := secp256k1.PrivKeyFromBytes(IL)
	if priv == nil || pub == nil {
		return nil, errors.New("invalid master key")
	}
	return &extendedKey{key: priv, chainCode: copy32(IR)}, nil
}

func (k *extendedKey) child(i uint32) (*extendedKey, error) {
	if k == nil || k.key == nil {
		return nil, errors.New("nil key")
	}
	var data []byte
	if i&0x80000000 != 0 {
		data = append([]byte{0x00}, k.key.Serialize()...)
	} else {
		data = append([]byte{0x02}, k.key.PubKey().SerializeCompressed()...)
	}
	data = append(data, ser32(i)...)
	mac := hmac.New(sha512.New, k.chainCode[:])
	_, _ = mac.Write(data)
	I := mac.Sum(nil)
	IL, IR := I[:32], I[32:]
	childScalar := new(secp256k1.ModNScalar)
	childScalar.SetByteSlice(IL)
	sum := new(secp256k1.ModNScalar)
	sum.Add2(&k.key.Key, childScalar)
	if sum.IsZero() {
		return nil, errors.New("invalid child key")
	}
	sk := sum.Bytes()
	priv, pub := secp256k1.PrivKeyFromBytes(sk[:])
	if priv == nil || pub == nil {
		return nil, errors.New("invalid child key")
	}
	return &extendedKey{key: priv, chainCode: copy32(IR)}, nil
}

func derivePath(seed []byte, path []uint32) (*extendedKey, error) {
	k, err := masterKeyFromSeed(seed)
	if err != nil {
		return nil, err
	}
	for _, idx := range path {
		k, err = k.child(idx)
		if err != nil {
			return nil, err
		}
	}
	return k, nil
}

func ser32(i uint32) []byte {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], i)
	return b[:]
}

func copy32(b []byte) [32]byte {
	var out [32]byte
	copy(out[:], b)
	return out
}

// MasterKeyFingerprint returns the BIP32 master key fingerprint (first 4 bytes of HASH160(compressed pubkey)).
func MasterKeyFingerprint(seed []byte) (uint32, error) {
	k, err := masterKeyFromSeed(seed)
	if err != nil {
		return 0, err
	}
	pub := k.key.PubKey().SerializeCompressed()
	h := sha256.Sum256(pub)
	r := ripemd160.New()
	_, _ = r.Write(h[:])
	sum := r.Sum(nil)
	if len(sum) < 4 {
		return 0, errors.New("fingerprint hash too short")
	}
	return binary.BigEndian.Uint32(sum[:4]), nil
}
