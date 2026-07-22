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

type scriptTestRowComment struct {
	ScriptSig    string
	ScriptPubKey string
	Flags        string
	Want         string
	Comment      string
	SkipReason   string
}

// TestCoreScriptTestsDERSIGCorpus replays Core script_tests.json rows that exercise BIP66 / DERSIG /
// lax DER padding (pre-activation blockchain compatibility).
func TestCoreScriptTestsDERSIGCorpus(t *testing.T) {
	path := coreScriptTestsJSONPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("script_tests.json missing: %v", err)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	var ran, passed, failed int
	for _, row := range rows {
		st, ok := parseScriptTestRowWithComment(row)
		if !ok || st.SkipReason != "" {
			continue
		}
		if !scriptTestRowIsDERSIGCorpus(st) {
			continue
		}
		sig, err := ParseScriptASM(st.ScriptSig)
		if err != nil {
			t.Fatalf("%s: scriptSig asm: %v", st.Comment, err)
		}
		pub, err := ParseScriptASM(st.ScriptPubKey)
		if err != nil {
			t.Fatalf("%s: scriptPubKey asm: %v", st.Comment, err)
		}
		flags := ParseScriptTestFlags(st.Flags)
		if strings.Contains(st.Flags, "P2SH") {
			flags |= ScriptVerifyP2SH
		}
		ran++
		var got ScriptError
		if scriptTestNeedsSpendContext(st.ScriptSig, st.ScriptPubKey, flags) {
			got = VerifyScriptTestSpend(sig, pub, flags)
		} else {
			got = VerifyScriptTest(sig, pub, flags)
		}
		want := ScriptError(st.Want)
		if got != want {
			failed++
			t.Errorf("%s: got %s want %s (flags=%q)", st.Comment, got, want, st.Flags)
			continue
		}
		passed++
	}
	if ran < 40 {
		t.Fatalf("expected at least 40 DERSIG/BIP66 corpus rows, ran=%d", ran)
	}
	if failed > 0 {
		t.Fatalf("DERSIG corpus failures: ran=%d passed=%d failed=%d", ran, passed, failed)
	}
	t.Logf("DERSIG/BIP66 corpus: ran=%d passed=%d", ran, passed)
}

func scriptTestRowIsDERSIGCorpus(st scriptTestRowComment) bool {
	up := strings.ToUpper(st.Comment + " " + st.Flags)
	if strings.Contains(up, "DERSIG") {
		return true
	}
	if strings.Contains(up, "BIP66") {
		return true
	}
	if strings.Contains(st.Comment, "padding") {
		return true
	}
	return false
}

func parseScriptTestRowWithComment(row json.RawMessage) (scriptTestRowComment, bool) {
	var cells []json.RawMessage
	if err := json.Unmarshal(row, &cells); err != nil || len(cells) < 4 {
		return scriptTestRowComment{}, false
	}
	if len(cells) > 0 {
		var arr []json.RawMessage
		if json.Unmarshal(cells[0], &arr) == nil && len(arr) > 0 {
			return scriptTestRowComment{SkipReason: "witness"}, true
		}
	}
	var sig, pub, flags, want, comment string
	if err := json.Unmarshal(cells[0], &sig); err != nil {
		return scriptTestRowComment{}, false
	}
	if strings.HasPrefix(sig, "Format") || strings.HasPrefix(sig, "It is") || strings.HasPrefix(sig, "Increase") {
		return scriptTestRowComment{}, false
	}
	if err := json.Unmarshal(cells[1], &pub); err != nil {
		return scriptTestRowComment{}, false
	}
	if err := json.Unmarshal(cells[2], &flags); err != nil {
		return scriptTestRowComment{}, false
	}
	if err := json.Unmarshal(cells[3], &want); err != nil {
		return scriptTestRowComment{}, false
	}
	if len(cells) >= 5 {
		_ = json.Unmarshal(cells[4], &comment)
	}
	st := scriptTestRowComment{
		ScriptSig: sig, ScriptPubKey: pub, Flags: flags, Want: want, Comment: comment,
	}
	if strings.Contains(flags, "WITNESS") {
		st.SkipReason = "witness"
	}
	return st, true
}
