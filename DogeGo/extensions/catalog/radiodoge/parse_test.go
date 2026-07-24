// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package radiodoge

import "testing"

func TestParseConfirmations(t *testing.T) {
	body := `{"logs":["[1] [DISPLAY] RX: Dconfirmation:DOGECOIN_RESPONSE:{\"result\":\"abc123\",\"error\":null}","noise"]}`
	cs := ParseConfirmations(body)
	if len(cs) != 1 || !cs[0].OK || cs[0].Result != "abc123" {
		t.Fatalf("got %#v", cs)
	}
	if !MatchConfirmation(body, "abc123") {
		t.Fatal("expected match")
	}
}

func TestExtractTxHexCandidates(t *testing.T) {
	hex := "0100000001" + stringsRepeat("ab", 60)
	body := `{"logs":["RX message=` + hex + `"]}`
	got := ExtractTxHexCandidates(body)
	if len(got) != 1 || got[0] != hex {
		t.Fatalf("got %#v", got)
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
