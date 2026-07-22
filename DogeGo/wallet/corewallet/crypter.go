// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package corewallet

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
)

// Core wallet encryption (src/wallet/crypter.cpp):
//   - Master key: derived from passphrase + salt via repeated SHA512 (EVP_BytesToKey,
//     nDerivationMethod 0), producing 32-byte AES key + 16-byte IV. AES-256-CBC decrypts
//     the 32-byte wallet master key stored in the mkey record.
//   - Per-key secret (ckey): AES-256-CBC with the master key; IV = SHA256(SHA256(pubkey))[:16].
const (
	wcKeySize   = 32
	wcIVSize    = 16
	wcBlockSize = 16
)

// masterKey mirrors CMasterKey.
type masterKey struct {
	cryptedKey       []byte
	salt             []byte
	derivationMethod uint32
	deriveIterations uint32
}

// deriveMasterKeyMaterial reproduces CCrypter::SetKeyFromPassphrase (nDerivationMethod 0).
func deriveMasterKeyMaterial(passphrase string, salt []byte, iterations uint32) (key, iv []byte, err error) {
	if iterations == 0 {
		return nil, nil, fmt.Errorf("zero derivation iterations")
	}
	if len(salt) != 8 {
		return nil, nil, fmt.Errorf("master key salt must be 8 bytes, got %d", len(salt))
	}
	// EVP_BytesToKey(EVP_aes_256_cbc, EVP_sha512, salt, pass, count) with no prior block.
	buf := make([]byte, 0, wcKeySize+wcIVSize)
	var prev []byte
	for len(buf) < wcKeySize+wcIVSize {
		h := sha512.New()
		h.Write(prev)
		h.Write([]byte(passphrase))
		h.Write(salt)
		digest := h.Sum(nil)
		// Additional rounds (Core loops nDeriveIterations-1 more SHA512 over the digest).
		for i := uint32(1); i < iterations; i++ {
			d := sha512.Sum512(digest)
			digest = d[:]
		}
		prev = digest
		buf = append(buf, digest...)
	}
	return buf[:wcKeySize], buf[wcKeySize : wcKeySize+wcIVSize], nil
}

// aesCBCDecrypt decrypts with AES-256-CBC and strips PKCS#7 padding.
func aesCBCDecrypt(key, iv, ciphertext []byte) ([]byte, error) {
	if len(key) != wcKeySize {
		return nil, fmt.Errorf("aes key must be %d bytes", wcKeySize)
	}
	if len(iv) != wcIVSize {
		return nil, fmt.Errorf("aes iv must be %d bytes", wcIVSize)
	}
	if len(ciphertext) == 0 || len(ciphertext)%wcBlockSize != 0 {
		return nil, fmt.Errorf("ciphertext not a multiple of block size")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, ciphertext)
	pad := int(out[len(out)-1])
	if pad <= 0 || pad > wcBlockSize || pad > len(out) {
		return nil, fmt.Errorf("bad PKCS#7 padding")
	}
	for _, b := range out[len(out)-pad:] {
		if int(b) != pad {
			return nil, fmt.Errorf("bad PKCS#7 padding")
		}
	}
	return out[:len(out)-pad], nil
}

// decryptMasterKey unlocks the 32-byte wallet master key from an mkey record.
func decryptMasterKey(mk masterKey, passphrase string) ([]byte, error) {
	if mk.derivationMethod != 0 {
		return nil, fmt.Errorf("unsupported master key derivation method %d", mk.derivationMethod)
	}
	key, iv, err := deriveMasterKeyMaterial(passphrase, mk.salt, mk.deriveIterations)
	if err != nil {
		return nil, err
	}
	plain, err := aesCBCDecrypt(key, iv, mk.cryptedKey)
	if err != nil {
		return nil, fmt.Errorf("wrong passphrase")
	}
	if len(plain) != wcKeySize {
		return nil, fmt.Errorf("wrong passphrase")
	}
	return plain, nil
}

// decryptSecret reproduces CCrypter::DecryptSecret: AES-256-CBC with IV = SHA256d(pubkey)[:16].
func decryptSecret(masterKeyMaterial, cryptedSecret, pubKey []byte) ([]byte, error) {
	if len(masterKeyMaterial) != wcKeySize {
		return nil, fmt.Errorf("master key material must be %d bytes", wcKeySize)
	}
	first := sha256.Sum256(pubKey)
	second := sha256.Sum256(first[:])
	iv := second[:wcIVSize]
	plain, err := aesCBCDecrypt(masterKeyMaterial, iv, cryptedSecret)
	if err != nil {
		return nil, err
	}
	if len(plain) != 32 {
		return nil, fmt.Errorf("decrypted secret is %d bytes, expected 32", len(plain))
	}
	return plain, nil
}

// parseMasterKeyValue decodes a serialized CMasterKey (mkey record value).
func parseMasterKeyValue(val []byte) (masterKey, error) {
	r := bytes.NewReader(val)
	var mk masterKey
	var err error
	if mk.cryptedKey, err = readBytesFrom(r); err != nil {
		return mk, fmt.Errorf("mkey cryptedKey: %w", err)
	}
	if mk.salt, err = readBytesFrom(r); err != nil {
		return mk, fmt.Errorf("mkey salt: %w", err)
	}
	if mk.derivationMethod, err = readUint32(r); err != nil {
		return mk, fmt.Errorf("mkey derivationMethod: %w", err)
	}
	if mk.deriveIterations, err = readUint32(r); err != nil {
		return mk, fmt.Errorf("mkey deriveIterations: %w", err)
	}
	return mk, nil
}
