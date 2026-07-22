// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
)

func TestExecGenerateToAddressRejectsAuxHeight(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j := &memJournal{tip: 158100, count: 158101, hdrs: [][]byte{append([]byte(nil), g80[:]...)}}
	addr, _ := chain.RandomP2PKHAddress(p)
	addrJ, _ := json.Marshal(addr)
	_, code, msg := execGenerateToAddress(j, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
		json.RawMessage(`1`),
		addrJ,
		json.RawMessage(`1`),
	})
	if code != -1 || msg == "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestMineLegacyScryptPoWRebootTestnet(t *testing.T) {
	if testing.Short() {
		t.Skip("scrypt mining integration test (run without -short or use fixtures)")
	}
	if runtime.GOOS == "windows" && os.Getenv("DOGEGO_RUN_SCRYPT_MINE") != "1" {
		t.Skip("scrypt mining is slow on windows; set DOGEGO_RUN_SCRYPT_MINE=1 to run")
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	if p.RelaxedPoW {
		t.Fatal("reboot testnet uses real scrypt PoW (RelaxedPoW=false)")
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderJournal(filepath.Join(t.TempDir(), "headers.bin"), g80[:])
	if err != nil {
		t.Fatal(err)
	}
	h160 := rebootTestnetMiningH160(t, p)
	_, payload1, err := mineLegacyBlockToAddress(j, nil, nil, nil, nil, p, chain.RebootTestnet, h160, defaultGenerateMaxTries)
	if err != nil {
		t.Fatalf("mine height 1: %v", err)
	}
	if err := pow.CheckScryptPoW(payload1[:80], binary.LittleEndian.Uint32(payload1[72:76])); err != nil {
		t.Fatalf("height 1 PoW: %v", err)
	}
	if _, err := consensus.ExtendHeadersFromPayload(j, nil, p, payload1, 0, time.Now().Unix()); err != nil {
		t.Fatalf("extend height 1: %v", err)
	}
	_, payload2, err := mineLegacyBlockToAddress(j, nil, nil, nil, nil, p, chain.RebootTestnet, h160, defaultGenerateMaxTries)
	if err != nil {
		t.Fatalf("mine height 2: %v", err)
	}
	blockTime := binary.LittleEndian.Uint32(payload2[68:72])
	gotBits := binary.LittleEndian.Uint32(payload2[72:76])
	expBits, err := consensus.NextBlockBits(j, chain.RebootTestnet, 2, blockTime)
	if err != nil {
		t.Fatal(err)
	}
	if gotBits != expBits {
		t.Fatalf("height 2 nBits: got 0x%x want 0x%x (Digishield nBits)", gotBits, expBits)
	}
	if _, err := consensus.ExtendHeadersFromPayload(j, nil, p, payload2, 1, time.Now().Unix()); err != nil {
		t.Fatalf("extend height 2: %v", err)
	}
	if tip, _ := j.TipHeight(); tip != 2 {
		t.Fatalf("tip=%d want 2", tip)
	}
	if os.Getenv("DOGEGO_GEN_REBOOT_FIXTURES") == "1" {
		writeRebootTestnetMinedFixture(t, 1, payload1)
		writeRebootTestnetMinedFixture(t, 2, payload2)
		_, payload3, err := mineLegacyBlockToAddress(j, nil, nil, nil, nil, p, chain.RebootTestnet, h160, 10_000_000)
		if err != nil {
			t.Fatalf("mine height 3 for fixture: %v", err)
		}
		if _, err := consensus.ExtendHeadersFromPayload(j, nil, p, payload3, 2, time.Now().Unix()); err != nil {
			t.Fatalf("extend height 3: %v", err)
		}
		writeRebootTestnetMinedFixture(t, 3, payload3)
	}
}

func writeRebootTestnetMinedFixture(t *testing.T, height int, payload []byte) {
	t.Helper()
	dir := filepath.Join("testdata", "reboottestnet_mined")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fmt.Sprintf("block%d.hex", height))
	if err := os.WriteFile(path, []byte(hex.EncodeToString(payload)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", path)
}
