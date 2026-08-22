// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package diskspace

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"dogego/analytics"
	"dogego/applog"
)

// Thresholds for the datadir volume during full-block IBD.
const (
	// CriticalFreeBytes is the free-space floor. Body download pauses at or below 1 GiB.
	CriticalFreeBytes = 1 << 30
	PollEvery         = 30 * time.Second
	NotifyCooldown     = 30 * time.Minute
)

// Snapshot is exposed on /api/summary and /api/live for the dashboard banner.
type Snapshot struct {
	Active            bool    `json:"active"`
	Paused            bool    `json:"paused"`
	ContinueAllowed   bool    `json:"continue_allowed"`
	OperatorContinued bool    `json:"operator_continued"`
	UsedPercent       float64 `json:"used_percent"`
	FreeBytes         int64   `json:"free_bytes"`
	TotalBytes        int64   `json:"total_bytes"`
	Path              string  `json:"path,omitempty"`
	Message           string  `json:"message,omitempty"`
	Advice            string  `json:"advice,omitempty"`
}

// PauseFn applies or clears the body-fetch pause on the IBD coordinator.
type PauseFn func(paused bool)

// NotifyFn delivers a desktop/tray warning (optional).
type NotifyFn func(message, advice string)

type state struct {
	mu                sync.Mutex
	path              string
	free              uint64
	total             uint64
	usedPct           float64
	warn              bool
	paused            bool
	operatorContinued bool
	lastNotify        time.Time
	setPause          PauseFn
}

var global atomic.Pointer[state]
var notifyHook atomic.Pointer[NotifyFn]

// SetNotify registers the OS notification hook (optional).
func SetNotify(fn NotifyFn) {
	if fn == nil {
		notifyHook.Store(nil)
		return
	}
	notifyHook.Store(&fn)
}

// lowFree reports whether free space is at or below the 1 GiB pause floor.
func lowFree(free uint64) bool {
	return free <= CriticalFreeBytes
}

// Start polls the datadir volume and pauses full-block getdata near capacity.
func Start(chainRoot string, setPause PauseFn) {
	if chainRoot == "" {
		return
	}
	st := &state{path: chainRoot, setPause: setPause}
	global.Store(st)
	st.poll()
	go func() {
		t := time.NewTicker(PollEvery)
		defer t.Stop()
		for range t.C {
			st.poll()
		}
	}()
}

func (st *state) poll() {
	if st == nil {
		return
	}
	free, total, err := analytics.VolumeUsage(st.path)
	if err != nil || total == 0 {
		return
	}
	usedPct := 100.0 * float64(total-free) / float64(total)
	if usedPct < 0 {
		usedPct = 0
	}
	if usedPct > 100 {
		usedPct = 100
	}

	st.mu.Lock()
	st.free = free
	st.total = total
	st.usedPct = usedPct
	critical := lowFree(free)

	if !critical {
		st.operatorContinued = false
	}

	wasPaused := st.paused
	st.warn = critical
	// Hard pause at or below 1 GiB; auto-resume once free space recovers.
	pause := critical
	if critical {
		st.operatorContinued = false
	}
	st.paused = pause
	notify := false
	if pause && (!wasPaused || time.Since(st.lastNotify) >= NotifyCooldown) {
		st.lastNotify = time.Now()
		notify = true
	}
	msg := st.messageLocked()
	advice := st.adviceLocked()
	setPause := st.setPause
	st.mu.Unlock()

	if setPause != nil {
		setPause(pause)
	}
	if notify {
		applog.Line("storage", msg)
		if p := notifyHook.Load(); p != nil && *p != nil {
			(*p)(msg, advice)
		}
	}
}

func (st *state) messageLocked() string {
	freeGiB := float64(st.free) / (1 << 30)
	totalGiB := float64(st.total) / (1 << 30)
	return fmt.Sprintf(
		"Datadir volume has %.1f GiB free of %.1f GiB (1 GiB or less). Full block download is paused.",
		freeGiB, totalGiB,
	)
}

func (st *state) adviceLocked() string {
	return "Free space on this drive, enlarge the volume if this is a VM or virtual disk, or upgrade to a larger drive. Download resumes automatically once more than 1 GiB is free."
}

// Current returns the current disk pressure state for UI/RPC.
func Current() Snapshot {
	st := global.Load()
	if st == nil {
		return Snapshot{}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.total == 0 {
		return Snapshot{Path: st.path}
	}
	out := Snapshot{
		Active:            st.warn,
		Paused:            st.paused,
		ContinueAllowed:   false, // auto-resume when free > 1 GiB; Continue cannot bypass the floor
		OperatorContinued: st.operatorContinued,
		UsedPercent:       st.usedPct,
		FreeBytes:         int64(st.free),
		TotalBytes:        int64(st.total),
		Path:              st.path,
	}
	if st.warn {
		out.Message = st.messageLocked()
		out.Advice = st.adviceLocked()
	}
	return out
}

// OperatorContinue resumes full-block fetch after a low-disk pause (not at or below 1 GiB free).
func OperatorContinue() (Snapshot, error) {
	st := global.Load()
	if st == nil {
		return Snapshot{}, fmt.Errorf("disk monitor not started")
	}
	st.mu.Lock()
	if st.total == 0 {
		st.mu.Unlock()
		return Snapshot{}, fmt.Errorf("disk usage unknown")
	}
	if lowFree(st.free) {
		st.mu.Unlock()
		return Current(), fmt.Errorf("1 GiB or less free; free space before continuing")
	}
	st.operatorContinued = true
	st.paused = false
	st.warn = false
	setPause := st.setPause
	st.mu.Unlock()
	if setPause != nil {
		setPause(false)
	}
	applog.Line("storage", "operator continued full block download after low disk space recovered")
	return Current(), nil
}
