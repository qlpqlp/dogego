// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"bytes"
	"crypto/aes"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/secp256k1"

	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/scrypt"
	"golang.org/x/text/unicode/norm"
)

const (
	bip38ScryptN = 16384
	bip38ScryptR = 8
	bip38ScryptP = 8
)

// DecryptBIP38 decrypts a BIP38 encrypted private key (6P…) with passphrase.
// pkhVer is the P2PKH version byte used to verify the embedded address hash (Dogecoin mainnet 0x1e, etc.).
func DecryptBIP38(encrypted, passphrase string, pkhVer byte) (secret []byte, compressed bool, err error) {
	raw, err := chain.DecodeBase58CheckBytes(strings.TrimSpace(encrypted))
	if err != nil {
		return nil, false, fmt.Errorf("invalid BIP38 key: %w", err)
	}
	if len(raw) != 39 {
		return nil, false, fmt.Errorf("invalid BIP38 payload length %d", len(raw))
	}
	pass := norm.NFC.String(passphrase)
	switch {
	case raw[0] == 0x01 && raw[1] == 0x42:
		return decryptBIP38NonEC(raw, pass, pkhVer)
	case raw[0] == 0x01 && raw[1] == 0x43:
		return decryptBIP38EC(raw, pass, pkhVer)
	default:
		return nil, false, errors.New("unsupported BIP38 key type")
	}
}

func decryptBIP38NonEC(raw []byte, passphrase string, pkhVer byte) ([]byte, bool, error) {
	if raw[2]&0x04 != 0 {
		return nil, false, errors.New("invalid non-EC BIP38 flags")
	}
	compressed := raw[2]&0x20 != 0
	addressHash := raw[3:7]
	enc1 := raw[7:23]
	enc2 := raw[23:39]
	derived, err := scrypt.Key([]byte(passphrase), addressHash, bip38ScryptN, bip38ScryptR, bip38ScryptP, 64)
	if err != nil {
		return nil, false, err
	}
	plain, err := bip38AESDecrypt(derived[32:], enc1, enc2, derived[:32])
	if err != nil {
		return nil, false, err
	}
	if err := verifyBIP38AddressHash(plain, compressed, addressHash, pkhVer); err != nil {
		return nil, false, err
	}
	return plain, compressed, nil
}

func decryptBIP38EC(raw []byte, passphrase string, pkhVer byte) ([]byte, bool, error) {
	if raw[2]&0x10 != 0 || raw[2]&0x08 != 0 {
		return nil, false, errors.New("unsupported BIP38 multisig flags")
	}
	addressHash := raw[3:7]
	secret, compressed, err := decryptBIP38ECSecret(raw, passphrase)
	if err != nil {
		return nil, false, err
	}
	if err := verifyBIP38AddressHash(secret, compressed, addressHash, pkhVer); err != nil {
		return nil, false, err
	}
	return secret, compressed, nil
}

