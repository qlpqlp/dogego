// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"strings"
	"testing"
)

type stubJournal struct {
	heights map[string]int64
	tip     int64
}

func (s stubJournal) HeightByDisplayHash(hex string) (int64, error) {
	if h, ok := s.heights[strings.ToLower(hex)]; ok {
		return h, nil
	}
	return 0, fmt.Errorf("not found")
}

func (s stubJournal) TipHeight() (int64, error)           { return s.tip, nil }
func (s stubJournal) ReadHeaderAt(int64) ([]byte, error) { return nil, fmt.Errorf("stub") }

func TestAssumeValidScriptChecks(t *testing.T) {
	hex := DefaultAssumeValidHex("mainnet")
	a := NewAssumeValid("mainnet", hex)
	j := stubJournal{heights: map[string]int64{hex: 5_050_000}, tip: 5_100_000}
	if err := a.Resolve(j); err != nil {
		t.Fatal(err)
	}
	a.SetHeaderTip(5_100_000)
	if !a.ScriptChecksEnabled(5_050_001) {
		t.Fatal("above assume height want scripts")
	}
	if a.ScriptChecksEnabled(5_049_000) {
		t.Fatal("deep history want skip")
	}
	if !a.ScriptChecksEnabled(5_099_000) {
		t.Fatal("tip window want scripts")
	}
}

func TestAssumeValidVerifyAll(t *testing.T) {
	a := NewAssumeValid("mainnet", "0")
	if !a.ScriptChecksEnabled(100) {
		t.Fatal("0 disables skip")
	}
}
