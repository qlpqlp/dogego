// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package consensus

import (
	"encoding/hex"
	"testing"
)

func TestZKAnchorDetectAndBuild(t *testing.T) {
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}
	script, err := BuildZKAnchorScript(hash)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := DetectZKAnchor(script)
	if !ok {
		t.Fatal("detect failed")
	}
	if a.Tag != ZKAnchorTag {
		t.Fatalf("tag %q", a.Tag)
	}
	if a.AnchorHash != hex.EncodeToString(hash) {
		t.Fatalf("hash %q", a.AnchorHash)
	}
	res, err := VerifyZKAnchorScriptHex(hex.EncodeToString(script))
	if err != nil {
		t.Fatal(err)
	}
	if res["valid"] != true {
		t.Fatalf("verify %#v", res)
	}
}
