// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dogego/chain"
)

func buildMockSigner(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "mocksigner")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	root := moduleRoot(t)
	cmd := exec.Command("go", "build", "-o", out, "./signer/cmd/mocksigner")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build mocksigner: %v\n%s", err, b)
	}
	return out
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "env", "GOMOD")
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	mod := strings.TrimSpace(string(b))
	if mod == "" || mod == "/dev/null" {
		t.Fatal("GOMOD empty")
	}
	return filepath.Dir(mod)
}

func TestExecEnumerateSignersMock(t *testing.T) {
	mock := buildMockSigner(t)
	paths := &DataPaths{SignerCommand: []string{mock}}
	res, code, msg := execEnumerateSigners(paths, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	list, ok := res.([]interface{})
	if !ok || len(list) != 1 {
		t.Fatalf("result %#v", res)
	}
	dev, _ := list[0].(map[string]interface{})
	if dev["type"] != "mock" {
		t.Fatalf("device %#v", dev)
	}
}

func TestExecSignerDisplayAddressMock(t *testing.T) {
	mock := buildMockSigner(t)
	paths := &DataPaths{SignerCommand: []string{mock}}
	res, code, msg := execSignerDisplayAddress(paths, mustWalletJSONParam(t, "pkh(02abc)"))
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if res != "DMockSignerAddr" {
		t.Fatalf("addr=%v", res)
	}
}

func TestExternalSignerClientSignPSBTMockPassthrough(t *testing.T) {
	mock := buildMockSigner(t)
	c := externalSignerClient(&DataPaths{SignerCommand: []string{mock}})
	signed, err := c.SignPSBT("cHM=")
	if err != nil {
		t.Fatal(err)
	}
	if signed != "cHM=" {
		t.Fatalf("signed=%q", signed)
	}
}

func TestWalletProcessPsbtExternalSignerMock(t *testing.T) {
	mock := buildMockSigner(t)
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	inp, _ := json.Marshal([]map[string]interface{}{{"txid": strings.Repeat("a", 64), "vout": 0}})
	outObj, _ := json.Marshal(map[string]float64{addr: 1.5})
	b64Res, code, msg := execCreatePsbt("test", nil, nil, nil, []json.RawMessage{inp, outObj})
	if code != 0 {
		t.Fatalf("createpsbt: %d %s", code, msg)
	}
	b64, _ := b64Res.(string)
	paths := &DataPaths{
		SignerCommand: []string{mock},
		WalletAddress: func() string { return addr },
		WalletWIFs:    func() []string { return nil },
	}
	psbtParam, _ := json.Marshal(b64)
	signTrue, _ := json.Marshal(true)
	res, code, msg := execWalletProcessPsbt("test", paths, nil, nil, nil, []json.RawMessage{psbtParam, signTrue})
	if code != 0 {
		t.Fatalf("walletprocesspsbt: %d %s", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok || m["psbt"] == "" {
		t.Fatalf("result %#v", res)
	}
}

func TestWalletProcessPsbtExternalSignerFail(t *testing.T) {
	mock := buildMockSigner(t)
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	addr, err := chain.RandomP2PKHAddress(p)
	if err != nil {
		t.Fatal(err)
	}
	inp, _ := json.Marshal([]map[string]interface{}{{"txid": strings.Repeat("b", 64), "vout": 0}})
	outObj, _ := json.Marshal(map[string]float64{addr: 1.0})
	b64Res, code, msg := execCreatePsbt("test", nil, nil, nil, []json.RawMessage{inp, outObj})
	if code != 0 {
		t.Fatalf("createpsbt: %d %s", code, msg)
	}
	b64, _ := b64Res.(string)
	paths := &DataPaths{
		SignerCommand: []string{mock, "--fail"},
		WalletAddress: func() string { return addr },
		WalletWIFs:    func() []string { return nil },
	}
	psbtParam, _ := json.Marshal(b64)
	signTrue, _ := json.Marshal(true)
	_, code, msg = execWalletProcessPsbt("test", paths, nil, nil, nil, []json.RawMessage{psbtParam, signTrue})
	if code == 0 {
		t.Fatal("expected signer failure")
	}
	if !strings.Contains(msg, "mock signer failure") {
		t.Fatalf("msg=%q", msg)
	}
}
