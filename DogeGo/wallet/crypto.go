// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/scrypt"
)

const (
	scryptN        = 32768
	scryptR        = 8
	scryptP        = 1
	scryptKeyLen   = 32
)

type walletSecrets struct {
	PrivKeyHex           string                   `json:"privkey_hex"`
	HDSeedHex            string                   `json:"hd_seed_hex,omitempty"`
	ExtraPrivkeysHex     []string                 `json:"extra_privkeys_hex,omitempty"`
	PqCommitSeedHex      string                   `json:"pq_commit_seed_hex,omitempty"`
	PqTag                string                   `json:"pq_tag,omitempty"`
	PqSendCounter        uint64                   `json:"pq_send_counter,omitempty"`
	PqKeys               map[string]pqKeyPairJSON `json:"pq_keys,omitempty"`
	PqCommitmentsEnabled bool                     `json:"pq_commitments_enabled,omitempty"`
	PqCarrierEnabled     bool                     `json:"pq_carrier_enabled,omitempty"`
}

func deriveKey(passphrase string, salt []byte) ([]byte, error) {
	return scrypt.Key([]byte(passphrase), salt, scryptN, scryptR, scryptP, scryptKeyLen)
}

func sealSecrets(key []byte, sec walletSecrets) (nonce, ciphertext []byte, err error) {
	plain, err := json.Marshal(sec)
	if err != nil {
		return nil, nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, gcm.Seal(nil, nonce, plain, nil), nil
}

func openSecrets(key []byte, nonce, ciphertext []byte) (walletSecrets, error) {
	var sec walletSecrets
	block, err := aes.NewCipher(key)
	if err != nil {
		return sec, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return sec, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return sec, fmt.Errorf("wrong passphrase")
	}
	if err := json.Unmarshal(plain, &sec); err != nil {
		return sec, err
	}
	return sec, nil
}

func encryptSecrets(passphrase string, sec walletSecrets) (salt, nonce, ciphertext []byte, err error) {
	salt = make([]byte, 32)
	if _, err = rand.Read(salt); err != nil {
		return nil, nil, nil, err
	}
	key, err := deriveKey(passphrase, salt)
	if err != nil {
		return nil, nil, nil, err
	}
	nonce, ciphertext, err = sealSecrets(key, sec)
	return salt, nonce, ciphertext, err
}

func decryptSecrets(passphrase string, salt, nonce, ciphertext []byte) (walletSecrets, error) {
	key, err := deriveKey(passphrase, salt)
	if err != nil {
		return walletSecrets{}, err
	}
	return openSecrets(key, nonce, ciphertext)
}
