// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dogeos

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const ExtensionID = "dogego.dogeos"

// Host is the minimal bridge the subprocess needs from DogeGo.
type Host interface {
	Log(line string)
}

// Service owns config, metrics polling, and RPC helpers.
type Service struct {
	dataDir string
	host    Host
	store   *configStore
	metrics *metricsRing

	mu     sync.Mutex
	stopCh chan struct{}
	wg     sync.WaitGroup
}

func NewService(dataDir string, host Host) *Service {
	return &Service{
		dataDir: strings.TrimSpace(dataDir),
		host:    host,
		store:   loadConfig(dataDir),
		metrics: &metricsRing{},
	}
}

func (s *Service) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopCh != nil {
		return
	}
	s.stopCh = make(chan struct{})
	s.wg.Add(1)
	go s.pollLoop(s.stopCh)
	s.log("dogeos: started (network=" + s.store.Get().NetworkID + ")")
}

func (s *Service) Stop() {
	s.mu.Lock()
	ch := s.stopCh
	s.stopCh = nil
	s.mu.Unlock()
	if ch != nil {
		close(ch)
		s.wg.Wait()
	}
	s.log("dogeos: stopped")
}

func (s *Service) log(line string) {
	if s.host != nil {
		s.host.Log(line)
	}
}

func (s *Service) Config() Config { return s.store.Get() }

func (s *Service) SetConfig(patch map[string]interface{}) (Config, error) {
	cfg := s.store.Get()
	if v, ok := patch["network_id"].(string); ok {
		id := strings.TrimSpace(v)
		if id != "" {
			if _, ok := FindNetwork(id); !ok {
				return cfg, fmt.Errorf("unknown network_id %q", id)
			}
			cfg.NetworkID = id
		}
	}
	if v, ok := patch["custom_rpc_url"].(string); ok {
		cfg.CustomRPCURL = strings.TrimSpace(v)
	}
	if v, ok := asInt(patch["poll_seconds"]); ok {
		cfg.PollSeconds = v
	}
	if v, ok := patch["metrics_enabled"].(bool); ok {
		cfg.MetricsEnabled = v
	}
	if err := s.store.Set(cfg); err != nil {
		return cfg, err
	}
	return s.store.Get(), nil
}

func (s *Service) Client() (*Client, NetworkProfile, error) {
	n, rpc, err := s.store.EffectiveRPC()
	if err != nil {
		return nil, n, err
	}
	return NewClient(rpc), n, nil
}

func (s *Service) client() (*Client, NetworkProfile, error) { return s.Client() }

func (s *Service) EffectiveNetwork() (NetworkProfile, string, error) {
	return s.store.EffectiveRPC()
}

func (s *Service) ProbeNow(ctx context.Context) (ProbeResult, error) {
	c, n, err := s.client()
	if err != nil {
		p := ProbeResult{OK: false, Error: err.Error(), ProbedAt: time.Now().Unix(), ExpectedChainID: n.ChainID}
		s.metrics.Push(p)
		return p, err
	}
	p := c.Probe(ctx, n.ChainID)
	s.metrics.Push(p)
	return p, nil
}

func (s *Service) Snapshot() map[string]interface{} {
	cfg := s.store.Get()
	n, rpc, rpcErr := s.store.EffectiveRPC()
	sum := s.metrics.Summary()
	last, _ := sum["last"].(ProbeResult)
	out := map[string]interface{}{
		"extension":       ExtensionID,
		"config":          cfg,
		"network":         n,
		"networks":        BuiltInNetworks(),
		"rpc_url":         rpc,
		"metrics":         sum,
		"last_probe":      last,
		"helpers":         Helpers(n, rpc),
		"docs":            "https://docs.dogeos.com/en/developers",
		"recorded_unix":   time.Now().Unix(),
		"not_dogecoin_l1": true,
		"evm_layer":       true,
	}
	if rpcErr != nil {
		out["rpc_error"] = rpcErr.Error()
	}
	out["ui"] = BuildUI(out)
	return out
}

func (s *Service) pollLoop(stop <-chan struct{}) {
	defer s.wg.Done()
	// Immediate first probe so the UI is not empty.
	ctx, cancel := context.WithTimeout(context.Background(), defaultRPCTimeout)
	_, _ = s.ProbeNow(ctx)
	cancel()

	for {
		cfg := s.store.Get()
		sec := cfg.PollSeconds
		if sec < 5 {
			sec = 5
		}
		t := time.NewTimer(time.Duration(sec) * time.Second)
		select {
		case <-stop:
			t.Stop()
			return
		case <-t.C:
		}
		if !cfg.MetricsEnabled {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultRPCTimeout)
		p, err := s.ProbeNow(ctx)
		cancel()
		if err != nil {
			s.log("dogeos: probe " + err.Error())
			continue
		}
		if !p.OK {
			s.log("dogeos: probe unhealthy: " + p.Error)
		}
	}
}

func asInt(v interface{}) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		return n, err == nil
	}
	return 0, false
}
