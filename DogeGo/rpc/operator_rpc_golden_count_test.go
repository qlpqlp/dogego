// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

const operatorRPCGoldenSubtestCount = 1568

// TestOperatorRPCGoldenSubtestCount locks the golden operator RPC error subtest scope in CI.
func TestOperatorRPCGoldenSubtestCount(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "operator_rpc_errors_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	got := len(regexp.MustCompile(`t\.Run\("`).FindAll(data, -1))
	if got != operatorRPCGoldenSubtestCount {
		t.Fatalf("operator RPC golden subtests: got %d want %d (update operatorRPCGoldenSubtestCount when adding cases)", got, operatorRPCGoldenSubtestCount)
	}
}
