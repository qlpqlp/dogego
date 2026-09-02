// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"
	"strconv"
	"sync/atomic"
)

// ContiguousReconcileStatus is a lock-free snapshot of startup body reconcile work
// (blk*.dat probe + journal measure) for the WebUI progress bar.
type ContiguousReconcileStatus struct {
	Active  bool
	Phase   string // "probe" | "measure"
	Detail  string
	Percent float64 // 0–100; -1 when unknown
	Current int64
	Total   int64
}

type contiguousReconcileState struct {
	active        atomic.Bool
	phase         atomic.Value // string
	detail        atomic.Value // string
	percentX100   atomic.Int64 // -100 = unknown; else percent*100 (monotonic while active)
	current       atomic.Int64
	total         atomic.Int64
	probeDone     atomic.Bool
	measureOrigin atomic.Int64 // height progress is measured from
}

const contiguousReconcileProbeWeight = 70.0

var contiguousReconcile contiguousReconcileState

func init() {
	contiguousReconcile.phase.Store("")
	contiguousReconcile.detail.Store("")
	contiguousReconcile.percentX100.Store(-100)
}

// BeginContiguousReconcile marks startup disk reconcile as in progress (WebUI).
func BeginContiguousReconcile() {
	contiguousReconcile.active.Store(true)
	contiguousReconcile.probeDone.Store(false)
	contiguousReconcile.phase.Store("probe")
	contiguousReconcile.detail.Store("Checking stored blocks...")
	contiguousReconcile.percentX100.Store(0)
	contiguousReconcile.current.Store(0)
	contiguousReconcile.total.Store(0)
	contiguousReconcile.measureOrigin.Store(0)
}

// EndContiguousReconcile clears startup disk reconcile progress.
func EndContiguousReconcile() {
	contiguousReconcile.active.Store(false)
	contiguousReconcile.probeDone.Store(false)
	contiguousReconcile.phase.Store("")
	contiguousReconcile.detail.Store("")
	contiguousReconcile.percentX100.Store(-100)
	contiguousReconcile.current.Store(0)
	contiguousReconcile.total.Store(0)
	contiguousReconcile.measureOrigin.Store(0)
}

// ContiguousReconcileProgress returns the current startup reconcile status, if any.
func ContiguousReconcileProgress() (ContiguousReconcileStatus, bool) {
	if !contiguousReconcile.active.Load() {
		return ContiguousReconcileStatus{}, false
	}
	phase, _ := contiguousReconcile.phase.Load().(string)
	detail, _ := contiguousReconcile.detail.Load().(string)
	px := contiguousReconcile.percentX100.Load()
	pct := -1.0
	if px >= 0 {
		pct = float64(px) / 100.0
	}
	return ContiguousReconcileStatus{
		Active:  true,
		Phase:   phase,
		Detail:  detail,
		Percent: pct,
		Current: contiguousReconcile.current.Load(),
		Total:   contiguousReconcile.total.Load(),
	}, true
}

func storeContiguousPercent(pct float64) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	x := int64(pct * 100)
	for {
		old := contiguousReconcile.percentX100.Load()
		// Never move the bar backwards while a reconcile is active.
		if old >= 0 && x < old {
			return
		}
		if contiguousReconcile.percentX100.CompareAndSwap(old, x) {
			return
		}
	}
}

func reportContiguousProbe(bytesDone, bytesTotal int64, fileLabel string) {
	if !contiguousReconcile.active.Load() {
		return
	}
	contiguousReconcile.phase.Store("probe")
	contiguousReconcile.current.Store(bytesDone)
	contiguousReconcile.total.Store(bytesTotal)
	// Wording: "Verifying" — this is a restart disk check, not IBD from height 0.
	detail := "Verifying stored block files..."
	if fileLabel != "" {
		detail = fmt.Sprintf("Verifying %s...", fileLabel)
	}
	if bytesTotal > 0 {
		frac := float64(bytesDone) / float64(bytesTotal)
		if frac > 1 {
			frac = 1
		}
		storeContiguousPercent(frac * contiguousReconcileProbeWeight)
		detail = fmt.Sprintf("%s (%d%%)", detail, int(frac*100))
	} else {
		storeContiguousPercent(0)
	}
	contiguousReconcile.detail.Store(detail)
}

func reportContiguousCheckpointVerified(height int64) {
	if !contiguousReconcile.active.Load() {
		return
	}
	contiguousReconcile.phase.Store("measure")
	contiguousReconcile.current.Store(height)
	contiguousReconcile.total.Store(height)
	storeContiguousPercent(100)
	contiguousReconcile.detail.Store(fmt.Sprintf("Checkpoint tip verified at height %s", formatHeight(height)))
}

func reportContiguousProbeDone() {
	if !contiguousReconcile.active.Load() {
		return
	}
	contiguousReconcile.probeDone.Store(true)
	storeContiguousPercent(contiguousReconcileProbeWeight)
}

func reportContiguousMeasure(height, origin, target int64) {
	if !contiguousReconcile.active.Load() {
		return
	}
	contiguousReconcile.phase.Store("measure")
	contiguousReconcile.current.Store(height)
	contiguousReconcile.measureOrigin.Store(origin)
	base := contiguousReconcileProbeWeight
	span := 100.0 - base

	// Always prefer a real end target so the bar is monotonic and never uses height%N.
	end := target
	if end < 0 || end < origin {
		end = contiguousReconcile.total.Load()
	}
	if end < origin {
		end = origin
	}
	if end > origin {
		contiguousReconcile.total.Store(end)
	}

	detail := fmt.Sprintf("Verifying bodies through height %s...", formatHeight(height))
	if end > origin {
		frac := float64(height-origin) / float64(end-origin)
		if height < origin {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		storeContiguousPercent(base + frac*span)
		detail = fmt.Sprintf("Verifying bodies %s / %s...", formatHeight(height), formatHeight(end))
	} else {
		// Tip already at origin (seed==probe): park in the measure band without oscillating.
		storeContiguousPercent(base + span*0.5)
	}
	contiguousReconcile.detail.Store(detail)
}

func formatHeight(h int64) string {
	if h < 0 {
		return "?"
	}
	s := strconv.FormatInt(h, 10)
	n := len(s)
	if n <= 3 {
		return s
	}
	out := make([]byte, 0, n+n/3)
	for i := 0; i < n; i++ {
		if i > 0 && (n-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}
