// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dogego/extensions"
)

func TestZKL2PrepareAnchor(t *testing.T) {
	ext, err := NewExtension(extensions.Manifest{})
	if err != nil {
		t.Fatal(err)
	}
	e := ext.(*Extension)
	dir := t.TempDir()
	host := &fakeHost{dir: dir, network: "testnet"}
	if err := e.OnEnable(nil, host); err != nil {
		t.Fatal(err)
	}
	defer e.OnDisable()
	hdr := L2BlockHeader{
		Version:     1,
		L2Height:    1,
		ParentHash:  strings.Repeat("0", 64),
		StateRoot:   strings.Repeat("a", 64),
		ProofDigest: strings.Repeat("b", 64),
		ProofMode:   ProofModeGroth16,
	}
	raw, _ := json.Marshal(hdr)
	out, err := e.HandleRPC("prepareanchor", []json.RawMessage{raw}, host)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]interface{})
	if !ok || m["anchor_hash"] == "" {
		t.Fatalf("prepare %#v", out)
	}
}

func TestZKL2StoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	blk := L2Block{
		Header: L2BlockHeader{
			Version:     1,
			L2Height:    1,
			ParentHash:  strings.Repeat("0", 64),
			StateRoot:   strings.Repeat("c", 64),
			ProofDigest: strings.Repeat("d", 64),
			ProofMode:   ProofModeGroth16,
		},
	}
	if err := st.PutL2Block(blk); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GetL2Block(1)
	if err != nil || !ok || got.Header.L2Height != 1 {
		t.Fatalf("get ok=%v err=%v got=%+v", ok, err, got)
	}
}

type fakeHost struct {
	dir     string
	network string
}

func (f *fakeHost) Network() string                            { return f.network }
func (f *fakeHost) TipHeight() (int64, error)                  { return 0, nil }
func (f *fakeHost) GetRawBlockByHeight(int64) ([]byte, error)  { return nil, nil }
func (f *fakeHost) LookupTxHex(string) (string, int64, bool)   { return "", 0, false }
func (f *fakeHost) BlockHashAtHeight(int64) (string, error)    { return "", fmt.Errorf("unwired") }
func (f *fakeHost) ConfirmedTxInBlock(string, string) (uint32, bool) { return 0, false }
func (f *fakeHost) DataDir() string                            { return f.dir }
func (f *fakeHost) ExtensionDataDir(id string) (string, error) { return filepath.Join(f.dir, id), nil }
func (f *fakeHost) Log(string)                                   {}

type fakeWalletHost struct {
	fakeHost
	sig string
}

func (f *fakeWalletHost) CallWalletRPC(method string, params []json.RawMessage) (interface{}, error) {
	if method != "signmessage" {
		return nil, fmt.Errorf("unexpected %q", method)
	}
	if f.sig == "" {
		f.sig = "test-signature"
	}
	return f.sig, nil
}

func TestZKL2SignAnchor(t *testing.T) {
	ext, err := NewExtension(extensions.Manifest{})
	if err != nil {
		t.Fatal(err)
	}
	e := ext.(*Extension)
	dir := t.TempDir()
	host := &fakeWalletHost{fakeHost: fakeHost{dir: dir, network: "testnet"}}
	if err := e.OnEnable(nil, host); err != nil {
		t.Fatal(err)
	}
	defer e.OnDisable()
	hdr := L2BlockHeader{
		Version:       1,
		L2Height:      1,
		ParentHash:    strings.Repeat("0", 64),
		StateRoot:     strings.Repeat("a", 64),
		ProofDigest:   strings.Repeat("b", 64),
		ProofMode:     ProofModeGroth16,
		SignerAddress: "nTestSignerAddress",
	}
	raw, _ := json.Marshal(hdr)
	out, err := e.HandleRPC("signanchor", []json.RawMessage{raw}, host)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]interface{})
	if !ok || m["signed"] != true || m["signature"] != "test-signature" {
		t.Fatalf("signanchor %#v", out)
	}
}
func (f *fakeHost) MkdirAll() error                              { return os.MkdirAll(filepath.Join(f.dir, ExtensionID), 0o755) }
