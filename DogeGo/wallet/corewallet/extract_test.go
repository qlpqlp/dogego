// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package corewallet

import (
	"bytes"
	"crypto/sha256"
	"strings"
	"testing"
)

func TestReadPrivKeyRaw32(t *testing.T) {
	raw := append([]byte{32}, make([]byte, 32)...)
	for i := range raw[1:] {
		raw[i+1] = byte(i)
	}
	sec, _, err := readPrivKey(raw)
	if err != nil || len(sec) != 32 {
		t.Fatalf("sec=%v err=%v", sec, err)
	}
}

func TestReadPrivKeyLegacyDER(t *testing.T) {
	der := []byte{0x30, 0x81, 0xd3, 0x02, 0x01, 0x01, 0x04, 0x20}
	der = append(der, bytes.Repeat([]byte{0xab}, 32)...)
	sec, _, err := readPrivKey(append([]byte{byte(len(der))}, der...))
	if err != nil || len(sec) != 32 || sec[0] != 0xab {
		t.Fatalf("sec=%v err=%v", sec, err)
	}
}

func TestScanWalletKVExtractsKey(t *testing.T) {
	pub := append([]byte{0x02}, bytes.Repeat([]byte{0xab}, 32)...)
	var keyBuf bytes.Buffer
	writeString(&keyBuf, "key")
	writeBytes(&keyBuf, pub)
	secret := bytes.Repeat([]byte{0xcd}, 32)
	var valBuf bytes.Buffer
	writeBytes(&valBuf, secret)

	kv := map[string][]byte{keyBuf.String(): valBuf.Bytes()}
	res := scanWalletKV(kv, 0x9e)
	if res.KeyCount != 1 || res.Encrypted || res.WatchCount != 0 {
		t.Fatalf("res=%#v", res)
	}
}

func TestScanWalletKVDescriptorKey(t *testing.T) {
	pub := append([]byte{0x03}, bytes.Repeat([]byte{0x11}, 32)...)
	var keyBuf bytes.Buffer
	writeString(&keyBuf, "walletdescriptorkey")
	writeBytes(&keyBuf, pub)
	secret := bytes.Repeat([]byte{0xee}, 32)
	var valBuf bytes.Buffer
	writeBytes(&valBuf, secret)

	kv := map[string][]byte{keyBuf.String(): valBuf.Bytes()}
	res := scanWalletKV(kv, 0x9e)
	if res.KeyCount != 1 || res.Encrypted {
		t.Fatalf("res=%#v", res)
	}
}

func TestScanWalletKVPoolRecord(t *testing.T) {
	pub := append([]byte{0x02}, bytes.Repeat([]byte{0x99}, 32)...)
	var poolKey bytes.Buffer
	writeString(&poolKey, "pool")
	poolKey.Write([]byte{5, 0, 0, 0, 0, 0, 0, 0})
	var poolVal bytes.Buffer
	writeBytes(&poolVal, pub)

	kv := map[string][]byte{poolKey.String(): poolVal.Bytes()}
	res := scanWalletKV(kv, 0x9e)
	if res.PoolCount != 1 || !res.poolIndexSeen || res.PoolIndexMin != 5 || res.PoolIndexMax != 5 {
		t.Fatalf("pool=%d min=%d max=%d res=%#v", res.PoolCount, res.PoolIndexMin, res.PoolIndexMax, res)
	}
	if res.PoolPubkeys != 1 {
		t.Fatalf("pool_pubkeys=%d", res.PoolPubkeys)
	}
}

