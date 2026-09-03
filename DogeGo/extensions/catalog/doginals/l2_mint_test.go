// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"
)

func TestPrepareL2MintImage(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x01}
	rec, body, err := PrepareL2Mint(map[string]interface{}{
		"address":      "DDummy",
		"kind":         "image",
		"op":           "inscribe",
		"name":         "Test",
		"content_type": "image/png",
		"content_b64":  base64.StdEncoding.EncodeToString(png),
	}, "mainnet")
	if err != nil {
		// address is dummy  -  prepare doesn't validate address format
		t.Fatal(err)
	}
	if rec.Kind != "image" || rec.ContentHash == "" || len(body) != len(png) {
		t.Fatalf("%+v body=%d", rec, len(body))
	}
	msg, err := rec.CanonicalSignMessage()
	if err != nil || msg == "" || rec.Signature != "" {
		t.Fatal(msg, err)
	}
}

func TestAcceptL2MintSignedRoundTrip(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	sk, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := sk.PubKey()
	h160 := hash160(pub.SerializeCompressed())
	addr := chain.Base58CheckEncode(p.PubkeyHashAddrID, h160[:])
	if addr == "" {
		t.Fatal("encode address")
	}
	body := []byte("Woof image bytes")
	rec, rawBody, err := PrepareL2Mint(map[string]interface{}{
		"address":      addr,
		"kind":         "file",
		"op":           "inscribe",
		"name":         "Woof",
		"content_type": "text/plain",
		"content_b64":  base64.StdEncoding.EncodeToString(body),
	}, "testnet")
	if err != nil {
		t.Fatal(err)
	}
	msg, err := rec.CanonicalSignMessage()
	if err != nil {
		t.Fatal(err)
	}
	mh := dogecoinMessageHash(msg)
	sig := ecdsa.SignCompact(sk, mh[:], true)
	rec.Signature = base64.StdEncoding.EncodeToString(sig)

	accepted, gotBody, err := AcceptL2Mint(rec, rawBody, "testnet")
	if err != nil {
		t.Fatal(err)
	}
	if accepted.ID == "" || string(gotBody) != string(body) {
		t.Fatalf("%+v body=%q", accepted, gotBody)
	}

	dir := t.TempDir()
	st, err := OpenStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.PutL2Mint(accepted, gotBody); err != nil {
		t.Fatal(err)
	}
	if err := st.PutL2Mint(accepted, gotBody); err == nil {
		t.Fatal("expected duplicate reject")
	}
	loaded, ok, err := st.GetL2Mint(accepted.ID)
	if err != nil || !ok || loaded.Address != addr {
		t.Fatalf("%v %v %+v", ok, err, loaded)
	}
	b2, ok, err := st.GetL2MintBody(accepted.ID)
	if err != nil || !ok || string(b2) != string(body) {
		t.Fatalf("%v %v %q", ok, err, b2)
	}
}

func TestBuildOrdEnvelopeRoundTrip(t *testing.T) {
	body := []byte("hello ordinal")
	env, err := BuildOrdEnvelope("text/plain", body)
	if err != nil {
		t.Fatal(err)
	}
	parsed, ok := ParseOrdEnvelope(env)
	if !ok || parsed.ContentType != "text/plain" || string(parsed.Body) != string(body) {
		t.Fatalf("ok=%v %+v", ok, parsed)
	}
}

func TestPrepareL2MintOrdinal(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x01}
	rec, body, err := PrepareL2Mint(map[string]interface{}{
		"address":      "DDummy",
		"kind":         "ordinals",
		"op":           "inscribe",
		"name":         "Ord L2",
		"content_type": "image/png",
		"content_b64":  base64.StdEncoding.EncodeToString(png),
	}, "mainnet")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Kind != "ordinal" || rec.Protocol != "ord" || rec.Op != "inscribe" {
		t.Fatalf("%+v", rec)
	}
	env, err := BuildOrdEnvelope(rec.ContentType, body)
	if err != nil {
		t.Fatal(err)
	}
	parsed, ok := ParseOrdEnvelope(env)
	if !ok || parsed.ContentType != "image/png" || len(parsed.Body) != len(png) {
		t.Fatalf("ok=%v %+v", ok, parsed)
	}
}

func TestErrMintL2Only(t *testing.T) {
	err := errMintL2Only()
	if err == nil || !strings.Contains(err.Error(), "L2 only") {
		t.Fatal(err)
	}
}

func TestPrepareTokenRequiresTick(t *testing.T) {
	_, _, err := PrepareL2Mint(map[string]interface{}{
		"address": "DTest",
		"kind":    "token",
		"op":      "mint",
		"amount":  "10",
	}, "mainnet")
	if err == nil {
		t.Fatal("expected tick required")
	}
}
