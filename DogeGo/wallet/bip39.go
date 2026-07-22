// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"crypto/sha256"
	"crypto/sha512"
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

//go:embed bip39_wordlist_en.txt
var bip39WordlistRaw string

var bip39Wordlist []string

func init() {
	lines := strings.Split(strings.TrimSpace(bip39WordlistRaw), "\n")
	bip39Wordlist = make([]string, 0, len(lines))
	for _, w := range lines {
		w = strings.TrimSpace(w)
		if w != "" {
			bip39Wordlist = append(bip39Wordlist, w)
		}
	}
	if len(bip39Wordlist) != 2048 {
		panic(fmt.Sprintf("bip39 wordlist: want 2048 words, got %d", len(bip39Wordlist)))
	}
}

// NormalizeMnemonic lowercases and collapses whitespace between BIP39 words.
func NormalizeMnemonic(mnemonic string) string {
	fields := strings.Fields(strings.TrimSpace(strings.ToLower(mnemonic)))
	return strings.Join(fields, " ")
}

// ValidateMnemonic checks word count, dictionary membership, and checksum.
func ValidateMnemonic(mnemonic string) error {
	words := strings.Fields(NormalizeMnemonic(mnemonic))
	switch len(words) {
	case 12, 15, 18, 21, 24:
	default:
		return fmt.Errorf("mnemonic must be 12, 15, 18, 21, or 24 words (got %d)", len(words))
	}
	if _, err := mnemonicWordsToEntropy(words); err != nil {
		return err
	}
	return nil
}

// MnemonicToSeed derives the 64-byte BIP39 seed (PBKDF2-HMAC-SHA512).
func MnemonicToSeed(mnemonic, passphrase string) ([]byte, error) {
	words := strings.Fields(NormalizeMnemonic(mnemonic))
	if _, err := mnemonicWordsToEntropy(words); err != nil {
		return nil, err
	}
	mnemonic = strings.Join(words, " ")
	salt := "mnemonic" + passphrase
	return pbkdf2.Key([]byte(mnemonic), []byte(salt), 2048, 64, sha512.New), nil
}

func mnemonicWordsToEntropy(words []string) ([]byte, error) {
	if len(words) == 0 {
		return nil, errors.New("empty mnemonic")
	}
	checksumBits := len(words) / 3
	entropyBits := len(words)*11 - checksumBits
	acc := big.NewInt(0)
	for _, word := range words {
		idx := bip39WordIndex(word)
		if idx < 0 {
			return nil, fmt.Errorf("unknown mnemonic word %q", word)
		}
		acc.Lsh(acc, 11)
		acc.Add(acc, big.NewInt(int64(idx)))
	}
	mask := new(big.Int).Lsh(big.NewInt(1), uint(checksumBits))
	mask.Sub(mask, big.NewInt(1))
	checksum := new(big.Int).And(acc, mask)
	acc.Rsh(acc, uint(checksumBits))
	entropyBytes := entropyBits / 8
	entropy := make([]byte, entropyBytes)
	if eb := acc.Bytes(); len(eb) > 0 {
		if len(eb) > entropyBytes {
			return nil, errors.New("mnemonic entropy overflow")
		}
		copy(entropy[entropyBytes-len(eb):], eb)
	}
	h := sha256.Sum256(entropy)
	want := h[0] >> (8 - checksumBits)
	if byte(checksum.Int64()) != want {
		return nil, errors.New("mnemonic checksum invalid")
	}
	return entropy, nil
}

func bip39WordIndex(word string) int {
	for i, w := range bip39Wordlist {
		if w == word {
			return i
		}
	}
	return -1
}
