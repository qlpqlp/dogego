// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"path/filepath"
	"testing"
)

func pushStr(s string) []byte {
	b := []byte(s)
	out := []byte{byte(len(b))}
	return append(out, b...)
}

func TestExtractP2SHSinglePartImage(t *testing.T) {
	// apezord-style: "ord" OP_1 content-type OP_0 body [sig] [redeem]
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x01}
	var script []byte
	script = append(script, pushStr("ord")...)
	script = append(script, 0x51) // OP_1 pieces=1
	script = append(script, pushStr("image/png")...)
	script = append(script, 0x00) // OP_0 separator
	script = append(script, pushStr(string(png))...)
	// fake sig + redeem
	sig := make([]byte, 71)
	redeem := make([]byte, 40)
	script = append(script, byte(len(sig)))
	script = append(script, sig...)
	script = append(script, byte(len(redeem)))
	script = append(script, redeem...)

	p, ok := ExtractP2SHInscriptionPartial(script)
	if !ok || !p.StartsOrd || p.Pieces != 1 || p.ContentType != "image/png" {
		t.Fatalf("%+v ok=%v", p, ok)
	}
	if len(p.Parts) != 1 {
		t.Fatalf("parts=%d", len(p.Parts))
	}
	asm := p2shAssembly{StartTxID: "aa", StartHeight: 1}
	if !asm.applyPartial(p) || !asm.complete() {
		t.Fatalf("asm=%+v", asm)
	}
	ins := InscriptionFromBody(1, "aa", 0, asm.ContentType, asm.Data, "p2sh")
	if ins.MediaKind != "image" || ins.Source != "p2sh" || !ins.HasContent {
		t.Fatalf("%+v", ins)
	}
}

func TestExtractP2SHMultiPartAndChain(t *testing.T) {
	part1 := []byte("Hello ")
	part2 := []byte("Doginals")
	var tx1 []byte
	tx1 = append(tx1, pushStr("ord")...)
	tx1 = append(tx1, 0x52) // OP_2 pieces=2
	tx1 = append(tx1, pushStr("text/plain;charset=utf-8")...)
	tx1 = append(tx1, 0x51) // separator 1
	tx1 = append(tx1, byte(len(part1)))
	tx1 = append(tx1, part1...)
	sig := make([]byte, 70)
	redeem := make([]byte, 33)
	tx1 = append(tx1, byte(len(sig)))
	tx1 = append(tx1, sig...)
	tx1 = append(tx1, byte(len(redeem)))
	tx1 = append(tx1, redeem...)

	p1, ok := ExtractP2SHInscriptionPartial(tx1)
	if !ok || !p1.StartsOrd || p1.Pieces != 2 {
		t.Fatalf("%+v", p1)
	}
	asm := p2shAssembly{StartTxID: "t1", StartHeight: 10, Vin: 0}
	if !asm.applyPartial(p1) || asm.complete() || asm.Remaining != 1 {
		t.Fatalf("after tx1 %+v", asm)
	}

	var tx2 []byte
	tx2 = append(tx2, 0x00) // separator 0
	tx2 = append(tx2, byte(len(part2)))
	tx2 = append(tx2, part2...)
	tx2 = append(tx2, byte(len(sig)))
	tx2 = append(tx2, sig...)
	tx2 = append(tx2, byte(len(redeem)))
	tx2 = append(tx2, redeem...)

	p2, ok := ExtractP2SHInscriptionPartial(tx2)
	if !ok || p2.StartsOrd || len(p2.Parts) != 1 {
		t.Fatalf("%+v ok=%v", p2, ok)
	}
	if !asm.applyPartial(p2) || !asm.complete() {
		t.Fatalf("after tx2 %+v", asm)
	}
	if string(asm.Data) != "Hello Doginals" {
		t.Fatalf("data=%q", asm.Data)
	}
	ins := InscriptionFromBody(10, "t1", 0, asm.ContentType, asm.Data, "p2sh")
	if ins.MediaKind != "text" || ins.Kind != "doginal" {
		t.Fatalf("%+v", ins)
	}
}

func TestP2SHPendingStoreAndContent(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	asm := p2shAssembly{
		Pieces: 2, Remaining: 1, ContentType: "text/plain",
		Data: []byte("Woof "), StartTxID: "ab", StartHeight: 5, Vin: 0,
	}
	if err := st.PutP2SHPending("deadbeef:0", asm); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.TakeP2SHPending("deadbeef:0")
	if err != nil || !ok || got.Remaining != 1 || string(got.Data) != "Woof " {
		t.Fatalf("%v %v %+v", ok, err, got)
	}
	if _, ok, _ = st.TakeP2SHPending("deadbeef:0"); ok {
		t.Fatal("expected consumed")
	}

	body := []byte(`{"p":"drc-20","op":"mint","tick":"dogi","amt":"100"}`)
	ins := InscriptionFromBody(5, "ab", 0, "application/json", body, "p2sh")
	if err := st.PutInscription(ins); err != nil {
		t.Fatal(err)
	}
	raw, ok, err := st.GetInscriptionBody(ins.ID)
	if err != nil || !ok || string(raw) != string(body) {
		t.Fatalf("%v %v %q", ok, err, raw)
	}
	if ins.MediaKind != "token" {
		t.Fatalf("media=%s", ins.MediaKind)
	}
}

func TestClassifyMediaKind(t *testing.T) {
	if ClassifyMediaKind("image/png", nil, false) != "image" {
		t.Fatal("image")
	}
	if ClassifyMediaKind("application/json", []byte(`{"p":"drc-20"}`), true) != "token" {
		t.Fatal("token")
	}
	if ClassifyMediaKind("application/pdf", []byte{1, 2, 3}, false) != "file" {
		t.Fatal("file")
	}
}
