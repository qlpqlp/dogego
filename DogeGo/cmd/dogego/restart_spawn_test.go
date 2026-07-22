// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"os"
	"testing"
)

func TestBuildRestartChildArgsPreservesFlags(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"dogego.exe", "node", "-datadir", "C:\\data", "-network", "testnet", "-waitpid=999"}
	args := buildRestartChildArgs(1234)
	if args[0] != "node" {
		t.Fatalf("subcommand = %q, want node", args[0])
	}
	if !hasFlag(args, "-datadir") || !hasFlag(args, "-network") {
		t.Fatalf("expected datadir/network preserved: %v", args)
	}
	for _, a := range args {
		if a == "-waitpid=999" {
			t.Fatalf("old waitpid should be stripped: %v", args)
		}
	}
	want := "-waitpid=1234"
	found := false
	for _, a := range args {
		if a == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing %q in %v", want, args)
	}
	if !hasFlag(args, "-nobrowser") {
		t.Fatalf("expected -nobrowser in %v", args)
	}
}

func TestParseWaitPIDArg(t *testing.T) {
	if got := parseWaitPIDArg([]string{"node", "-waitpid=42"}); got != 42 {
		t.Fatalf("got %d want 42", got)
	}
	if got := parseWaitPIDArg([]string{"node"}); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}
