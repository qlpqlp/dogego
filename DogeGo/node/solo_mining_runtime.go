// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SoloMiningRuntime starts and stops the reboot-testnet background solo miner from the web dashboard.
type SoloMiningRuntime struct {
	mu sync.Mutex

	parent context.Context
	active *atomic.Bool
	kick   chan struct{}

	cancel context.CancelFunc
	opts   SoloMinerOpts

	eligible       func() (ok bool, detail string)
	mineRequested  func() bool
	payoutAddress  string
	restartNote    string
}

// SoloMiningRuntimeConfig wires the solo mining controller.
type SoloMiningRuntimeConfig struct {
	Parent        context.Context
	Active        *atomic.Bool
	Kick          chan struct{}
	Eligible      func() (ok bool, detail string)
	MineRequested func() bool
	PayoutAddress string
	RestartNote   string
}

// NewSoloMiningRuntime builds a mining controller. Call Configure before Start.
func NewSoloMiningRuntime(c SoloMiningRuntimeConfig) *SoloMiningRuntime {
	if c.Parent == nil {
		c.Parent = context.Background()
	}
	return &SoloMiningRuntime{
		parent:        c.Parent,
		active:        c.Active,
		kick:          c.Kick,
		eligible:      c.Eligible,
		mineRequested: c.MineRequested,
		payoutAddress: strings.TrimSpace(c.PayoutAddress),
		restartNote:   c.RestartNote,
	}
}

// Configure sets RunSoloMiner dependencies (call once chain paths are ready).
func (m *SoloMiningRuntime) Configure(opts SoloMinerOpts) {
	m.mu.Lock()
	defer m.mu.Unlock()
	opts.Active = m.active
	opts.MineKick = m.kick
	m.opts = opts
	if addr := strings.TrimSpace(opts.MiningAddr); addr != "" {
		m.payoutAddress = addr
	}
}

// Start runs the background solo miner until Stop or parent context ends.
func (m *SoloMiningRuntime) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active != nil && m.active.Load() {
		return fmt.Errorf("mining already running")
	}
	ok, msg := m.eligibility()
	if !ok {
		if msg == "" {
			msg = "mining unavailable on this network or node mode"
		}
		return fmt.Errorf("%s", msg)
	}
	if strings.TrimSpace(m.opts.MiningAddr) == "" {
		return fmt.Errorf("mining address required (enable wallet or set miningaddress)")
	}
	if m.cancel != nil {
		return fmt.Errorf("mining stop still in progress")
	}
	mctx, cancel := context.WithCancel(m.parent)
	m.cancel = cancel
	opts := m.opts
	opts.Active = m.active
	opts.MineKick = m.kick
	go func() {
		RunSoloMiner(mctx, opts)
		m.mu.Lock()
		if m.cancel != nil {
			m.cancel = nil
		}
		m.mu.Unlock()
	}()
	return nil
}

// Stop cancels the background solo miner.
func (m *SoloMiningRuntime) Stop() error {
	m.mu.Lock()
	cancel := m.cancel
	m.cancel = nil
	m.mu.Unlock()
	if cancel == nil {
		if m.active != nil && m.active.Load() {
			return fmt.Errorf("mining stop already in progress")
		}
		return fmt.Errorf("mining not running")
	}
	cancel()
	return nil
}

// Restart stops then starts the background solo miner.
func (m *SoloMiningRuntime) Restart() error {
	_ = m.Stop()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if m.active == nil || !m.active.Load() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return m.Start()
}

func (m *SoloMiningRuntime) eligibility() (bool, string) {
	if m.eligible != nil {
		return m.eligible()
	}
	return false, "mining not configured"
}

// ServiceStatus returns a dashboard service row for GET /api/services.
func (m *SoloMiningRuntime) ServiceStatus() ServiceStatus {
	active := m.active != nil && m.active.Load()
	ok, eligDetail := m.eligibility()
	mineReq := false
	if m.mineRequested != nil {
		mineReq = m.mineRequested()
	}
	detail := eligDetail
	if active {
		detail = "background solo miner active (~15s interval)"
		if m.payoutAddress != "" {
			detail += " · payout " + m.payoutAddress
		}
	} else if ok {
		if mineReq {
			detail = "configured (mine=true) but stopped this run"
		} else {
			detail = "stopped this run (config mine=false)"
		}
	}
	row := ServiceStatus{
		ID:    "mining",
		Label: "Solo mining (background)",
		Running: active,
		Detail:  detail,
	}
	if m.restartNote != "" {
		row.RestartNote = m.restartNote
	}
	if !ok {
		row.RestartNote = eligDetail
		return row
	}
	if active {
		row.CanStop = true
		row.Actions = []string{"stop", "restart"}
	} else {
		row.CanStart = true
		row.Actions = []string{"start"}
	}
	return row
}
