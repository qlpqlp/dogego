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

func TestParseOrdEnvelopeDRC20(t *testing.T) {
	body := []byte(`{"p":"drc-20","op":"mint","tick":"wow","amt":"100"}`)
	ct := []byte("text/plain")
	var script []byte
	script = append(script, 0x00, 0x63)       // OP_FALSE OP_IF
	script = append(script, byte(len("ord"))) // push
	script = append(script, []byte("ord")...)
	script = append(script, 0x51) // OP_1 = content type tag
	script = append(script, byte(len(ct)))
	script = append(script, ct...)
	script = append(script, 0x00) // OP_0 = body tag
	script = append(script, byte(len(body)))
	script = append(script, body...)
	script = append(script, 0x68) // OP_ENDIF

	env, ok := ParseOrdEnvelope(script)
	if !ok || env.ContentType != "text/plain" {
		t.Fatalf("%+v ok=%v", env, ok)
	}
	ins, ok := DetectInscriptionFromWitness(10, "abcd", 0, [][]byte{script})
	if !ok || ins.Kind != "drc20" || ins.Tick != "WOW" || ins.Op != "mint" || ins.Source != "envelope" {
		t.Fatalf("%+v", ins)
	}
}

func TestLedgerMintAndTransfer(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	addr := "DTestAddress111111111111111111111"
	if err := st.PutInscription(Inscription{
		ID: "t1", Height: 1, TxID: "aa", Vout: 0, Kind: "drc20", Tick: "WOW", Op: "mint",
		Amount: "1000", Address: addr, RecordedUnix: 1,
	}); err != nil {
		t.Fatal(err)
	}
	bals, err := st.GetAddressBalances(addr, 10)
	if err != nil || len(bals) != 1 || bals[0].Balance != "1000" {
		t.Fatalf("%v %+v", err, bals)
	}
	op := "bb:0"
	if err := st.PutInscription(Inscription{
		ID: "t2", Height: 2, TxID: "bb", Vout: 0, Kind: "drc20", Tick: "WOW", Op: "transfer",
		Amount: "400", Address: addr, Outpoint: op, RecordedUnix: 2,
	}); err != nil {
		t.Fatal(err)
	}
	bals, _ = st.GetAddressBalances(addr, 10)
	if bals[0].Balance != "600" || bals[0].TransfersCount != 1 {
		t.Fatalf("%+v", bals[0])
	}
	recv := "DRecv2222222222222222222222222222"
	if err := st.ApplySpends(3, "cc", []string{op}, recv, 3); err != nil {
		t.Fatal(err)
	}
	fromB, _ := st.GetAddressBalances(addr, 10)
	toB, _ := st.GetAddressBalances(recv, 10)
	if fromB[0].TransfersCount != 0 || toB[0].Balance != "400" {
		t.Fatalf("from=%+v to=%+v", fromB, toB)
	}
}

