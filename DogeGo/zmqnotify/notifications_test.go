// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package zmqnotify

import "testing"

func TestActiveNotificationsCoreShape(t *testing.T) {
	cfg := Config{
		PubHashBlock: "28332",
		PubRawTx:     "tcp://127.0.0.1:28335",
	}
	rows := cfg.ActiveNotifications()
	if len(rows) != 2 {
		t.Fatalf("len=%d want 2", len(rows))
	}
	if rows[0]["type"] != "pubhashblock" || rows[0]["address"] != "tcp://28332" {
		t.Fatalf("hashblock=%#v", rows[0])
	}
	if rows[1]["type"] != "pubrawtx" {
		t.Fatalf("rawtx=%#v", rows[1])
	}
}