func decryptBIP38ECSecret(raw []byte, passphrase string) ([]byte, bool, error) {
	flag := raw[2]
	compressed := flag&0x20 != 0
	if flag&0x10 != 0 || flag&0x08 != 0 {
		return nil, false, errors.New("unsupported BIP38 multisig flags")
	}
	hasLotSeq := flag&0x04 != 0
	addressHash := raw[3:7]
	ownerEntropy := raw[7:15]
	encPart1 := raw[15:23]
	encPart2 := raw[23:39]

	pass := norm.NFC.String(passphrase)
	ownerSalt := ownerEntropy
	if hasLotSeq {
		ownerSalt = ownerEntropy[:4]
	}
	preFactor, err := scrypt.Key([]byte(pass), ownerSalt, bip38ScryptN, bip38ScryptR, bip38ScryptP, 32)
	if err != nil {
		return nil, false, err
	}
	passFactor, err := bip38ECPassFactor(preFactor, ownerEntropy, hasLotSeq)
	if err != nil {
		return nil, false, err
	}
	priv, _ := secp256k1.PrivKeyFromBytes(passFactor)
	if priv == nil {
		return nil, false, errors.New("invalid passfactor")
	}
	passpoint := priv.PubKey().SerializeCompressed()

	seedBPass, err := scrypt.Key(passpoint, append(append([]byte{}, addressHash...), ownerEntropy...), 1024, 1, 1, 64)
	if err != nil {
		return nil, false, err
	}
	derivedHalf1 := seedBPass[:32]
	derivedHalf2 := seedBPass[32:64]

	decPart2, err := aesDecryptECB(derivedHalf2, encPart2)
	if err != nil {
		return nil, false, err
	}
	tmp := xorBytes(decPart2, derivedHalf1[16:32])
	seedBPart2 := tmp[8:16]

	block1 := append(append([]byte{}, encPart1...), tmp[:8]...)
	decPart1, err := aesDecryptECB(derivedHalf2, block1)
	if err != nil {
		return nil, false, err
	}
	seedBPart1 := xorBytes(decPart1, derivedHalf1[:16])
	seedB := append(seedBPart1, seedBPart2...)

	h := sha256.Sum256(seedB)
	factorBHash := sha256.Sum256(h[:])
	fb, _ := secp256k1.PrivKeyFromBytes(factorBHash[:])
	if fb == nil {
		return nil, false, errors.New("invalid factorb")
	}
	var product secp256k1.ModNScalar
	product.Mul2(&priv.Key, &fb.Key)
	out := product.Bytes()
	return out[:], compressed, nil
}

// bip38ECPassFactor derives the EC-multiply passfactor (BIP38); lot/sequence uses double-SHA256(prefactor||ownerentropy).
func bip38ECPassFactor(preFactor, ownerEntropy []byte, hasLotSeq bool) ([]byte, error) {
	if !hasLotSeq {
		return append([]byte(nil), preFactor...), nil
	}
	buf := append(append([]byte(nil), preFactor...), ownerEntropy...)
	h := sha256.Sum256(buf)
	h2 := sha256.Sum256(h[:])
	return h2[:], nil
}

func xorBytes(a, b []byte) []byte {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = a[i] ^ b[i]
	}
	return out
}

func bip38AESDecrypt(aesKey, enc1, enc2, xorKey []byte) ([]byte, error) {
	p1, err := aesDecryptECB(aesKey, enc1)
	if err != nil {
		return nil, err
	}
	p2, err := aesDecryptECB(aesKey, enc2)
	if err != nil {
		return nil, err
	}
	out := append(p1, p2...)
	for i := range out {
		out[i] ^= xorKey[i]
	}
	return out, nil
}

func aesDecryptECB(key, block []byte) ([]byte, error) {
	if len(block)%aes.BlockSize != 0 {
		return nil, errors.New("invalid AES block size")
	}
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(block))
	for i := 0; i < len(block); i += aes.BlockSize {
		c.Decrypt(out[i:i+aes.BlockSize], block[i:i+aes.BlockSize])
	}
	return out, nil
}

func verifyBIP38AddressHash(secret []byte, compressed bool, addressHash []byte, pkhVer byte) error {
	priv, _ := secp256k1.PrivKeyFromBytes(secret)
	if priv == nil {
		return errors.New("invalid decrypted key")
	}
	addr := bip38Address(priv, compressed, pkhVer)
	if bytes.Equal(bip38AddressHash(addr), addressHash) {
		return nil
	}
	// Allow Bitcoin-mainnet-encoded paper wallets (version 0x00).
	if pkhVer != 0x00 {
		addr = bip38Address(priv, compressed, 0x00)
		if bytes.Equal(bip38AddressHash(addr), addressHash) {
			return nil
		}
	}
	return errors.New("BIP38 passphrase incorrect or wrong network")
}

func bip38Address(priv *secp256k1.PrivateKey, compressed bool, pkhVer byte) string {
	if compressed {
		return p2pkh(pkhVer, priv.PubKey())
	}
	pub := priv.PubKey().SerializeUncompressed()
	h := sha256.Sum256(pub)
	r := ripemd160.New()
	_, _ = r.Write(h[:])
	return chain.Base58CheckEncode(pkhVer, r.Sum(nil))
}

func bip38AddressHash(address string) []byte {
	h := sha256.Sum256([]byte(address))
	h2 := sha256.Sum256(h[:])
	return h2[:4]
}
