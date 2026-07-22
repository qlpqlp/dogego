// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"testing"
)

func TestParseListSinceBlockIncludeWatchonly(t *testing.T) {
	watchJ, _ := json.Marshal(true)
	_, _, minConf, inc, code, msg := parseListSinceBlockParams(&memJournal{}, []json.RawMessage{
		json.RawMessage(`""`),
		json.RawMessage(`3`),
		watchJ,
	})
	if code != 0 {
		t.Fatalf("parse: %d %s", code, msg)
	}
	if minConf != 3 || !inc {
		t.Fatalf("minconf %d include_watchonly %v", minConf, inc)
	}
	falseJ, _ := json.Marshal(false)
	_, _, _, inc2, code, msg := parseListSinceBlockParams(&memJournal{}, []json.RawMessage{
		json.RawMessage(`null`),
		json.RawMessage(`1`),
		falseJ,
	})
	if code != 0 || inc2 {
		t.Fatalf("false: %d %s inc=%v", code, msg, inc2)
	}
}
