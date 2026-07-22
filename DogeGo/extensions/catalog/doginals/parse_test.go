// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"encoding/hex"
	"path/filepath"
	"testing"

	"dogego/wire"
)

func TestParseDRC20JSON(t *testing.T) {
	p, ok := ParseDRC20JSON([]byte(`{"p":"drc-20","op":"mint","tick":"doge","amt":"1000"}`))
	if !ok || p.Tick != "DOGE" || p.Op != "mint" || p.Amt != "1000" {
		t.Fatalf("%+v ok=%v", p, ok)
	}
	if _, ok := ParseDRC20JSON([]byte(`{"p":"brc-20"}`)); ok {
		t.Fatal("expected reject")
	}
}

func TestDetectInscriptionOPReturn(t *testing.T) {
	payload := []byte(`{"p":"drc-20","op":"deploy","tick":"wow","max":"21000000","lim":"1000"}`)
	pk := append([]byte{0x6a, byte(len(payload))}, payload...)
	o := wire.TxOut{Value: 0, PkScript: pk}
	ins, ok := DetectInscriptionFromOutput(42, "abcd", 0, o)
	if !ok || ins.Kind != "drc20" || ins.Tick != "WOW" || ins.Op != "deploy" {
		t.Fatalf("%+v", ins)
	}
}

func TestAssetNormalizeAndStore(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, err := NormalizeAsset(Asset{Kind: "nft", Name: "Much #1", URI: "ipfs://x"})
	if err != nil || a.ID == "" {
		t.Fatal(err, a)
	}
	if err := st.PutAsset(a); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetAsset(a.ID)
	if err != nil || !ok || got.Name != "Much #1" {
		t.Fatalf("%v %v %+v", ok, err, got)
	}
	ids := EncodeInv([]string{a.ID})
	if len(DecodeInv(ids)) != 1 {
		t.Fatal(hex.EncodeToString(ids))
	}
}

func TestBuildDRC20JSON(t *testing.T) {
	b, err := BuildDRC20JSON("mint", "woof", "100", "", "")
	if err != nil {
		t.Fatal(err)
	}
	p, ok := ParseDRC20JSON(b)
	if !ok || p.Op != "mint" || p.Tick != "WOOF" || p.Amt != "100" {
		t.Fatalf("%s %+v", b, p)
	}
	prev, err := PreviewInscription("deploy", "wow", "", "21000000", "1000")
	if err != nil {
		t.Fatal(err)
	}
	if prev["bytes"].(int) > 80 {
		t.Fatal(prev)
	}
}

func TestExtractOPReturn(t *testing.T) {
	if ExtractOPReturnPayload([]byte{0x6a, 0x03, 'a', 'b', 'c'}) == nil {
		t.Fatal("want payload")
	}
	if ExtractOPReturnPayload([]byte{0x76}) != nil {
		t.Fatal("not opreturn")
	}
}