func TestScanWalletKVPoolKeysMatched(t *testing.T) {
	pub := append([]byte{0x02}, bytes.Repeat([]byte{0x99}, 32)...)
	var keyBuf bytes.Buffer
	writeString(&keyBuf, "key")
	writeBytes(&keyBuf, pub)
	secret := bytes.Repeat([]byte{0xcd}, 32)
	var valBuf bytes.Buffer
	writeBytes(&valBuf, secret)
	var poolKey bytes.Buffer
	writeString(&poolKey, "pool")
	poolKey.Write([]byte{2, 0, 0, 0, 0, 0, 0, 0})
	var poolVal bytes.Buffer
	writeBytes(&poolVal, pub)

	kv := map[string][]byte{
		keyBuf.String():  valBuf.Bytes(),
		poolKey.String(): poolVal.Bytes(),
	}
	res := scanWalletKV(kv, 0x9e)
	if res.PoolKeysMatched != 1 {
		t.Fatalf("pool_keys_matched=%d res=%#v", res.PoolKeysMatched, res)
	}
	out := &ProbeResult{PoolPubkeys: res.PoolPubkeys, PoolKeysMatched: res.PoolKeysMatched}
	finalizeProbePoolStats(out)
	if out.PoolKeysUnmatched != 0 {
		t.Fatalf("pool_keys_unmatched=%d", out.PoolKeysUnmatched)
	}
}

func TestScanWalletKVPoolKeysUnmatched(t *testing.T) {
	pubMatch := append([]byte{0x02}, bytes.Repeat([]byte{0x99}, 32)...)
	pubOther := append([]byte{0x02}, bytes.Repeat([]byte{0x88}, 32)...)
	secret := bytes.Repeat([]byte{0xcd}, 32)
	kv := map[string][]byte{
		encodeWalletDBKey("key", pubMatch): encodeWalletDBPrivVal(secret),
		encodeWalletDBPoolKey(1):           encodeCompactBlob(pubMatch),
		encodeWalletDBPoolKey(2):           encodeCompactBlob(pubOther),
	}
	res := scanWalletKV(kv, 0x9e)
	if res.PoolPubkeys != 2 || res.PoolKeysMatched != 1 {
		t.Fatalf("pubkeys=%d matched=%d", res.PoolPubkeys, res.PoolKeysMatched)
	}
	out := &ProbeResult{PoolPubkeys: res.PoolPubkeys, PoolKeysMatched: res.PoolKeysMatched}
	finalizeProbePoolStats(out)
	if out.PoolKeysUnmatched != 1 {
		t.Fatalf("pool_keys_unmatched=%d", out.PoolKeysUnmatched)
	}
}

func TestScanWalletKVPoolEntriesTruncated(t *testing.T) {
	pub := append([]byte{0x02}, bytes.Repeat([]byte{0x99}, 32)...)
	secret := bytes.Repeat([]byte{0xcd}, 32)
	kv := map[string][]byte{
		encodeWalletDBKey("key", pub): encodeWalletDBPrivVal(secret),
	}
	for i := int64(0); i < maxPoolEntriesProbe+1; i++ {
		kv[encodeWalletDBPoolKey(i)] = encodeCompactBlob(pub)
	}
	res := scanWalletKV(kv, 0x9e)
	if res.PoolCount != maxPoolEntriesProbe+1 || len(res.poolEntries) != maxPoolEntriesProbe {
		t.Fatalf("pool=%d entries=%d", res.PoolCount, len(res.poolEntries))
	}
}

func TestScanWalletKVCollectsEncryptedRecords(t *testing.T) {
	// mkey record: key = ("mkey", uint32 id); value = CMasterKey.
	var mkKey bytes.Buffer
	writeString(&mkKey, "mkey")
	mkKey.Write([]byte{1, 0, 0, 0}) // nID = 1

	salt := []byte{1, 1, 2, 3, 5, 8, 13, 21}
	passphrase := "s3cret"
	iterations := uint32(2)
	master := bytes.Repeat([]byte{0x33}, wcKeySize)
	dk, iv, err := deriveMasterKeyMaterial(passphrase, salt, iterations)
	if err != nil {
		t.Fatal(err)
	}
	cryptedMaster := aesCBCEncrypt(t, dk, iv, master)

	var mkVal bytes.Buffer
	writeBytes(&mkVal, cryptedMaster)
	writeBytes(&mkVal, salt)
	mkVal.Write([]byte{0, 0, 0, 0}) // derivationMethod
	mkVal.Write([]byte{2, 0, 0, 0}) // iterations = 2

	// ckey record: key = ("ckey", pubkey); value = crypted secret.
	pub := append([]byte{0x03}, bytes.Repeat([]byte{0xbb}, 32)...)
	secret := bytes.Repeat([]byte{0x44}, 32)
	f1 := sha256.Sum256(pub)
	f2 := sha256.Sum256(f1[:])
	cryptedSecret := aesCBCEncrypt(t, master, f2[:wcIVSize], secret)

	var ckKey bytes.Buffer
	writeString(&ckKey, "ckey")
	writeBytes(&ckKey, pub)
	var ckVal bytes.Buffer
	writeBytes(&ckVal, cryptedSecret)

	kv := map[string][]byte{
		mkKey.String(): mkVal.Bytes(),
		ckKey.String(): ckVal.Bytes(),
	}
	res := scanWalletKV(kv, 0x9e)
	if !res.Encrypted || len(res.cryptedKeys) != 1 || len(res.masterKeys) != 1 {
		t.Fatalf("res encrypted=%v ck=%d mk=%d", res.Encrypted, len(res.cryptedKeys), len(res.masterKeys))
	}
	res.Lines = []string{"# header"}
	n, err := decryptCryptedKeys(res, 0x9e, passphrase)
	if err != nil || n != 1 {
		t.Fatalf("decrypt n=%d err=%v", n, err)
	}
}

