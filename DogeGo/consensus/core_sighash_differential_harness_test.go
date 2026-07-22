// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"dogego/chain"
	"dogego/wire"
)

// TestCoreSighashDifferentialHarness matches DogeGo legacy sighash to Core sighash.json vectors.
func TestCoreSighashDifferentialHarness(t *testing.T) {
	path := coreSighashJSONPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("sighash.json missing: %v", err)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	var ran, failed int
	for _, row := range rows {
		var cells []json.RawMessage
		if err := json.Unmarshal(row, &cells); err != nil || len(cells) < 5 {
			continue
		}
		var rawTx, rawScript, wantHex string
		var nIn, nHashType int64
		if err := json.Unmarshal(cells[0], &rawTx); err != nil {
			continue
		}
		if err := json.Unmarshal(cells[1], &rawScript); err != nil {
			continue
		}
		if err := json.Unmarshal(cells[2], &nIn); err != nil {
			continue
		}
		if err := json.Unmarshal(cells[3], &nHashType); err != nil {
			continue
		}
		if err := json.Unmarshal(cells[4], &wantHex); err != nil {
			continue
		}
		ran++
		txBytes, err := hex.DecodeString(rawTx)
		if err != nil {
			t.Fatalf("tx hex: %v", err)
		}
		tx, err := wire.ReadTx(bytes.NewReader(txBytes))
		if err != nil {
			t.Fatalf("read tx: %v", err)
		}
		scriptCode, err := hex.DecodeString(rawScript)
		if err != nil {
			t.Fatalf("script hex: %v", err)
		}
		got, err := wire.CalcSignatureHashLegacy(scriptCode, uint32(nHashType), tx, int(nIn))
		if err != nil {
			t.Fatalf("sighash: %v", err)
		}
		want, err := chain.Hash256FromDisplayHex(wantHex)
		if err != nil {
			t.Fatalf("want hex: %v", err)
		}
		if got != want {
			failed++
			if failed <= 3 {
				t.Errorf("sighash mismatch row %d: got %x want display %s", ran, got, wantHex)
			}
			continue
		}
		if ran >= 200 {
			break
		}
	}
	if ran < 50 {
		t.Fatalf("too few sighash vectors: ran=%d failed=%d", ran, failed)
	}
	if failed > 0 {
		t.Fatalf("sighash mismatches: %d of %d", failed, ran)
	}
	t.Logf("sighash vectors OK: %d", ran)
}

func coreSighashJSONPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "src", "test", "data", "sighash.json"))
}
