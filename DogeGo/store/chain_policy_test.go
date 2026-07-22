// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"path/filepath"
	"testing"
)

func TestChainPolicyInvalidPersist(t *testing.T) {
	dir := t.TempDir()
	p, err := LoadChainPolicy(dir)
	if err != nil {
		t.Fatal(err)
	}
	h := repeatHex('a')
	if err := p.AddInvalid(h); err != nil {
		t.Fatal(err)
	}
	p2, err := LoadChainPolicy(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !p2.IsInvalid(h) {
		t.Fatal("not persisted")
	}
	if err := p2.RemoveInvalid(h); err != nil {
		t.Fatal(err)
	}
	p3, err := LoadChainPolicy(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p3.IsInvalid(h) {
		t.Fatal("still invalid")
	}
	if filepath.Base(p.path) != "chain_policy.json" {
		t.Fatal(p.path)
	}
}

func repeatHex(b byte) string {
	s := make([]byte, 64)
	for i := range s {
		s[i] = b
	}
	return string(s)
}