func TestScanWalletKVCollectsEncryptedDescriptorRecords(t *testing.T) {
	var mkKey bytes.Buffer
	writeString(&mkKey, "mkey")
	mkKey.Write([]byte{1, 0, 0, 0})

	salt := []byte{9, 8, 7, 6, 5, 4, 3, 2}
	passphrase := "desc-pass"
	iterations := uint32(2)
	master := bytes.Repeat([]byte{0x55}, wcKeySize)
	dk, iv, err := deriveMasterKeyMaterial(passphrase, salt, iterations)
	if err != nil {
		t.Fatal(err)
	}
	cryptedMaster := aesCBCEncrypt(t, dk, iv, master)

	var mkVal bytes.Buffer
	writeBytes(&mkVal, cryptedMaster)
	writeBytes(&mkVal, salt)
	mkVal.Write([]byte{0, 0, 0, 0})
	mkVal.Write([]byte{2, 0, 0, 0})

	pub := append([]byte{0x02}, bytes.Repeat([]byte{0xcc}, 32)...)
	secret := bytes.Repeat([]byte{0x66}, 32)
	f1 := sha256.Sum256(pub)
	f2 := sha256.Sum256(f1[:])
	cryptedSecret := aesCBCEncrypt(t, master, f2[:wcIVSize], secret)

	var dkKey bytes.Buffer
	writeString(&dkKey, "walletdescriptorckey")
	writeBytes(&dkKey, pub)
	var dkVal bytes.Buffer
	writeBytes(&dkVal, cryptedSecret)

	kv := map[string][]byte{
		mkKey.String(): mkVal.Bytes(),
		dkKey.String(): dkVal.Bytes(),
	}
	res := scanWalletKV(kv, 0x9e)
	if !res.Encrypted || len(res.cryptedKeys) != 1 {
		t.Fatalf("res encrypted=%v ck=%d", res.Encrypted, len(res.cryptedKeys))
	}
	res.Lines = []string{"# header"}
	n, err := decryptCryptedKeys(res, 0x9e, passphrase)
	if err != nil || n != 1 {
		t.Fatalf("decrypt n=%d err=%v", n, err)
	}
}

func TestReadTypeKey(t *testing.T) {
	var buf bytes.Buffer
	writeString(&buf, "key")
	writeBytes(&buf, []byte{0x02, 0x01})
	typ, rest, err := readType(buf.Bytes())
	if err != nil || typ != "key" || len(rest) != 3 || rest[0] != 2 || rest[1] != 0x02 || rest[2] != 0x01 {
		t.Fatalf("typ=%q rest=%v err=%v", typ, rest, err)
	}
}

func TestPoolIndexRangeNote(t *testing.T) {
	min, max := int64(2), int64(5)
	p := &ProbeResult{PoolCount: 2, PoolIndexMin: &min, PoolIndexMax: &max}
	if got := PoolIndexRangeNote(p); got != "pool_idx=2-5" {
		t.Fatalf("got %q", got)
	}
	single := int64(3)
	p2 := &ProbeResult{PoolCount: 1, PoolIndexMin: &single, PoolIndexMax: &single}
	if got := PoolIndexRangeNote(p2); got != "pool_idx=3" {
		t.Fatalf("single got %q", got)
	}
}

