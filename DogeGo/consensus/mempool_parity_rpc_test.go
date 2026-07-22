// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"os"
	"reflect"
	"testing"
)

func TestMempoolParityRPCFixtureMatchesBuilders(t *testing.T) {
	live, err := BuildMempoolParityRPCRows()
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := LoadMempoolParityRPCRows()
	if err != nil {
		t.Fatalf("load fixture (run with UPDATE_MEMPOOL_PARITY_RPC=1 to generate): %v", err)
	}
	if !reflect.DeepEqual(live, onDisk) {
		t.Fatalf("mempool_parity_rpc.json stale; run: UPDATE_MEMPOOL_PARITY_RPC=1 go test ./consensus -run TestUpdateMempoolParityRPCFixture -count=1")
	}
	if len(onDisk) < 3 {
		t.Fatalf("expected at least 3 stateless rows, got %d", len(onDisk))
	}
}

func TestUpdateMempoolParityRPCFixture(t *testing.T) {
	if os.Getenv("UPDATE_MEMPOOL_PARITY_RPC") != "1" {
		t.Skip("set UPDATE_MEMPOOL_PARITY_RPC=1 to regenerate testdata/mempool_parity_rpc.json")
	}
	if err := WriteMempoolParityRPCFixture(); err != nil {
		t.Fatal(err)
	}
}
