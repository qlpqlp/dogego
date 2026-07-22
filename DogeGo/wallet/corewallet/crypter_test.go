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
	"testing"
)

// aesCBCEncrypt is a test-only inverse of aesCBCDecrypt (PKCS#7 padding).
func aesCBCEncrypt(t *testing.T, key, iv, plain []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	pad := wcBlockSize - len(plain)%wcBlockSize
	padded := append(append([]byte(nil), plain...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return out
}

func TestDeriveMasterKeyDeterministic(t *testing.T) {
	salt := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	k1, iv1, err := deriveMasterKeyMaterial("hunter2", salt, 1)
	if err != nil {
		t.Fatal(err)
	}
	k2, iv2, err := deriveMasterKeyMaterial("hunter2", salt, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) || !bytes.Equal(iv1, iv2) {
		t.Fatal("derivation not deterministic")
	}
	if len(k1) != wcKeySize || len(iv1) != wcIVSize {
		t.Fatalf("key=%d iv=%d", len(k1), len(iv1))
	}
	kOther, _, _ := deriveMasterKeyMaterial("hunter3", salt, 1)
	if bytes.Equal(k1, kOther) {
		t.Fatal("different passphrase produced same key")
	}
}

func TestDecryptSecretRoundTrip(t *testing.T) {
	master := bytes.Repeat([]byte{0x11}, wcKeySize)
	pub := append([]byte{0x02}, bytes.Repeat([]byte{0xaa}, 32)...)
	secret := bytes.Repeat([]byte{0xcd}, 32)

	first := sha256.Sum256(pub)
	second := sha256.Sum256(first[:])
	crypted := aesCBCEncrypt(t, master, second[:wcIVSize], secret)

	got, err := decryptSecret(master, crypted, pub)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, secret) {
		t.Fatalf("secret mismatch: %x", got)
	}
}

func TestDecryptMasterKeyRoundTrip(t *testing.T) {
	salt := []byte{9, 8, 7, 6, 5, 4, 3, 2}
	passphrase := "correct horse battery staple"
	iterations := uint32(3)
	master := bytes.Repeat([]byte{0x22}, wcKeySize)

	key, iv, err := deriveMasterKeyMaterial(passphrase, salt, iterations)
	if err != nil {
		t.Fatal(err)
	}
	mk := masterKey{
		cryptedKey:       aesCBCEncrypt(t, key, iv, master),
		salt:             salt,
		derivationMethod: 0,
		deriveIterations: iterations,
	}
	got, err := decryptMasterKey(mk, passphrase)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, master) {
		t.Fatalf("master mismatch: %x", got)
	}
	if _, err := decryptMasterKey(mk, "wrong"); err == nil {
		t.Fatal("expected wrong passphrase error")
	}
}

// TestDecryptCryptedKeysEndToEnd builds a synthetic encrypted wallet scan result and recovers a WIF.
func TestDecryptCryptedKeysEndToEnd(t *testing.T) {
	salt := []byte{1, 1, 2, 3, 5, 8, 13, 21}
	passphrase := "s3cret"
	iterations := uint32(2)
	master := bytes.Repeat([]byte{0x33}, wcKeySize)

	key, iv, err := deriveMasterKeyMaterial(passphrase, salt, iterations)
	if err != nil {
		t.Fatal(err)
	}
	mk := masterKey{
		cryptedKey:       aesCBCEncrypt(t, key, iv, master),
		salt:             salt,
		derivationMethod: 0,
		deriveIterations: iterations,
	}

	pub := append([]byte{0x03}, bytes.Repeat([]byte{0xbb}, 32)...)
	secret := bytes.Repeat([]byte{0x44}, 32)
	first := sha256.Sum256(pub)
	second := sha256.Sum256(first[:])
	crypted := aesCBCEncrypt(t, master, second[:wcIVSize], secret)

	res := &ExtractResult{
		Encrypted:   true,
		Lines:       []string{"# header"},
		masterKeys:  []masterKey{mk},
		cryptedKeys: []cryptedKeyRecord{{pubKey: pub, cryptedSecret: crypted}},
	}
	n, err := decryptCryptedKeys(res, 0x9e, passphrase)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("recovered %d keys", n)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("expected 1 WIF line, got lines=%v", res.Lines)
	}
	if _, err := decryptCryptedKeys(res, 0x9e, "wrong"); err == nil {
		t.Fatal("expected wrong passphrase error")
	}
}

func TestParseMasterKeyValueRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	writeBytes(&buf, bytes.Repeat([]byte{0xee}, 48)) // cryptedKey
	writeBytes(&buf, []byte{1, 2, 3, 4, 5, 6, 7, 8}) // salt
	buf.Write([]byte{0, 0, 0, 0})                    // derivationMethod = 0
	buf.Write([]byte{0x10, 0x27, 0, 0})              // deriveIterations = 10000

	mk, err := parseMasterKeyValue(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(mk.cryptedKey) != 48 || len(mk.salt) != 8 {
		t.Fatalf("cryptedKey=%d salt=%d", len(mk.cryptedKey), len(mk.salt))
	}
	if mk.derivationMethod != 0 || mk.deriveIterations != 10000 {
		t.Fatalf("method=%d iters=%d", mk.derivationMethod, mk.deriveIterations)
	}
}
