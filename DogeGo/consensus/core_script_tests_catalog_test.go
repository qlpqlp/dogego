// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCoreScriptTestsJSONCatalog documents coverage of Core's script_tests.json corpus
// (EvalScript runner + template verifier; witness rows intentionally skipped).
func TestCoreScriptTestsJSONCatalog(t *testing.T) {
	path := coreScriptTestsJSONPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("Core script_tests.json not found at %s: %v", path, err)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	var cases, comments int
	for _, row := range rows {
		var cells []json.RawMessage
		if err := json.Unmarshal(row, &cells); err != nil {
			continue
		}
		if len(cells) < 4 {
			comments++
			continue
		}
		var scriptSig string
		if err := json.Unmarshal(cells[0], &scriptSig); err != nil {
			comments++
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(scriptSig), "Format") || strings.HasPrefix(scriptSig, "It is") {
			comments++
			continue
		}
		cases++
	}
	if cases < 100 {
		t.Fatalf("expected large script_tests corpus, got %d cases", cases)
	}
	supported := len(loadCoreScriptVectors(t))
	t.Logf("Core script_tests.json: %d executable cases, %d header/comment rows; DogeGo template vectors: %d", cases, comments, supported)
}

func coreScriptTestsJSONPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "src", "test", "data", "script_tests.json"))
}
