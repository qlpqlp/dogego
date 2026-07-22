// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package corewallet

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"dogego/chain"
	"dogego/wallet/bdb"
)

const maxPoolEntriesProbe = 64

// PoolEntry is one Core BDB pool record (index + optional pubkey from value).
type PoolEntry struct {
	Index            int64  `json:"index"`
	PubKeyHex        string `json:"pubkey_hex,omitempty"`
	SpendsKeyMatched bool   `json:"spends_key_matched,omitempty"`
}

// ExtractResult holds keys parsed from a Core wallet.dat BDB file.
type ExtractResult struct {
	Lines        []string
	Encrypted    bool
	KeyCount     int
	WatchCount   int
	PoolCount    int
	PoolPubkeys           int
	PoolKeysMatched       int
	PoolIndexMin          int64
	PoolIndexMax          int64
	poolIndexSeen         bool
	poolEntries           []PoolEntry
	knownSpendPubkeys     map[string]struct{}

	// Encrypted-wallet material (populated during scan; used when a passphrase is supplied).
	cryptedKeys []cryptedKeyRecord
	masterKeys  []masterKey
}

// cryptedKeyRecord is one ckey / walletdescriptorckey entry.
type cryptedKeyRecord struct {
	pubKey        []byte
	cryptedSecret []byte
}

// ProbeResult summarizes a wallet.dat without importing.
type ProbeResult struct {
	IsBDB           bool   `json:"is_bdb"`
	Encrypted       bool   `json:"encrypted"`
	KeyCount        int    `json:"key_count"`
	EncryptedKeys   int    `json:"encrypted_keys,omitempty"`
	WatchCount      int    `json:"watch_count"`
	PoolCount       int    `json:"pool_count,omitempty"`
	PoolPubkeys            int         `json:"pool_pubkeys,omitempty"`
	PoolKeysMatched        int         `json:"pool_keys_matched,omitempty"`
	PoolKeysUnmatched      int         `json:"pool_keys_unmatched,omitempty"`
	PoolIndexMin           *int64      `json:"pool_index_min,omitempty"`
	PoolIndexMax           *int64      `json:"pool_index_max,omitempty"`
	PoolIndicesReplayed    *bool       `json:"pool_indices_replayed,omitempty"`
	PoolEntries            []PoolEntry `json:"pool_entries,omitempty"`
	PoolEntriesTruncated   bool        `json:"pool_entries_truncated,omitempty"`
	CanImport              bool        `json:"can_import"`
	NeedsPassphrase bool   `json:"needs_passphrase,omitempty"`
	Hint            string `json:"hint,omitempty"`
}

// ProbeWalletDat inspects a Core wallet.dat (native BDB read; no import).
func ProbeWalletDat(path string, wifVer byte) (*ProbeResult, error) {
	out := &ProbeResult{}
	if !bdb.IsBDBFile(path) {
		out.Hint = "not a Berkeley DB wallet.dat"
		return out, nil
	}
	out.IsBDB = true
	kv, err := bdb.OpenKV(path)
	if err != nil {
		return out, err
	}
	res := scanWalletKV(kv, wifVer)
	out.Encrypted = res.Encrypted
	out.KeyCount = res.KeyCount
	out.EncryptedKeys = len(res.cryptedKeys)
	out.WatchCount = res.WatchCount
	out.PoolCount = res.PoolCount
	out.PoolPubkeys = res.PoolPubkeys
	out.PoolKeysMatched = res.PoolKeysMatched
	finalizeProbePoolStats(out)
	if len(res.poolEntries) > 0 {
		out.PoolEntries = append([]PoolEntry(nil), res.poolEntries...)
		out.PoolEntriesTruncated = res.PoolCount > len(res.poolEntries)
	}
	if res.poolIndexSeen {
		min, max := res.PoolIndexMin, res.PoolIndexMax
		out.PoolIndexMin = &min
		out.PoolIndexMax = &max
	}
	switch {
	case res.Encrypted && len(res.cryptedKeys) > 0:
		out.NeedsPassphrase = true
		out.CanImport = len(res.masterKeys) > 0
		if out.CanImport {
			out.Hint = "encrypted wallet.dat - pass a passphrase to dogego_importwalletdat for native decryption"
		} else {
			out.Hint = "encrypted wallet.dat missing master key record - use via_core_rpc"
		}
	case res.Encrypted && res.KeyCount == 0:
		out.Hint = "encrypted wallet.dat - unlock in Core and use via_core_rpc, or dogego_importwalletdat with core_rpc_addr"
	case res.KeyCount == 0:
		out.Hint = "no spend keys found"
	default:
		out.CanImport = res.KeyCount > 0
		out.Hint = "ready for dogego_importwalletdat (native BDB)"
	}
	if out.PoolCount > 0 {
		poolHint := PoolKeypoolHint()
		if out.Hint == "" || out.Hint == "no spend keys found" {
			out.Hint = poolHint
		} else if !strings.Contains(out.Hint, "keypool") {
			out.Hint = out.Hint + "; " + poolHint
		}
		replayed := false
		out.PoolIndicesReplayed = &replayed
	}
	return out, nil
}

