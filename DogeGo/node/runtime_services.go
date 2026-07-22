// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"dogego/analytics"
	"dogego/mempool"
	"dogego/ui"
)

// ServiceStatus is one controllable subsystem for the web dashboard.
type ServiceStatus struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Running     bool     `json:"running"`
	Detail      string   `json:"detail,omitempty"`
	CanStart    bool     `json:"can_start"`
	CanStop     bool     `json:"can_stop"`
	Actions     []string `json:"actions,omitempty"`
	RestartNote string   `json:"restart_note,omitempty"`
}

// RuntimeServices exposes start/stop for in-process subsystems (loopback UI only).
type RuntimeServices struct {
	mu sync.RWMutex

	parent       context.Context
	pool         *mempool.Pool
	fullNode     bool
	analyticsCfg func() analytics.SidecarConfig

	analyticsCancel context.CancelFunc
	analyticsRun    bool

	rpcConfigured    atomic.Bool
	rpcListening     atomic.Bool
	rpcDispatchReady atomic.Bool

	p2pStatus func() map[string]any
	mining    *SoloMiningRuntime
}

// RuntimeServicesConfig wires dependencies for RuntimeServices.
type RuntimeServicesConfig struct {
	Parent       context.Context
	Pool         *mempool.Pool
	FullNode     bool
	AnalyticsCfg func() analytics.SidecarConfig
	P2PStatus    func() map[string]any
}

// NewRuntimeServices builds the runtime controller. Analytics is not started until StartAnalytics.
func NewRuntimeServices(c RuntimeServicesConfig) *RuntimeServices {
	s := &RuntimeServices{
		parent:       c.Parent,
		pool:         c.Pool,
		fullNode:     c.FullNode,
		analyticsCfg: c.AnalyticsCfg,
		p2pStatus:    c.P2PStatus,
	}
	if c.Parent == nil {
		s.parent = context.Background()
	}
	return s
}

// SetMining registers reboot-testnet background mining control (optional).
func (s *RuntimeServices) SetMining(m *SoloMiningRuntime) {
	s.mu.Lock()
	s.mining = m
	s.mu.Unlock()
}

// SetP2PStatus sets the live P2P snapshot provider (call after peer manager wiring).
func (s *RuntimeServices) SetP2PStatus(fn func() map[string]any) {
	s.mu.Lock()
	s.p2pStatus = fn
	s.mu.Unlock()
}

// SetRPCConfigured marks whether JSON-RPC was enabled at node startup.
func (s *RuntimeServices) SetRPCConfigured(on bool) {
	s.rpcConfigured.Store(on)
	if !on {
		s.rpcListening.Store(false)
		s.rpcDispatchReady.Store(false)
	}
}

// SetRPCListening updates whether the JSON-RPC HTTP port is bound.
func (s *RuntimeServices) SetRPCListening(on bool) {
	s.rpcListening.Store(on)
	if !on {
		s.rpcDispatchReady.Store(false)
	}
}

// SetRPCDispatchReady updates whether chain RPC methods are wired.
func (s *RuntimeServices) SetRPCDispatchReady(on bool) {
	s.rpcDispatchReady.Store(on)
}

// RPCListening reports whether the JSON-RPC HTTP port is bound.
func (s *RuntimeServices) RPCListening() bool {
	return s.rpcListening.Load()
}

// RPCDispatchReady reports whether chain RPC methods are wired.
func (s *RuntimeServices) RPCDispatchReady() bool {
	return s.rpcDispatchReady.Load()
}

// AnalyticsRunning reports whether the embedded analytics sidecar goroutine is active.
func (s *RuntimeServices) AnalyticsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.analyticsRun
}

// StartAnalytics runs the embedded Pebble sidecar until StopAnalytics or parent ctx ends.
func (s *RuntimeServices) StartAnalytics() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.analyticsRun {
		return fmt.Errorf("analytics sidecar already running")
	}
	if s.analyticsCfg == nil {
		return fmt.Errorf("analytics sidecar not configured")
	}
	actx, cancel := context.WithCancel(s.parent)
	s.analyticsCancel = cancel
	s.analyticsRun = true
	cfg := s.analyticsCfg()
	go func() {
		analytics.RunSidecar(actx, cfg)
		s.mu.Lock()
		s.analyticsRun = false
		s.analyticsCancel = nil
		s.mu.Unlock()
	}()
	return nil
}

// StopAnalytics cancels the embedded analytics sidecar.
func (s *RuntimeServices) StopAnalytics() error {
	s.mu.Lock()
	if !s.analyticsRun || s.analyticsCancel == nil {
		s.mu.Unlock()
		return fmt.Errorf("analytics sidecar not running")
	}
	cancel := s.analyticsCancel
	s.analyticsCancel = nil
	s.analyticsRun = false
	s.mu.Unlock()
	cancel()
	return nil
}

