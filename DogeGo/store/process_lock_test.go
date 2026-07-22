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

func TestProcessLockExclusive(t *testing.T) {
	dir := t.TempDir()
	a, err := AcquireProcessLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Release()
	_, err = AcquireProcessLock(dir)
	if err == nil {
		t.Fatal("expected second lock to fail")
	}
}

func TestProcessLockReleaseAllowsReacquire(t *testing.T) {
	dir := t.TempDir()
	a, err := AcquireProcessLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	a.Release()
	b, err := AcquireProcessLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	b.Release()
}

func TestProcessLockCreatesFile(t *testing.T) {
	dir := t.TempDir()
	l, err := AcquireProcessLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	if _, err := os.Stat(filepath.Join(dir, ".dogego-process.lock")); err != nil {
		t.Fatal(err)
	}
}

func TestProcessLockStalePIDRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".dogego-process.lock")
	if err := os.WriteFile(path, []byte("pid=999999991\n"), 0600); err != nil {
		t.Fatal(err)
	}
	l, err := AcquireProcessLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	_, err = AcquireProcessLock(dir)
	if err == nil {
		t.Fatal("expected second lock to fail while first held")
	}
}