// PoolKeypoolHint explains Core BDB pool handling during DogeGo wallet.dat migration.
func PoolKeypoolHint() string {
	return "Core keypool entries detected - spend keys import via ckey/key; native import replays matched HD receive pubkeys into hd_keypool (pool_indices_replayed) and stores Core pool indices in hd_keypool_core_index; run keypoolrefill for additional receive keys; pool-only pubkeys without spend keys stay unmatched"
}

// PoolUnmatchedHint returns operator guidance when pool_keys_unmatched > 0.
func PoolUnmatchedHint(unmatched int) string {
	if unmatched <= 0 {
		return ""
	}
	return fmt.Sprintf("%d Core pool pubkey(s) have no spend key in wallet.dat - DogeGo cannot recover those private keys; run keypoolrefill to issue fresh HD receive keys", unmatched)
}

// SuggestedKeypoolRefillSize returns a keypoolrefill newsize when Core left pool-only rows (0 = default).
func SuggestedKeypoolRefillSize(unmatched int) int {
	const defaultSize = 100
	if unmatched <= 0 {
		return 0
	}
	if unmatched > defaultSize {
		return unmatched
	}
	return defaultSize
}

// UnmatchedPoolEntries returns probe pool rows whose pubkey has no spend key in wallet.dat.
func UnmatchedPoolEntries(p *ProbeResult) []PoolEntry {
	if p == nil || p.PoolKeysUnmatched <= 0 {
		return nil
	}
	var out []PoolEntry
	for _, e := range p.PoolEntries {
		if e.PubKeyHex != "" && !e.SpendsKeyMatched {
			out = append(out, e)
		}
	}
	return out
}

// ExtractDumpLines reads an unencrypted Core BDB wallet.dat and returns dumpwallet-style lines.
func ExtractDumpLines(path string, wifVer byte) (*ExtractResult, error) {
	if !bdb.IsBDBFile(path) {
		return nil, fmt.Errorf("not a Berkeley DB wallet.dat")
	}
	kv, err := bdb.OpenKV(path)
	if err != nil {
		return nil, err
	}
	res := &ExtractResult{
		Lines: []string{
			fmt.Sprintf("# Wallet dump created by DogeGo %s", time.Now().UTC().Format(time.RFC3339)),
			"# * Created on " + time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		},
	}
	for k, v := range kv {
		appendWalletRecord(res, []byte(k), v, wifVer)
	}
	finalizePoolKeyMatches(res)
	if res.Encrypted && res.KeyCount == 0 {
		return res, fmt.Errorf("encrypted wallet.dat - pass options.passphrase for native decryption, or unlock in Core and use via_core_rpc dumpwallet")
	}
	if res.KeyCount == 0 {
		return res, fmt.Errorf("no spend keys found in wallet.dat")
	}
	return res, nil
}

func scanWalletKV(kv map[string][]byte, wifVer byte) *ExtractResult {
	res := &ExtractResult{}
	for k, v := range kv {
		appendWalletRecord(res, []byte(k), v, wifVer)
	}
	finalizePoolKeyMatches(res)
	return res
}

