// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func TestMainnetHeight1HeaderAndStubValidation(t *testing.T) {
	seg := filepath.Join("..", "dogedata", "mainnet", "headers", "seg", "0000000000.bin")
	b, err := os.ReadFile(seg)
	if err != nil {
		t.Skip("no local mainnet segments:", err)
	}
	if len(b) < 160 {
		t.Skipf("segment too short for height 1 header (%d bytes, need 160; sync mainnet headers or use field-evidence export)", len(b))
	}
	h1 := b[80:160]
	want := pow.BlockHashLE(h1)
	t.Logf("height 1 display hash %s", pow.BlockHashHex(h1))
	t.Logf("height 1 LE hash %x", want[:8])

	stubPath := filepath.Join("..", "dogedata", "mainnet", "rawblocks", "a2bed9893f8670dffb312ae9d6de7f883a792937316e6b59c034608f0368bc82.bin")
	raw, err := os.ReadFile(stubPath)
	if err != nil {
		stub := store.MakeTestBlockRaw(t, h1)
		raw = stub
		t.Logf("using MakeTestBlockRaw len=%d", len(raw))
	} else {
		t.Logf("on-disk stub len=%d", len(raw))
	}
	if err := wire.ValidateBlockPayload(raw, want); err != nil {
		t.Logf("ValidateBlockPayload: %v", err)
	} else {
		t.Log("ValidateBlockPayload: OK")
	}
	minB := store.MinRawBlockBytes(chain.MainnetDogecoin, 1)
	t.Logf("MinRawBlockBytes height 1 = %d", minB)
	if len(raw) >= minB {
		t.Log("payload meets min size for height 1")
	} else {
		t.Logf("payload below min (%d < %d)", len(raw), minB)
	}
}
