// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileBanManagerPersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "banlist.json")
	f := LoadFileBanManager(path)
	if err := f.SetBan("192.0.2.1", "add", 3600, false); err != nil {
		t.Fatal(err)
	}
	f2 := LoadFileBanManager(path)
	list := f2.ListBanned()
	if len(list) != 1 {
		t.Fatalf("list %#v", list)
	}
	if list[0]["address"].(string) != "192.0.2.1" {
		t.Fatalf("address %#v", list[0])
	}
	until := list[0]["banned_until"].(int64)
	if until <= time.Now().Unix() {
		t.Fatalf("banned_until %d", until)
	}
	f2.ClearBanned()
	if len(f2.ListBanned()) != 0 {
		t.Fatal("expected empty after clear")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []banFileEntry
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("file rows %#v", rows)
	}
}