// ExtractDumpLinesWithPassphrase reads a Core BDB wallet.dat and returns dumpwallet-style
// lines. If the wallet is encrypted, the passphrase is used to natively decrypt keys
// (Core CCrypter scheme). An empty passphrase behaves like ExtractDumpLines.
func ExtractDumpLinesWithPassphrase(path string, wifVer byte, passphrase string) (*ExtractResult, error) {
	if !bdb.IsBDBFile(path) {
		return nil, fmt.Errorf("not a Berkeley DB wallet.dat")
	}
	kv, err := bdb.OpenKV(path)
	if err != nil {
		return nil, err
	}
	res := &ExtractResult{
		Lines: []string{
			fmt.Sprintf("# Wallet dump created by DogeGo %s", time.Now().UTC().Format(time.RFC3339)),
			"# * Created on " + time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		},
	}
	for k, v := range kv {
		appendWalletRecord(res, []byte(k), v, wifVer)
	}
	if res.Encrypted && len(res.cryptedKeys) > 0 {
		if passphrase == "" {
			return res, fmt.Errorf("encrypted wallet.dat requires a passphrase")
		}
		n, err := decryptCryptedKeys(res, wifVer, passphrase)
		if err != nil {
			return res, err
		}
		res.KeyCount += n
		finalizePoolKeyMatches(res)
	}
	if res.KeyCount == 0 {
		if res.Encrypted {
			return res, fmt.Errorf("no keys recovered from encrypted wallet.dat")
		}
		return res, fmt.Errorf("no spend keys found in wallet.dat")
	}
	return res, nil
}

// decryptCryptedKeys unlocks the master key with passphrase and decrypts each ckey record,
// appending WIF lines. Returns the number of keys recovered.
func decryptCryptedKeys(res *ExtractResult, wifVer byte, passphrase string) (int, error) {
	if len(res.masterKeys) == 0 {
		return 0, fmt.Errorf("encrypted wallet.dat has no master key record")
	}
	var master []byte
	var lastErr error
	for _, mk := range res.masterKeys {
		m, err := decryptMasterKey(mk, passphrase)
		if err != nil {
			lastErr = err
			continue
		}
		master = m
		break
	}
	if master == nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("wrong passphrase")
		}
		return 0, lastErr
	}
	recovered := 0
	for _, ck := range res.cryptedKeys {
		secret, err := decryptSecret(master, ck.cryptedSecret, ck.pubKey)
		if err != nil {
			continue
		}
		compressed := len(ck.pubKey) == 33 && (ck.pubKey[0] == 0x02 || ck.pubKey[0] == 0x03)
		wif, err := chain.EncodeWIF(secret, wifVer, compressed)
		if err != nil {
			continue
		}
		if res.Lines != nil {
			appendDumpWIFLine(res, wif)
		}
		noteKnownSpendPubkey(res, ck.pubKey)
		recovered++
	}
	if recovered == 0 {
		return 0, fmt.Errorf("passphrase accepted but no keys could be decrypted")
	}
	return recovered, nil
}

func appendWalletRecord(res *ExtractResult, keyBytes, valBytes []byte, wifVer byte) {
	typ, keyRest, err := readType(keyBytes)
	if err != nil {
		return
	}
	switch typ {
	case "key", "wkey", "walletdescriptorkey":
		pub, err := readBytes(keyRest)
		if err != nil {
			return
		}
		secret, _, err := readPrivKey(valBytes)
		if err != nil || len(secret) != 32 {
			return
		}
		compressed := len(pub) == 33 && (pub[0] == 0x02 || pub[0] == 0x03)
		wif, err := chain.EncodeWIF(secret, wifVer, compressed)
		if err != nil {
			return
		}
		if res.Lines != nil {
			appendDumpWIFLine(res, wif)
		}
		noteKnownSpendPubkey(res, pub)
		res.KeyCount++
	case "ckey", "walletdescriptorckey":
		res.Encrypted = true
		pub, err := readBytes(keyRest)
		if err != nil {
			return
		}
		secret, err := readBytes(valBytes)
		if err != nil || len(secret) == 0 {
			return
		}
		res.cryptedKeys = append(res.cryptedKeys, cryptedKeyRecord{pubKey: pub, cryptedSecret: secret})
	case "mkey":
		res.Encrypted = true
		mk, err := parseMasterKeyValue(valBytes)
		if err != nil {
			return
		}
		res.masterKeys = append(res.masterKeys, mk)
	case "watchs":
		script, err := readBytes(keyRest)
		if err != nil || len(script) == 0 {
			return
		}
		if len(valBytes) > 0 && valBytes[0] == '1' {
			if res.Lines != nil {
				res.Lines = append(res.Lines, "script=1 "+hex.EncodeToString(script))
			}
			res.WatchCount++
		}
	case "pool":
		res.PoolCount++
		var idx int64
		if len(keyRest) >= 8 {
			idx = int64(binary.LittleEndian.Uint64(keyRest[:8]))
			if !res.poolIndexSeen {
				res.poolIndexSeen = true
				res.PoolIndexMin = idx
				res.PoolIndexMax = idx
			} else {
				if idx < res.PoolIndexMin {
					res.PoolIndexMin = idx
				}
				if idx > res.PoolIndexMax {
					res.PoolIndexMax = idx
				}
			}
		}
		entry := PoolEntry{Index: idx}
		if pub, err := readBytes(valBytes); err == nil && len(pub) >= 33 && (pub[0] == 0x02 || pub[0] == 0x03) {
			res.PoolPubkeys++
			entry.PubKeyHex = hex.EncodeToString(pub)
		}
		if len(res.poolEntries) < maxPoolEntriesProbe {
			res.poolEntries = append(res.poolEntries, entry)
		}
	}
}

