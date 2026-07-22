// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/wallet"
	"dogego/wallet/corewallet"
)

func TestValidateAddressHDFields(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	recv, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	wifVer := p.PrivKeyWIFVersion
	paths := &DataPaths{
		WalletDefaultAddress:  func() string { return w.Address() },
		WalletContainsAddress: func(a string) bool { return w.ContainsAddress(a) },
		WalletWIFForAddress: func(a string) (string, error) {
			priv, err := w.PrivKeyForAddress(a)
			if err != nil {
				return "", err
			}
			return chain.EncodeWIF(priv.Serialize(), wifVer, true)
		},
		WalletAddressHDPath: func(a string) (string, bool, bool) { return w.AddressHDPath(a) },
	}
	addrJ, _ := json.Marshal(recv)
	res, code, msg := execValidateAddress("test", paths, []json.RawMessage{addrJ})
	if code != 0 {
		t.Fatalf("validateaddress: %s", msg)
	}
	m := res
	if m["ismine"] != true {
		t.Fatalf("ismine %#v", m)
	}
	if m["hdkeypath"] == nil || m["hdkeypath"] == "" {
		t.Fatal("expected hdkeypath")
	}
	if m["pubkey"] == nil || m["pubkey"] == "" {
		t.Fatal("expected pubkey")
	}
}

func TestValidateAddressKeypoolCoreIndex(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	recv := w.PeekReceiveAddress()
	pub, ok := w.CompressedPubKeyForAddress(recv)
	if !ok {
		t.Fatal("missing pubkey")
	}
	pubHex := hex.EncodeToString(pub)
	if _, err := w.ReplayCorePoolIntoHDKeypool([]corewallet.PoolEntry{{Index: 77, PubKeyHex: pubHex}}); err != nil {
		t.Fatal(err)
	}
	paths := &DataPaths{
		WalletContainsAddress:         func(a string) bool { return w.ContainsAddress(a) },
		WalletAddressHDPath:           func(a string) (string, bool, bool) { return w.AddressHDPath(a) },
		WalletAddressInReceiveKeypool: func(a string) bool { return w.IsReceiveInKeypool(a) },
		WalletAddressInChangeKeypool:  func(a string) bool { return w.IsChangeInKeypool(a) },
		WalletAddressCorePoolIndex:    func(a string) (int64, bool) { return w.CorePoolIndexForAddress(a) },
	}
	recvJ, _ := json.Marshal(recv)
	res, code, msg := execValidateAddress("test", paths, []json.RawMessage{recvJ})
	if code != 0 {
		t.Fatalf("validateaddress: %s", msg)
	}
	m := res
	if m["iskeypool"] != true {
		t.Fatalf("iskeypool %#v", m["iskeypool"])
	}
	switch idx := m["hd_keypool_core_index"].(type) {
	case float64:
		if int(idx) != 77 {
			t.Fatalf("core index=%v", m["hd_keypool_core_index"])
		}
	case int64:
		if idx != 77 {
			t.Fatalf("core index=%v", m["hd_keypool_core_index"])
		}
	default:
		t.Fatalf("hd_keypool_core_index %#v", m["hd_keypool_core_index"])
	}
}
