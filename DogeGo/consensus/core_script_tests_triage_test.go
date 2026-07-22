// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestCoreScriptTestsMismatchSummary(t *testing.T) {
	raw, err := os.ReadFile(coreScriptTestsJSONPath())
	if err != nil {
		t.Skip(err)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
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
		counts[string(want)+" -> "+string(got)]++
	}
	type kv struct {
		k string
		n int
	}
	var list []kv
	for k, n := range counts {
		list = append(list, kv{k, n})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
	for i, e := range list {
		if i >= 12 {
			break
		}
		t.Logf("%4d  %s", e.n, e.k)
	}
}
