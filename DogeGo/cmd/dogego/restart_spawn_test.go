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

func TestFilterRestartArgsStripsReplaceTarget(t *testing.T) {
	got := filterRestartArgs([]string{"node", "-datadir=x", "-replacetarget=C:\\old.exe", "-waitpid=9"})
	for _, a := range got {
		if a == "-replacetarget=C:\\old.exe" || a == "-waitpid=9" {
			t.Fatalf("should strip helper flags: %v", got)
		}
	}
	if !hasFlag(got, "-datadir") {
		t.Fatalf("datadir missing: %v", got)
	}
}

func TestBuildApplyChildArgs(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"dogego.exe", "node", "-datadir", "C:\\data"}
	args := buildApplyChildArgs(55, "C:\\Program Files\\dogego\\dogego.exe")
	foundReplace := false
	foundWait := false
	for _, a := range args {
		if a == "-replacetarget=C:\\Program Files\\dogego\\dogego.exe" {
			foundReplace = true
		}
		if a == "-waitpid=55" {
			foundWait = true
		}
	}
	if !foundReplace || !foundWait {
		t.Fatalf("args=%v", args)
	}
}
