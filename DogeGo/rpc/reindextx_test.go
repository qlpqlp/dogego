// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestExecReindexTx_EmptyChainDir(t *testing.T) {
	dir := t.TempDir()
	paths := &DataPaths{ChainDataDir: dir}
	res, code, msg := execReindexTx(paths, nil)
	if code != 0 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok {
		t.Fatalf("result %#v", res)
	}
	if v, ok := m["blocks_indexed"].(int); ok && v != 0 {
		t.Fatalf("blocks_indexed %d", v)
	}
}

func TestExecReindexTx_NoChainDir(t *testing.T) {
	_, code, msg := execReindexTx(nil, nil)
	if code != -1 {
		t.Fatalf("code=%d msg=%s", code, msg)
	}
}
