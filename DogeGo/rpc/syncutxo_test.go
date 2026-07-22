// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"testing"
	"time"
)

func TestExecSyncUtxoCacheAlias(t *testing.T) {
	// syncutxocache dispatches to execSyncUtxo in dispatch.go (same handler as syncutxo).
	paths := &DataPaths{SyncUtxo: func() error { return nil }}
	res, code, msg := execSyncUtxo(paths, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	if res == nil {
		t.Fatal("nil result")
	}
}

func TestExecSyncUtxoAsync(t *testing.T) {
	done := make(chan struct{})
	paths := &DataPaths{
		SyncUtxo: func() error { return nil },
		SyncUtxoBounded: func(maxBlocks int) error {
			<-done
			return nil
		},
	}
	res, code, msg := execSyncUtxo(paths, nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	m, ok := res.(map[string]interface{})
	if !ok || m["dogego_syncutxo_async"] != true {
		t.Fatalf("result=%v", res)
	}
	if !SyncUtxoRPCInFlight() {
		t.Fatal("expected in-flight while background sync runs")
	}
	res2, code2, _ := execSyncUtxo(paths, nil)
	if code2 != 0 {
		t.Fatal(code2)
	}
	m2 := res2.(map[string]interface{})
	if m2["already_in_flight"] != true {
		t.Fatalf("second call=%v", res2)
	}
	close(done)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if !SyncUtxoRPCInFlight() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("syncutxo still in flight")
}
