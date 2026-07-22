// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ibdconvergence

import (
	"fmt"
	"time"

	"dogego/walletmigration"
)

// Options configures a timed IBD convergence check.
type Options struct {
	IntervalSec             int
	MinContiguousAdvance      int64
	MinBlocksAdvance          int64
	MinRawProbeAdvance        int64
	MaxContiguousRegression   int64
	DiskOnly                  bool
	DataDir                   string
	Network                   string
	RPC                       walletmigration.RPCClient
	WebURL                    string
}

// VerifyResult reports whether forward IBD progress occurred between two snapshots.
type VerifyResult struct {
	OK                bool             `json:"ok"`
	T0                ProgressSnapshot `json:"t0"`
	T1                ProgressSnapshot `json:"t1"`
	ContiguousAdvance int64            `json:"contiguous_advance"`
	BlockAdvance      int64            `json:"block_advance"`
	ProbeAdvance      int64            `json:"probe_advance"`
	BodyCoveragePct   float64          `json:"body_coverage_pct,omitempty"`
	ConnectRatePerMin float64          `json:"connect_rate_per_min,omitempty"`
	Issues            []string         `json:"issues,omitempty"`
	Notes             []string         `json:"notes,omitempty"`
	Doc               string           `json:"doc,omitempty"`
}

// Verify waits IntervalSec between two progress snapshots and checks forward movement.
func Verify(opts Options) VerifyResult {
	if opts.IntervalSec <= 0 {
		opts.IntervalSec = 120
	}
	if opts.MinContiguousAdvance <= 0 {
		opts.MinContiguousAdvance = 1
	}
	if opts.MinBlocksAdvance <= 0 {
		opts.MinBlocksAdvance = 1
	}
	if opts.MinRawProbeAdvance <= 0 {
		opts.MinRawProbeAdvance = 1
	}
	if opts.MaxContiguousRegression <= 0 {
		opts.MaxContiguousRegression = 64
	}
	out := VerifyResult{
		Doc: "docs/OPERATOR.md; mirrors scripts/ibd_convergence_check.ps1",
	}
	chainDir := ""
	if opts.DataDir != "" {
		if dir, err := ResolveChainDir(opts.DataDir, opts.Network); err == nil {
			chainDir = dir
		} else {
			out.Issues = append(out.Issues, "chain_dir: "+err.Error())
		}
	}
	snapOpts := SnapshotOptions{
		DiskOnly:   opts.DiskOnly,
		ChainDir:   chainDir,
		WebURL:     opts.WebURL,
		RPC:        opts.RPC,
		RPCTimeout: 45 * time.Second,
	}
	t0, err := CollectSnapshot(snapOpts)
	if err != nil {
		out.Issues = append(out.Issues, err.Error())
		return out
	}
	out.T0 = t0
	time.Sleep(time.Duration(opts.IntervalSec) * time.Second)
	t1, err := CollectSnapshot(snapOpts)
	if err != nil {
		out.Issues = append(out.Issues, "t1: "+err.Error())
		return out
	}
	out.T1 = t1
	return CompareSnapshots(out, opts)
}

// CompareSnapshots evaluates progress between two snapshots (unit-testable).
func CompareSnapshots(out VerifyResult, opts Options) VerifyResult {
	a, b := out.T0, out.T1
	if a.Contiguous != nil && b.Contiguous != nil {
		out.ContiguousAdvance = *b.Contiguous - *a.Contiguous
	}
	if a.Blocks != nil && b.Blocks != nil {
		out.BlockAdvance = *b.Blocks - *a.Blocks
	}
	if a.RawProbe != nil && b.RawProbe != nil {
		out.ProbeAdvance = *b.RawProbe - *a.RawProbe
	}
	if out.ContiguousAdvance < -opts.MaxContiguousRegression {
		out.Issues = append(out.Issues, fmt.Sprintf("contiguous regression %d (drop > %d)", out.ContiguousAdvance, opts.MaxContiguousRegression))
		return out
	}
	if b.ReplayTarget != nil && b.Contiguous != nil && *b.ReplayTarget > *b.Contiguous+1 {
		remain := *b.ReplayTarget - *b.Contiguous
		pct := 100.0 * float64(*b.Contiguous) / float64(*b.ReplayTarget)
		out.Notes = append(out.Notes, fmt.Sprintf("snapshot body replay: %d/%d (%.1f%%; ~%d left)", *b.Contiguous, *b.ReplayTarget, pct, remain))
	}
	if opts.IntervalSec > 0 && out.BlockAdvance > 0 {
		out.ConnectRatePerMin = float64(out.BlockAdvance) * 60.0 / float64(opts.IntervalSec)
	}
	if a.Contiguous != nil && b.Contiguous != nil && a.Blocks != nil && b.Blocks != nil {
		lag0 := *a.Contiguous - *a.Blocks
		lag1 := *b.Contiguous - *b.Blocks
		if lag0 > 0 || lag1 > 0 {
			out.Notes = append(out.Notes, fmt.Sprintf("connect lag: %d -> %d (delta %d)", lag0, lag1, lag1-lag0))
		}
	}
	ok := out.ContiguousAdvance >= opts.MinContiguousAdvance ||
		out.BlockAdvance >= opts.MinBlocksAdvance ||
		out.ProbeAdvance >= opts.MinRawProbeAdvance
	if !ok && connectCaughtUp(a) && connectCaughtUp(b) {
		if b.RawInFlight != nil && *b.RawInFlight > 0 {
			out.Notes = append(out.Notes, fmt.Sprintf("body-only IBD: connect caught up (blocks=contiguous=%d); in_flight_batches=%d", *b.Contiguous, *b.RawInFlight))
			ok = true
		}
	}
	if !ok {
		out.Issues = append(out.Issues, "no measurable IBD progress in window (node stopped or stalled)")
		return out
	}
	out.OK = true
	if b.Headers != nil && b.Contiguous != nil && *b.Headers > 0 {
		out.BodyCoveragePct = 100.0 * float64(*b.Contiguous) / float64(*b.Headers)
	}
	return out
}

func connectCaughtUp(s ProgressSnapshot) bool {
	if s.Blocks == nil || s.Contiguous == nil {
		return false
	}
	return *s.Blocks == *s.Contiguous && *s.Contiguous >= 0
}