func TestApplyPoolProbeFields(t *testing.T) {
	min, max := int64(4), int64(8)
	p := &ProbeResult{
		PoolCount:            2,
		PoolPubkeys:          2,
		PoolKeysMatched:      1,
		PoolKeysUnmatched:    1,
		PoolIndexMin:         &min,
		PoolIndexMax:         &max,
		PoolEntries:          []PoolEntry{{Index: 4, PubKeyHex: "02aa", SpendsKeyMatched: true}, {Index: 8, PubKeyHex: "03bb"}},
		PoolEntriesTruncated: false,
	}
	out := map[string]interface{}{}
	ApplyPoolProbeFields(out, p)
	if out["pool_count"] != 2 || out["pool_keys_matched"] != 1 || out["pool_keys_unmatched"] != 1 {
		t.Fatalf("counts %#v", out)
	}
	entries, ok := out["pool_entries"].([]map[string]interface{})
	if !ok || len(entries) != 2 || entries[0]["index"] != int64(4) {
		t.Fatalf("entries %#v", out["pool_entries"])
	}
	if out["keypool_hint"] != PoolKeypoolHint() {
		t.Fatalf("hint %#v", out["keypool_hint"])
	}
	if out["pool_indices_replayed"] != false {
		t.Fatalf("pool_indices_replayed %#v", out["pool_indices_replayed"])
	}
	if out["pool_unmatched_hint"] == "" {
		t.Fatalf("pool_unmatched_hint %#v", out["pool_unmatched_hint"])
	}
	unmatched, ok := out["pool_unmatched_entries"].([]map[string]interface{})
	if !ok || len(unmatched) != 1 || unmatched[0]["index"] != int64(8) {
		t.Fatalf("pool_unmatched_entries %#v", out["pool_unmatched_entries"])
	}
	if matched, _ := entries[0]["spends_key_matched"].(bool); !matched {
		t.Fatalf("entry0 spends_key_matched %#v", entries[0])
	}
	if _, ok := entries[1]["spends_key_matched"]; ok {
		t.Fatalf("entry1 should not be spends_key_matched %#v", entries[1])
	}
	ApplyPoolProbeFields(out, &ProbeResult{PoolCount: 0})
	if out["pool_count"] != 2 {
		t.Fatalf("zero pool should not clear prior fields")
	}
}

func TestPoolUnmatchedHintAndRefillSize(t *testing.T) {
	if PoolUnmatchedHint(0) != "" {
		t.Fatal("empty hint")
	}
	if !strings.Contains(PoolUnmatchedHint(2), "2 Core pool") {
		t.Fatalf("hint %q", PoolUnmatchedHint(2))
	}
	if SuggestedKeypoolRefillSize(0) != 0 || SuggestedKeypoolRefillSize(50) != 100 || SuggestedKeypoolRefillSize(150) != 150 {
		t.Fatalf("sizes 0=%d 50=%d 150=%d", SuggestedKeypoolRefillSize(0), SuggestedKeypoolRefillSize(50), SuggestedKeypoolRefillSize(150))
	}
	p := &ProbeResult{
		PoolKeysUnmatched: 1,
		PoolEntries: []PoolEntry{
			{Index: 1, PubKeyHex: "02aa", SpendsKeyMatched: true},
			{Index: 2, PubKeyHex: "03bb"},
		},
	}
	unmatched := UnmatchedPoolEntries(p)
	if len(unmatched) != 1 || unmatched[0].Index != 2 {
		t.Fatalf("unmatched %#v", unmatched)
	}
}

func writeString(w *bytes.Buffer, s string) {
	writeBytes(w, []byte(s))
}

func writeBytes(w *bytes.Buffer, b []byte) {
	n := len(b)
	switch {
	case n < 253:
		w.WriteByte(byte(n))
	case n <= 0xffff:
		w.WriteByte(253)
		w.WriteByte(byte(n))
		w.WriteByte(byte(n >> 8))
	default:
		panic("test helper: oversize")
	}
	w.Write(b)
}
