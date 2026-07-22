// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestExecDogegoRecoverHeadersUnwired(t *testing.T) {
	_, code, msg := execDogegoRecoverHeaders(nil)
	if code == 0 {
		t.Fatal("expected error")
	}
	if msg == "" {
		t.Fatal("expected message")
	}
}

func TestExecDogegoRecoverHeadersNoChange(t *testing.T) {
	paths := &DataPaths{
		RecoverHeaderJournal: func() (int64, int64, bool, error) {
			return 100, 100, false, errRecoverNoChange()
		},
	}
	_, code, _ := execDogegoRecoverHeaders(paths)
	if code == 0 {
		t.Fatal("expected error when journal unchanged and restart unwired")
	}
}

func TestExecDogegoRecoverHeadersRestartWhenStuck(t *testing.T) {
	var restarted bool
	paths := &DataPaths{
		RecoverHeaderJournal: func() (int64, int64, bool, error) {
			return 371336, 371336, false, errRecoverNoChange()
		},
		RestartHeaderSyncIfStuck: func() bool {
			restarted = true
			return true
		},
	}
	out, code, msg := execDogegoRecoverHeaders(paths)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if !restarted {
		t.Fatal("expected restart")
	}
	m, ok := out.(map[string]interface{})
	if !ok || m["dogego_header_sync_restarted"] != true {
		t.Fatalf("out %#v", out)
	}
}

func TestExecDogegoRecoverHeadersIdempotentRestart(t *testing.T) {
	var restarts int
	paths := &DataPaths{
		RecoverHeaderJournal: func() (int64, int64, bool, error) {
			return 371336, 371336, false, errRecoverNoChange()
		},
		RestartHeaderSyncIfStuck: func() bool {
			restarts++
			return true
		},
	}
	for i := 0; i < 2; i++ {
		out, code, msg := execDogegoRecoverHeaders(paths)
		if code != 0 || msg != "" {
			t.Fatalf("pass %d: code=%d msg=%q", i, code, msg)
		}
		m, ok := out.(map[string]interface{})
		if !ok || m["dogego_header_sync_restarted"] != true {
			t.Fatalf("pass %d out %#v", i, out)
		}
	}
	if restarts != 2 {
		t.Fatalf("restarts=%d want 2", restarts)
	}
}

type recoverNoChange struct{}

func (recoverNoChange) Error() string { return "no header journal change (tip still 100)" }

func errRecoverNoChange() error { return recoverNoChange{} }
