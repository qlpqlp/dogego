// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"dogego/wallet"
	"dogego/wallet/corewallet"
)

func TestExecProbeWalletDatWrongArgs(t *testing.T) {
	_, code, _ := execProbeWalletDat("test", nil, nil)
	if code != -32602 {
		t.Fatalf("code=%d", code)
	}
}

func TestExecProbeWalletDatEmptyPath(t *testing.T) {
	_, code, msg := execProbeWalletDat("test", nil, []json.RawMessage{json.RawMessage(`""`)})
	if code != -8 || msg == "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecProbeWalletDatNotBDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not.dat")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := mustProbeWalletDatMap(t, "testnet", nil, path)
	if probeMapBool(m, "is_bdb") {
		t.Fatalf("expected not bdb %#v", m)
	}
}

func TestExecProbeWalletDatSyntheticFixture(t *testing.T) {
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := bytes.Repeat([]byte{0xee}, 32)
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDat(path, pub, secret); err != nil {
		t.Fatal(err)
	}
	m := mustProbeWalletDatMap(t, "testnet", nil, path)
	if !probeMapBool(m, "is_bdb") || !probeMapBool(m, "can_import") || probeMapInt(m, "key_count") != 1 {
		t.Fatalf("probe %#v", m)
	}
}

func TestExecProbeWalletDatEncryptedDescriptorSyntheticFixture(t *testing.T) {
	pub := append([]byte{0x02}, make([]byte, 32)...)
	secret := bytes.Repeat([]byte{0x33}, 32)
	passphrase := "descriptor-probe-pass"
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestEncryptedDescriptorWalletDat(path, pub, secret, passphrase); err != nil {
		t.Fatal(err)
	}
	m := mustProbeWalletDatMap(t, "testnet", nil, path)
	if !probeMapBool(m, "is_bdb") || !probeMapBool(m, "needs_passphrase") || !probeMapBool(m, "can_import") || probeMapInt(m, "encrypted_keys") != 1 {
		t.Fatalf("probe %#v", m)
	}
}

func TestExecProbeWalletDatPoolSyntheticFixture(t *testing.T) {
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := bytes.Repeat([]byte{0x44}, 32)
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDatWithPool(path, pub, secret, 2); err != nil {
		t.Fatal(err)
	}
	m := mustProbeWalletDatMap(t, "testnet", nil, path)
	if !probeMapBool(m, "is_bdb") || probeMapInt(m, "pool_count") != 1 || probeMapInt(m, "key_count") != 1 || !probeMapBool(m, "can_import") {
		t.Fatalf("probe %#v", m)
	}
	if probeMapInt(m, "pool_pubkeys") != 1 {
		t.Fatalf("pool_pubkeys=%d", probeMapInt(m, "pool_pubkeys"))
	}
	if probeMapInt(m, "pool_keys_matched") != 1 {
		t.Fatalf("pool_keys_matched=%d", probeMapInt(m, "pool_keys_matched"))
	}
	if probeMapInt(m, "pool_keys_unmatched") != 0 {
		t.Fatalf("pool_keys_unmatched=%d", probeMapInt(m, "pool_keys_unmatched"))
	}
	min := probeMapInt64Ptr(m, "pool_index_min")
	max := probeMapInt64Ptr(m, "pool_index_max")
	if min == nil || *min != 2 || max == nil || *max != 2 {
		t.Fatalf("pool indices min=%v max=%v", min, max)
	}
	replayed := probeMapBoolPtr(m, "pool_indices_replayed")
	if replayed == nil || *replayed {
		t.Fatalf("pool_indices_replayed %#v", replayed)
	}
}

func TestExecProbeWalletDatMultiPoolRange(t *testing.T) {
	pub := append([]byte{0x03}, make([]byte, 32)...)
	secret := bytes.Repeat([]byte{0x55}, 32)
	path := filepath.Join(t.TempDir(), "wallet.dat")
	if err := corewallet.WriteTestWalletDatWithPools(path, pub, secret, []int64{1, 5, 9}); err != nil {
		t.Fatal(err)
	}
	m := mustProbeWalletDatMap(t, "testnet", nil, path)
	if probeMapInt(m, "pool_count") != 3 || probeMapInt(m, "pool_pubkeys") != 3 || probeMapInt(m, "pool_keys_matched") != 3 || probeMapInt(m, "pool_keys_unmatched") != 0 {
		t.Fatalf("probe %#v", m)
	}
	entries, ok := m["pool_entries"].([]interface{})
	if !ok || len(entries) != 3 {
		t.Fatalf("pool_entries %#v", m["pool_entries"])
	}
	min := probeMapInt64Ptr(m, "pool_index_min")
	max := probeMapInt64Ptr(m, "pool_index_max")
	if min == nil || *min != 1 || max == nil || *max != 9 {
		t.Fatalf("pool indices min=%v max=%v", min, max)
	}
}

func TestExecProbeWalletDatIncludesHDKeypoolCoreIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not.dat")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletHDKeypoolCoreIndex: func() []wallet.HDKeypoolCoreIndexEntry {
			return []wallet.HDKeypoolCoreIndexEntry{
				{ReceiveIndex: 0, CoreIndex: 7},
				{ReceiveIndex: 2, CoreIndex: 9},
			}
		},
	}
	m := mustProbeWalletDatMap(t, "testnet", paths, path)
	if probeMapInt(m, "pool_core_indices_stored") != 2 {
		t.Fatalf("pool_core_indices_stored=%d", probeMapInt(m, "pool_core_indices_stored"))
	}
	rv := reflect.ValueOf(m["hd_keypool_core_index"])
	if !rv.IsValid() || rv.Kind() != reflect.Slice || rv.Len() != 2 {
		t.Fatalf("hd_keypool_core_index %#v", m["hd_keypool_core_index"])
	}
}

func mustProbeWalletDatMap(t *testing.T, chainName string, paths *DataPaths, filename string) map[string]interface{} {
	t.Helper()
	if paths == nil {
		paths = &DataPaths{}
	}
	if paths.BaseDataDir == "" {
		abs, err := filepath.Abs(filepath.Dir(filename))
		if err != nil {
			t.Fatal(err)
		}
		paths.BaseDataDir = abs
		paths.ChainDataDir = abs
	}
	res, code, msg := execProbeWalletDat(chainName, paths, mustWalletJSONParam(t, filename))
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("type %T", res)
	}
	return m
}

func probeMapBool(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func probeMapInt(m map[string]interface{}, key string) int {
	switch n := m[key].(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

func probeMapInt64Ptr(m map[string]interface{}, key string) *int64 {
	switch n := m[key].(type) {
	case float64:
		v := int64(n)
		return &v
	case int64:
		return &n
	default:
		return nil
	}
}

func probeMapBoolPtr(m map[string]interface{}, key string) *bool {
	v, ok := m[key].(bool)
	if !ok {
		return nil
	}
	return &v
}
