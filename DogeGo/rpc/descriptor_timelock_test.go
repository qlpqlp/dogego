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
	"strings"
	"testing"

	"dogego/secp256k1"

	"dogego/chain"
	"dogego/consensus"
	"dogego/wallet"
)

func TestImportDescriptorsShCLTVMulti(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	desc := "sh(cltv(500000)multi(1," + h1 + "," + h2 + "))"
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif1, _ := chain.EncodeWIF(k1.Serialize(), p.PrivKeyWIFVersion, true)
	paths := importDescTestPaths(w, p)
	req, _ := json.Marshal(map[string]interface{}{"desc": desc, "keys": []string{wif1}})
	_, code, msg := execImportDescriptors("test", paths, nil, nil, []json.RawMessage{json.RawMessage("[" + string(req) + "]")})
	if code != 0 {
		t.Fatalf("importdescriptors: %s", msg)
	}
	redeem := w.WatchRedeemScript(w.WatchScripts()[0])
	n, pubs, ok := consensus.MultisigRedeemFromP2SH(redeem)
	if !ok || n != 1 || len(pubs) != 2 {
		t.Fatalf("multisig from CLTV redeem n=%d pubs=%d ok=%v", n, len(pubs), ok)
	}
	got, ok := consensus.P2SHRedeemDescriptor(redeem, p.PubkeyHashAddrID)
	if !ok || !strings.HasPrefix(got, "sh(cltv(") {
		t.Fatalf("descriptor %q ok=%v", got, ok)
	}
}

func TestGetDescriptorInfoShCLTVMulti(t *testing.T) {
	p, _ := chain.ParamsFor(chain.RebootTestnet)
	k1, _ := secp256k1.NewPrivateKey()
	k2, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	h2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
	desc := `sh(cltv(42)multi(1,` + h1 + `,` + h2 + `))`
	w, err := wallet.LoadOrCreate(filepath.Join(t.TempDir(), "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	wif1, _ := chain.EncodeWIF(k1.Serialize(), p.PrivKeyWIFVersion, true)
	_ = w.ImportPrivKey(wif1, p.PrivKeyWIFVersion, p.PubkeyHashAddrID)
	paths := importDescTestPaths(w, p)
	res, code, msg := execGetDescriptorInfo("test", paths, []json.RawMessage{json.RawMessage(`"` + desc + `"`)})
	if code != 0 {
		t.Fatalf("getdescriptorinfo: %s", msg)
	}
	m := res.(map[string]interface{})
	if m["issolvable"] != true {
		t.Fatalf("issolvable %#v", m)
	}
}

func TestGetDescriptorInfoShCLTVMultiDuplicateKeysRejected(t *testing.T) {
	k1, _ := secp256k1.NewPrivateKey()
	h1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
	desc := `sh(cltv(42)multi(1,` + h1 + `,` + h1 + `))`
	_, code, msg := execGetDescriptorInfo("test", nil, []json.RawMessage{json.RawMessage(`"` + desc + `"`)})
	if code == 0 {
		t.Fatal("expected duplicate key rejection")
	}
	if !strings.Contains(msg, "unsupported descriptor") {
		t.Fatalf("msg %q", msg)
	}
}
