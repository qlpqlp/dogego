// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestCoreScriptTestsFailureDump logs first failures per mismatch class (DOGEGO_SCRIPT_DUMP=1).
func TestCoreScriptTestsFailureDump(t *testing.T) {
	if os.Getenv("DOGEGO_SCRIPT_DUMP") == "" {
		t.Skip("set DOGEGO_SCRIPT_DUMP=1")
	}
	raw, err := os.ReadFile(coreScriptTestsJSONPath())
	if err != nil {
		t.Skip(err)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, row := range rows {
		st, ok := parseScriptTestRow(row)
		if !ok || st.SkipReason != "" {
			continue
		}
		sig, err := ParseScriptASM(st.ScriptSig)
		if err != nil {
			continue
		}
		pub, err := ParseScriptASM(st.ScriptPubKey)
		if err != nil {
			continue
		}
		flags := ParseScriptTestFlags(st.Flags)
		if strings.Contains(st.Flags, "P2SH") {
			flags |= ScriptVerifyP2SH
		}
		var got ScriptError
		if scriptTestNeedsSpendContext(st.ScriptSig, st.ScriptPubKey, flags) {
			got = VerifyScriptTestSpend(sig, pub, flags)
		} else {
			got = VerifyScriptTest(sig, pub, flags)
		}
		want := ScriptError(st.Want)
		if got == want {
			continue
		}
		key := string(want) + " -> " + string(got)
		if seen[key] >= 2 {
			continue
		}
		seen[key]++
		t.Logf("--- %s ---", key)
		t.Logf("flags=%q", st.Flags)
		t.Logf("sig=%s", truncStr(st.ScriptSig, 120))
		t.Logf("pub=%s", truncStr(st.ScriptPubKey, 120))
	}
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
