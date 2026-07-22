// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import "testing"

type fakeRelayBook struct {
	scores map[string]int
	order  []string
}

func (f *fakeRelayBook) NoteTry(addr string) {
	f.order = append(f.order, "try:"+addr)
}

func (f *fakeRelayBook) NoteSuccess(addr string) {
	f.order = append(f.order, "ok:"+addr)
}

func (f *fakeRelayBook) NoteFailure(addr string) {
	f.order = append(f.order, "fail:"+addr)
}

func (f *fakeRelayBook) RelayDialScore(addr string) int {
	if f.scores == nil {
		return 0
	}
	return f.scores[addr]
}

func TestSortTargetsByReputation(t *testing.T) {
	targets := []string{"b.example:24433", "a.example:24433", "c.example:24433"}
	book := &fakeRelayBook{scores: map[string]int{
		"a.example:24433": 200,
		"b.example:24433": 50,
		"c.example:24433": 100,
	}}
	SortTargetsByReputation(targets, book)
	if targets[0] != "a.example:24433" || targets[1] != "c.example:24433" || targets[2] != "b.example:24433" {
		t.Fatalf("order %v", targets)
	}
}
