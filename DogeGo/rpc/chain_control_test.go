// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestExecInvalidateBlockBadHash(t *testing.T) {
	_, code, msg := execInvalidateBlock(nil, nil, []json.RawMessage{json.RawMessage(`"nothex"`)})
	if code != -8 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecInvalidateBlockNoPaths(t *testing.T) {
	_, code, msg := execInvalidateBlock(nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('a') + `"`)})
	if code != -1 || msg == "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecInvalidateBlockNotFound(t *testing.T) {
	paths := &DataPaths{
		InvalidateBlock: func(string) error { return fmt.Errorf("block not found") },
	}
	_, code, msg := execInvalidateBlock(nil, paths, []json.RawMessage{json.RawMessage(`"` + repeatHex('a') + `"`)})
	if code != -5 || msg != "Block not found" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecPreciousBlockNotFound(t *testing.T) {
	paths := &DataPaths{
		MarkPreciousBlock: func(string) error { return fmt.Errorf("block not found") },
	}
	_, code, msg := execPreciousBlock(nil, paths, []json.RawMessage{json.RawMessage(`"` + repeatHex('b') + `"`)})
	if code != -5 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecReconsiderBlockNotFound(t *testing.T) {
	paths := &DataPaths{
		ReconsiderBlock: func(string) error { return fmt.Errorf("block not found") },
	}
	_, code, msg := execReconsiderBlock(paths, []json.RawMessage{json.RawMessage(`"` + repeatHex('c') + `"`)})
	if code != -5 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func repeatHex(b byte) string {
	s := make([]byte, 64)
	for i := range s {
		s[i] = b
	}
	return string(s)
}
