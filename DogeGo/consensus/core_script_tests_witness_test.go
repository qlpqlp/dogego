// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/json"
	"os"
	"testing"
)

// TestCoreScriptTestsWitnessRowsIntentionallyDeclined documents segwit witness rows in Core script_tests.json
// that DogeGo skips by design (Dogecoin segwit disabled; witness rejected at mempool admission).
func TestCoreScriptTestsWitnessRowsIntentionallyDeclined(t *testing.T) {
	path := coreScriptTestsJSONPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("script_tests.json missing: %v", err)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	var witness int
	for _, row := range rows {
		st, ok := parseScriptTestRow(row)
		if !ok {
			continue
		}
		if st.SkipReason == "witness" {
			witness++
		}
	}
	if witness < 20 {
		t.Fatalf("expected at least 20 witness script_tests rows, got %d", witness)
	}
	t.Logf("script_tests witness rows intentionally declined: %d (Dogecoin segwit disabled)", witness)
}
