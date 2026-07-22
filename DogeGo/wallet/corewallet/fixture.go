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
	"encoding/binary"
	"fmt"

	"dogego/wallet/bdb"
)

// WriteTestWalletDat writes a minimal Core wallet.dat with one unencrypted spend key.
// wifVer is the chain WIF version byte used when extracting dump lines (e.g. testnet vs mainnet).
func WriteTestWalletDat(path string, pubKey, secret []byte) error {
	kv := map[string][]byte{
		encodeWalletDBKey("key", pubKey): encodeWalletDBPrivVal(secret),
	}
	return bdb.WriteFixtureWallet(path, kv)
}

// WriteTestWalletDatMultiKey writes a minimal wallet.dat with two unencrypted spend keys.
func WriteTestWalletDatMultiKey(path string, pubKey1, secret1, pubKey2, secret2 []byte) error {
	kv := map[string][]byte{
		encodeWalletDBKey("key", pubKey1): encodeWalletDBPrivVal(secret1),
		encodeWalletDBKey("key", pubKey2): encodeWalletDBPrivVal(secret2),
	}
	return bdb.WriteFixtureWallet(path, kv)
}

// WriteTestWalletDatWithPools writes a wallet.dat with one spend key and multiple Core pool entries.
func WriteTestWalletDatWithPools(path string, pubKey, secret []byte, poolIndices []int64) error {
	kv := map[string][]byte{
		encodeWalletDBKey("key", pubKey): encodeWalletDBPrivVal(secret),
	}
	for _, idx := range poolIndices {
		kv[encodeWalletDBPoolKey(idx)] = encodeCompactBlob(pubKey)
	}
	return bdb.WriteFixtureWallet(path, kv)
}

// WriteTestWalletDatWithMixedPool writes a wallet.dat with one spend key, one matching pool entry, and one pool-only pubkey.
func WriteTestWalletDatWithMixedPool(path string, spendPub, secret, poolOnlyPub []byte, matchedIdx, unmatchedIdx int64) error {
	kv := map[string][]byte{
		encodeWalletDBKey("key", spendPub):    encodeWalletDBPrivVal(secret),
		encodeWalletDBPoolKey(matchedIdx):     encodeCompactBlob(spendPub),
		encodeWalletDBPoolKey(unmatchedIdx):   encodeCompactBlob(poolOnlyPub),
	}
	return bdb.WriteFixtureWallet(path, kv)
}

// WriteTestWalletDatWithPool writes a minimal wallet.dat with one spend key and one Core pool entry.
func WriteTestWalletDatWithPool(path string, pubKey, secret []byte, poolIndex int64) error {
	kv := map[string][]byte{
		encodeWalletDBKey("key", pubKey): encodeWalletDBPrivVal(secret),
		encodeWalletDBPoolKey(poolIndex): encodeCompactBlob(pubKey),
	}
	return bdb.WriteFixtureWallet(path, kv)
}

func encodeWalletDBPoolKey(index int64) string {
	var buf bytes.Buffer
	writeCompactString(&buf, "pool")
	var idx [8]byte
	for i := 0; i < 8; i++ {
		idx[i] = byte(index >> (8 * i))
	}
	buf.Write(idx[:])
	return buf.String()
}

// WriteTestEncryptedWalletDat writes a minimal encrypted Core wallet.dat (mkey + ckey).
func WriteTestEncryptedWalletDat(path string, pubKey, secret []byte, passphrase string) error {
	kv, err := buildEncryptedWalletKV(pubKey, secret, passphrase)
	if err != nil {
		return err
	}
	return bdb.WriteFixtureWallet(path, kv)
}

// WriteTestEncryptedDescriptorWalletDat writes a minimal encrypted descriptor-wallet fixture
// (mkey + walletdescriptorckey), matching Core descriptor-only encrypted wallets.
func WriteTestEncryptedDescriptorWalletDat(path string, pubKey, secret []byte, passphrase string) error {
	kv, err := buildEncryptedWalletKVWithType("walletdescriptorckey", pubKey, secret, passphrase)
	if err != nil {
		return err
	}
	return bdb.WriteFixtureWallet(path, kv)
}

func buildEncryptedWalletKV(pubKey, secret []byte, passphrase string) (map[string][]byte, error) {
	return buildEncryptedWalletKVWithType("ckey", pubKey, secret, passphrase)
}

func buildEncryptedWalletKVWithType(keyType string, pubKey, secret []byte, passphrase string) (map[string][]byte, error) {
	salt := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	iterations := uint32(2)
	master := bytes.Repeat([]byte{0x33}, wcKeySize)
	dk, iv, err := deriveMasterKeyMaterial(passphrase, salt, iterations)
	if err != nil {
		return nil, err
	}
	cryptedMaster, err := encryptAESCBCPKCS7(dk, iv, master)
	if err != nil {
		return nil, err
	}
	f1 := sha256.Sum256(pubKey)
	f2 := sha256.Sum256(f1[:])
	cryptedSecret, err := encryptAESCBCPKCS7(master, f2[:wcIVSize], secret)
	if err != nil {
		return nil, err
	}

	var mkKey bytes.Buffer
	writeCompactString(&mkKey, "mkey")
	mkKey.Write([]byte{1, 0, 0, 0})

	mkVal := encodeMasterKeyValue(cryptedMaster, salt, iterations)
	ckKey := encodeWalletDBKey(keyType, pubKey)

	return map[string][]byte{
		mkKey.String(): mkVal,
		ckKey:          encodeCompactBlob(cryptedSecret),
	}, nil
}

func encodeWalletDBKey(typ string, pubKey []byte) string {
	var buf bytes.Buffer
	writeCompactString(&buf, typ)
	writeCompactBytes(&buf, pubKey)
	return buf.String()
}

func encodeWalletDBPrivVal(secret []byte) []byte {
	return encodeCompactBlob(secret)
}

func encodeCompactBlob(b []byte) []byte {
	var buf bytes.Buffer
	writeCompactBytes(&buf, b)
	return buf.Bytes()
}

func encodeMasterKeyValue(cryptedKey, salt []byte, iterations uint32) []byte {
	var buf bytes.Buffer
	writeCompactBytes(&buf, cryptedKey)
	writeCompactBytes(&buf, salt)
	var method [4]byte
	var iters [4]byte
	binary.LittleEndian.PutUint32(iters[:], iterations)
	buf.Write(method[:])
	buf.Write(iters[:])
	return buf.Bytes()
}

func writeCompactString(w *bytes.Buffer, s string) {
	writeCompactBytes(w, []byte(s))
}

func writeCompactBytes(w *bytes.Buffer, b []byte) {
	n := len(b)
	switch {
	case n < 253:
		w.WriteByte(byte(n))
	case n <= 0xffff:
		w.WriteByte(253)
		w.WriteByte(byte(n))
		w.WriteByte(byte(n >> 8))
	default:
		panic("corewallet fixture: oversize blob")
	}
	w.Write(b)
}

func encryptAESCBCPKCS7(key, iv, plain []byte) ([]byte, error) {
	if len(key) != wcKeySize || len(iv) != wcIVSize {
		return nil, fmt.Errorf("bad key or iv size")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	pad := wcBlockSize - len(plain)%wcBlockSize
	padded := append(append([]byte(nil), plain...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return out, nil
}
