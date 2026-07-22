// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"flag"
	"testing"
)

func TestVersionFlagSetParsesCheckAndJSON(t *testing.T) {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	check := fs.Bool("check", false, "")
	jsonOut := fs.Bool("json", false, "")
	if err := fs.Parse([]string{"-check", "-json"}); err != nil {
		t.Fatal(err)
	}
	if !*check || !*jsonOut {
		t.Fatalf("check=%v json=%v", *check, *jsonOut)
	}
}