// Status returns dashboard-facing service rows.
func (s *RuntimeServices) Status() []ServiceStatus {
	out := []ServiceStatus{
		{
			ID: "node", Label: "Node process", Running: true,
			Detail:  "P2P, sync, and dashboard run in this process",
			CanStop: true, Actions: []string{"stop", "restart"},
		},
	}
	s.mu.RLock()
	mining := s.mining
	p2pFn := s.p2pStatus
	s.mu.RUnlock()
	if mining != nil {
		out = append(out, mining.ServiceStatus())
	}

	p2pDetail := "connecting…"
	p2pRun := false
	if p2pFn != nil {
		if snap := p2pFn(); snap != nil {
			if v, ok := snap["connections_total"].(int); ok && v > 0 {
				p2pRun = true
				p2pDetail = fmt.Sprintf("%d peer connection(s)", v)
			} else if v, ok := snap["connections_total"].(float64); ok && v > 0 {
				p2pRun = true
				p2pDetail = fmt.Sprintf("%.0f peer connection(s)", v)
			}
			if h, ok := snap["health_message"].(string); ok && h != "" {
				p2pDetail = h
			}
			if addr, ok := snap["peer_addr"].(string); ok && addr != "" {
				p2pRun = true
				if p2pDetail == "connecting…" {
					p2pDetail = "primary peer " + addr
				}
			}
		}
	}
	out = append(out, ServiceStatus{
		ID: "p2p", Label: "P2P network", Running: p2pRun, Detail: p2pDetail,
		RestartNote: "Stop the node to disconnect all peers",
	})

	rpcDetail := "disabled in config"
	rpcRun := false
	canStart := false
	if s.rpcConfigured.Load() {
		rpcDetail = "starting (bind pending)"
		canStart = false
		if s.rpcListening.Load() {
			rpcRun = true
			if s.rpcDispatchReady.Load() {
				rpcDetail = "listening"
			} else {
				rpcDetail = "warming up (port open; methods after chain init)"
			}
		}
	}
	out = append(out, ServiceStatus{
		ID: "rpc", Label: "JSON-RPC", Running: rpcRun, Detail: rpcDetail,
		CanStart: canStart, RestartNote: "Change RPC listen address in config and restart the node",
	})

	mpRun := true
	mpDetail := "accepting transactions"
	mpActions := []string{"pause", "clear"}
	if s.pool != nil {
		if s.pool.Paused() {
			mpRun = false
			mpDetail = fmt.Sprintf("paused - %d tx(s) retained", s.pool.Count())
			mpActions = []string{"resume", "clear"}
		} else {
			mpDetail = fmt.Sprintf("%d tx(s), %d bytes", s.pool.Count(), s.pool.TotalBytes())
		}
	}
	out = append(out, ServiceStatus{
		ID: "mempool", Label: "Mempool relay", Running: mpRun, Detail: mpDetail,
		CanStart: s.pool != nil && s.pool.Paused(),
		CanStop:  s.pool != nil && !s.pool.Paused(),
		Actions:  mpActions,
	})

	anRun := s.AnalyticsRunning()
	anDetail := "stopped"
	if anRun {
		anDetail = "updating dogego_analytics.db"
		if !s.fullNode {
			anDetail += " (SPV: headers only)"
		}
	}
	anActions := []string{"start"}
	if anRun {
		anActions = []string{"stop"}
	}
	out = append(out, ServiceStatus{
		ID: "analytics", Label: "Analytics indexer (embedded)", Running: anRun, Detail: anDetail,
		CanStart:    s.analyticsCfg != nil && !anRun,
		CanStop:     anRun,
		Actions:     anActions,
		RestartNote: "CLI: dogego indexer status|scan - separate from this sidecar",
	})

	return out
}

// ServiceRows implements ui.ServiceController.
func (s *RuntimeServices) ServiceRows() []ui.ServiceRow {
	sts := s.Status()
	out := make([]ui.ServiceRow, len(sts))
	for i, st := range sts {
		out[i] = ui.ServiceRow{
			ID: st.ID, Label: st.Label, Running: st.Running, Detail: st.Detail,
			CanStart: st.CanStart, CanStop: st.CanStop, Actions: st.Actions, RestartNote: st.RestartNote,
		}
	}
	return out
}

// ApplyServiceAction implements ui.ServiceController.
func (s *RuntimeServices) ApplyServiceAction(id, action string) error {
	return s.applyAction(id, action)
}

// applyAction applies a runtime action. Actions: start, stop, pause, resume, clear.
func (s *RuntimeServices) applyAction(serviceID, action string) error {
	switch serviceID {
	case "analytics":
		switch action {
		case "start":
			return s.StartAnalytics()
		case "stop":
			return s.StopAnalytics()
		default:
			return fmt.Errorf("unknown action %q for analytics", action)
		}
	case "mempool":
		if s.pool == nil {
			return fmt.Errorf("mempool not available")
		}
		switch action {
		case "pause", "stop":
			s.pool.SetPaused(true)
			return nil
		case "resume", "start":
			s.pool.SetPaused(false)
			return nil
		case "clear":
			s.pool.Clear()
			return nil
		default:
			return fmt.Errorf("unknown action %q for mempool", action)
		}
	case "mining":
		s.mu.RLock()
		mining := s.mining
		s.mu.RUnlock()
		if mining == nil {
			return fmt.Errorf("mining control not available")
		}
		switch action {
		case "start":
			return mining.Start()
		case "stop":
			return mining.Stop()
		case "restart":
			return mining.Restart()
		default:
			return fmt.Errorf("unknown action %q for mining", action)
		}
	case "rpc":
		return fmt.Errorf("JSON-RPC cannot be started or stopped at runtime; edit config and restart the node")
	case "p2p":
		return fmt.Errorf("P2P cannot be stopped independently; use Stop node or restart")
	case "node":
		return fmt.Errorf("use /api/control/shutdown or /api/control/restart for the node process")
	default:
		return fmt.Errorf("unknown service %q", serviceID)
	}
}
