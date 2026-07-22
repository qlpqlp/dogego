// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	_ "embed"
	"regexp"
	"testing"
)

//go:embed dispatch.go
var dispatchGoSource string

// TestDispatchCasesListedInSupportedMethods ensures every dispatch switch arm is advertised in help.
func TestDispatchCasesListedInSupportedMethods(t *testing.T) {
	re := regexp.MustCompile(`\n\tcase "([a-z][a-z0-9_]*)":`)
	supported := make(map[string]struct{}, len(SupportedMethods()))
	for _, m := range SupportedMethods() {
		supported[m] = struct{}{}
	}
	seen := make(map[string]struct{})
	for _, sub := range re.FindAllStringSubmatch(dispatchGoSource, -1) {
		m := sub[1]
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		if _, ok := supported[m]; !ok {
			t.Fatalf("dispatch handles %q but SupportedMethods() omits it", m)
		}
	}
}

// dispatchCaseMethods returns every RPC name listed in dispatch switch cases (incl. combined case lines).
func dispatchCaseMethods() []string {
	caseLine := regexp.MustCompile(`(?m)^\s*case ([^:]+):`)
	nameRe := regexp.MustCompile(`"([a-z][a-z0-9_]*)"`)
	seen := make(map[string]struct{})
	var out []string
	for _, sub := range caseLine.FindAllStringSubmatch(dispatchGoSource, -1) {
		for _, nm := range nameRe.FindAllStringSubmatch(sub[1], -1) {
			m := nm[1]
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			out = append(out, m)
		}
	}
	return out
}

// TestSupportedMethodsImplementedInDispatch ensures help-listed RPCs have a dispatch case.
func TestSupportedMethodsImplementedInDispatch(t *testing.T) {
	cases := dispatchCaseMethods()
	caseSet := make(map[string]struct{}, len(cases))
	for _, m := range cases {
		caseSet[m] = struct{}{}
	}
	for _, m := range SupportedMethods() {
		if _, ok := caseSet[m]; ok {
			continue
		}
		if isExtensionMethod(m) {
			continue
		}
		t.Fatalf("SupportedMethods lists %q but dispatch.go switch has no case for it (have %d cases)", m, len(cases))
	}
}