func readType(b []byte) (typ string, rest []byte, err error) {
	r := bytes.NewReader(b)
	s, err := readStringFrom(r)
	if err != nil {
		return "", nil, err
	}
	rest, _ = io.ReadAll(r)
	return s, rest, nil
}

func readBytes(b []byte) ([]byte, error) {
	r := bytes.NewReader(b)
	n, err := readCompactSize(r)
	if err != nil {
		return nil, err
	}
	if n > 10_000 {
		return nil, fmt.Errorf("oversize blob")
	}
	out := make([]byte, n)
	_, err = io.ReadFull(r, out)
	return out, err
}

func readStringFrom(r *bytes.Reader) (string, error) {
	b, err := readBytesFrom(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func readBytesFrom(r *bytes.Reader) ([]byte, error) {
	n, err := readCompactSize(r)
	if err != nil {
		return nil, err
	}
	if n > 10_000 {
		return nil, fmt.Errorf("oversize blob")
	}
	out := make([]byte, n)
	_, err = io.ReadFull(r, out)
	return out, err
}

func readUint32(r *bytes.Reader) (uint32, error) {
	var x [4]byte
	if _, err := io.ReadFull(r, x[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(x[:]), nil
}

func readCompactSize(r *bytes.Reader) (uint64, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	switch b[0] {
	case 253:
		var x [2]byte
		if _, err := io.ReadFull(r, x[:]); err != nil {
			return 0, err
		}
		return uint64(binary.LittleEndian.Uint16(x[:])), nil
	case 254:
		var x [4]byte
		if _, err := io.ReadFull(r, x[:]); err != nil {
			return 0, err
		}
		return uint64(binary.LittleEndian.Uint32(x[:])), nil
	case 255:
		var x [8]byte
		if _, err := io.ReadFull(r, x[:]); err != nil {
			return 0, err
		}
		return binary.LittleEndian.Uint64(x[:]), nil
	default:
		return uint64(b[0]), nil
	}
}

func readPrivKey(val []byte) (secret []byte, checksum []byte, err error) {
	r := bytes.NewReader(val)
	raw, err := readBytesFrom(r)
	if err != nil {
		return nil, nil, err
	}
	if len(raw) == 32 {
		secret = raw
	} else if len(raw) >= 40 && raw[0] == 0x30 {
		switch {
		case len(raw) >= 40 && bytes.HasPrefix(raw, []byte{0x30, 0x81, 0xd3, 0x02, 0x01, 0x01, 0x04, 0x20}):
			secret = raw[8:40]
		case len(raw) >= 41 && bytes.HasPrefix(raw, []byte{0x30, 0x82, 0x01, 0x13, 0x02, 0x01, 0x01, 0x04, 0x20}):
			secret = raw[9:41]
		}
	}
	if secret == nil {
		return nil, nil, fmt.Errorf("unrecognized privkey encoding")
	}
	if r.Len() >= 32 {
		checksum = make([]byte, 32)
		_, _ = io.ReadFull(r, checksum)
	}
	return secret, checksum, nil
}

// appendDumpWIFLine adds a Core dumpwallet-compatible spend-key line (timestamp,wif).
func appendDumpWIFLine(res *ExtractResult, wif string) {
	res.Lines = append(res.Lines, fmt.Sprintf("%d,%s", time.Now().Unix(), wif))
}

func noteKnownSpendPubkey(res *ExtractResult, pub []byte) {
	if res == nil || len(pub) < 33 {
		return
	}
	if res.knownSpendPubkeys == nil {
		res.knownSpendPubkeys = make(map[string]struct{})
	}
	res.knownSpendPubkeys[hex.EncodeToString(pub)] = struct{}{}
}

func finalizePoolKeyMatches(res *ExtractResult) {
	if res == nil || len(res.poolEntries) == 0 || len(res.knownSpendPubkeys) == 0 {
		return
	}
	for _, e := range res.poolEntries {
		if e.PubKeyHex == "" {
			continue
		}
		if _, ok := res.knownSpendPubkeys[e.PubKeyHex]; ok {
			res.PoolKeysMatched++
		}
	}
	for i := range res.poolEntries {
		if res.poolEntries[i].PubKeyHex == "" {
			continue
		}
		if _, ok := res.knownSpendPubkeys[res.poolEntries[i].PubKeyHex]; ok {
			res.poolEntries[i].SpendsKeyMatched = true
		}
	}
}

func finalizeProbePoolStats(out *ProbeResult) {
	if out == nil || out.PoolPubkeys <= out.PoolKeysMatched {
		return
	}
	out.PoolKeysUnmatched = out.PoolPubkeys - out.PoolKeysMatched
}

// PoolIndexRangeNote returns a compact pool index summary for operator logs, or "" when unset.
func PoolIndexRangeNote(p *ProbeResult) string {
	if p == nil || p.PoolCount == 0 || p.PoolIndexMin == nil || p.PoolIndexMax == nil {
		return ""
	}
	if *p.PoolIndexMin == *p.PoolIndexMax {
		return fmt.Sprintf("pool_idx=%d", *p.PoolIndexMin)
	}
	return fmt.Sprintf("pool_idx=%d-%d", *p.PoolIndexMin, *p.PoolIndexMax)
}

// ApplyPoolProbeFields copies pool probe metadata into a JSON-RPC result map (probe or import).
func ApplyPoolProbeFields(out map[string]interface{}, p *ProbeResult) {
	if out == nil || p == nil || p.PoolCount == 0 {
		return
	}
	out["pool_count"] = p.PoolCount
	if p.PoolPubkeys > 0 {
		out["pool_pubkeys"] = p.PoolPubkeys
	}
	if p.PoolKeysMatched > 0 {
		out["pool_keys_matched"] = p.PoolKeysMatched
	}
	if p.PoolKeysUnmatched > 0 {
		out["pool_keys_unmatched"] = p.PoolKeysUnmatched
	}
	if len(p.PoolEntries) > 0 {
		entries := make([]map[string]interface{}, len(p.PoolEntries))
		for i, e := range p.PoolEntries {
			row := map[string]interface{}{
				"index":      e.Index,
				"pubkey_hex": e.PubKeyHex,
			}
			if e.SpendsKeyMatched {
				row["spends_key_matched"] = true
			}
			entries[i] = row
		}
		out["pool_entries"] = entries
	}
	if p.PoolKeysUnmatched > 0 {
		out["pool_unmatched_hint"] = PoolUnmatchedHint(p.PoolKeysUnmatched)
		if unmatched := UnmatchedPoolEntries(p); len(unmatched) > 0 {
			rows := make([]map[string]interface{}, len(unmatched))
			for i, e := range unmatched {
				rows[i] = map[string]interface{}{
					"index":      e.Index,
					"pubkey_hex": e.PubKeyHex,
				}
			}
			out["pool_unmatched_entries"] = rows
		}
	}
	if p.PoolEntriesTruncated {
		out["pool_entries_truncated"] = true
	}
	if p.PoolIndexMin != nil {
		out["pool_index_min"] = *p.PoolIndexMin
		out["pool_index_max"] = *p.PoolIndexMax
	}
	out["pool_indices_replayed"] = false
	out["keypool_hint"] = PoolKeypoolHint()
}
