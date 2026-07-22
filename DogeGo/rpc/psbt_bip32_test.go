// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

func TestParseBIP32PathString(t *testing.T) {
	got, err := parseBIP32PathString("m/44'/3'/0'/0/0")
	if err != nil {
		t.Fatal(err)
	}
	want := []uint32{0x8000002c, 0x80000003, 0x80000000, 0, 0}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d]=%#x want %#x", i, got[i], want[i])
		}
	}
}

func TestWalletCreateFundedPsbtBIP32Derivations(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	recv, err := w.NewReceiveAddress()
	if err != nil {
		t.Fatal(err)
	}
	spendPK := w.P2PKHScript()
	utxo := store.NewUtxoCache()
	utxo.SetTipHeightForTest(100)
	var op [36]byte
	op[0] = 1
	utxo.AddUtxoForTest(op, store.UtxoEntry{Value: 10e8, PkScript: spendPK, Height: 100})

	paths := &DataPaths{
		Utxo: utxo,
		WalletDefaultAddress:           func() string { return recv },
		WalletSpendScripts:             func() [][]byte { return w.SpendScripts() },
		WalletOwnsScript:               func(spk []byte) bool { return w.OwnsScript(spk) },
		WalletAddressHDPath:            func(a string) (string, bool, bool) { return w.AddressHDPath(a) },
		WalletMasterKeyFingerprint:     func() (uint32, bool) { return w.MasterKeyFingerprint() },
		WalletCompressedPubKeyForAddress: func(a string) ([]byte, bool) { return w.CompressedPubKeyForAddress(a) },
	}

	outJ, _ := json.Marshal(map[string]interface{}{recv: 1.0})
	res, code, msg := execWalletCreateFundedPsbt("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
		json.RawMessage(`[]`),
		outJ,
	})
	if code != 0 {
		t.Fatalf("walletcreatefundedpsbt %d %s", code, msg)
	}
	m := res.(map[string]interface{})
	psbtB64, _ := m["psbt"].(string)
	raw, err := base64.StdEncoding.DecodeString(psbtB64)
	if err != nil {
		t.Fatal(err)
	}
	psbt, err := wire.ParsePSBT(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !psbtHasBIP32Deriv(psbt.Inputs[0]) {
		t.Fatal("expected input bip32_derivation")
	}
	if !psbtHasBIP32Deriv(psbt.Outputs[0]) {
		t.Fatal("expected output bip32_derivation")
	}

	dec, code, msg := execDecodePsbt("test", []json.RawMessage{json.RawMessage(`"` + psbtB64 + `"`)})
	if code != 0 {
		t.Fatalf("decodepsbt %d %s", code, msg)
	}
	dm := dec.(map[string]interface{})
	inputs, _ := dm["inputs"].([]interface{})
	in0, _ := inputs[0].(map[string]interface{})
	if in0["bip32_derivs"] == nil {
		t.Fatalf("decode missing bip32_derivs: %#v", in0)
	}
}

func psbtHasBIP32Deriv(m []wire.PsbtKeyValue) bool {
	for _, kv := range m {
		if kv.Type == wire.PsbtInBIP32Derivation || kv.Type == wire.PsbtOutBIP32Derivation {
			if len(kv.Subkey) == 33 && len(kv.Value) >= 8 {
				return true
			}
		}
	}
	return false
}
