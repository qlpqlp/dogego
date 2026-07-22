// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/wire"
)

func TestCommittedAuxpowHeaderHex(t *testing.T) {
	hx, ok := CommittedAuxpowHeaderHex(371337)
	if !ok || len(hx) != 160 {
		t.Fatalf("371337: ok=%v len=%d", ok, len(hx))
	}
	hx, ok = CommittedAuxpowHeaderHex(371338)
	if !ok || len(hx) != 160 {
		t.Fatalf("371338: ok=%v len=%d", ok, len(hx))
	}
	if _, ok := CommittedAuxpowHeaderHex(999999); ok {
		t.Fatal("unexpected height")
	}
}

// TestMainnetFieldLegacyScryptHeader371340 verifies post-activation legacy scrypt field_header (no aux version bit).
func TestMainnetFieldLegacyScryptHeader371340(t *testing.T) {
	hx, ok := loadCommittedHeaderHexAt(371340)
	if !ok {
		t.Fatal("core_header_vectors missing mainnet_field_header_371340")
	}
	h80 := mustDecodeHeader80(t, hx)
	if isAuxpowVersionU(nVersionLE(h80)) {
		t.Fatal("height 371340 field_header must be legacy scrypt (no aux version bit)")
	}
	if err := verifyCommittedFieldHeader(chain.MainnetDogecoin, 371340, h80); err != nil {
		t.Fatal(err)
	}
}

func TestTryReadMainnetFieldAuxFromBlockFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	// When headers_aux.bin slot is empty but a bundled body exists, export can recover aux_hex.
	raw, ok := tryReadMainnetFieldBlockRaw(371337)
	if !ok {
		t.Skip("no bundled body at 371337 in DOGEGO_FIELD_DATADIR")
	}
	blob, has, err := wire.ExtractAuxPowBytesFromBlock(raw)
	if err != nil || !has || len(blob) == 0 {
		t.Skip("371337 block has no extractable aux (datadir layout)")
	}
	// Journal may still be empty at 371337 on sparse aux backfill; fallback must match block aux.
	hx, ok := tryReadMainnetFieldAuxHex(371337)
	if !ok {
		t.Fatalf("tryReadMainnetFieldAuxHex(371337) with block present: ok=false")
	}
	want := strings.ToUpper(hex.EncodeToString(blob))
	if hx != want {
		t.Fatalf("aux hex mismatch journal fallback vs block extract")
	}
}

func mustDecodeHeader80(t *testing.T, hx string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimSpace(hx))
	if err != nil || len(b) != 80 {
		t.Fatalf("header_hex: %v len=%d", err, len(b))
	}
	return b
}
