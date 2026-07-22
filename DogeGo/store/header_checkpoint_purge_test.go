// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPurgeStaleHeaderSyncTemps(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, headerSyncFile+".tmp")
	if err := os.WriteFile(tmp, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := PurgeStaleHeaderSyncTemps(dir)
	if err != nil || n != 1 {
		t.Fatalf("purge n=%d err=%v", n, err)
	}
}
