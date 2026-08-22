// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package diskspace

import "testing"

func TestCurrentEmpty(t *testing.T) {
	global.Store(nil)
	got := Current()
	if got.Active || got.Paused {
		t.Fatalf("unexpected active snapshot without monitor: %+v", got)
	}
}

func TestOperatorContinueWithoutMonitor(t *testing.T) {
	global.Store(nil)
	if _, err := OperatorContinue(); err == nil {
		t.Fatal("expected error when monitor not started")
	}
}

func TestLowFreeThreshold(t *testing.T) {
	if lowFree(CriticalFreeBytes) {
		// 1 GiB exactly must pause ("1 GB or less")
	} else {
		t.Fatal("expected pause at exactly 1 GiB free")
	}
	if lowFree(CriticalFreeBytes - 1) {
		// ok
	} else {
		t.Fatal("expected pause below 1 GiB")
	}
	if lowFree(CriticalFreeBytes + 1) {
		t.Fatal("must not pause with more than 1 GiB free")
	}
	// High used percent with plenty of free space must not matter.
	if lowFree(20 << 30) {
		t.Fatal("20 GiB free must not pause")
	}
}

func TestPauseCallback(t *testing.T) {
	var paused bool
	st := &state{
		path:     t.TempDir(),
		total:    100,
		free:     10,
		usedPct:  90,
		warn:     true,
		paused:   true,
		setPause: func(p bool) { paused = p },
	}
	global.Store(st)
	st.setPause(true)
	if !paused {
		t.Fatal("expected pause callback")
	}
	st.operatorContinued = true
	st.paused = false
	st.setPause(false)
	if paused {
		t.Fatal("expected resume callback")
	}
	global.Store(nil)
}

func TestOperatorContinueBlockedAtFloor(t *testing.T) {
	st := &state{
		path:   t.TempDir(),
		total:  100 << 30,
		free:   CriticalFreeBytes,
		warn:   true,
		paused: true,
	}
	global.Store(st)
	defer global.Store(nil)
	if _, err := OperatorContinue(); err == nil {
		t.Fatal("expected error at 1 GiB free")
	}
}
