// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dogeos

import (
	"sync"
	"time"
)

const maxMetricSamples = 120

// MetricSample is one probe point for charts / history.
type MetricSample struct {
	At          int64 `json:"at"`
	OK          bool  `json:"ok"`
	LatencyMS   int64 `json:"latency_ms"`
	BlockNumber int64 `json:"block_number"`
	ChainID     int64 `json:"chain_id"`
	GasGwei     string `json:"gas_gwei,omitempty"`
	Error       string `json:"error,omitempty"`
}

type metricsRing struct {
	mu      sync.Mutex
	samples []MetricSample
	last    ProbeResult
}

func (m *metricsRing) Push(p ProbeResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.last = p
	s := MetricSample{
		At:          p.ProbedAt,
		OK:          p.OK,
		LatencyMS:   p.LatencyMS,
		BlockNumber: p.BlockNumber,
		ChainID:     p.ChainID,
		GasGwei:     p.GasPriceGwei,
		Error:       p.Error,
	}
	m.samples = append(m.samples, s)
	if len(m.samples) > maxMetricSamples {
		m.samples = append([]MetricSample(nil), m.samples[len(m.samples)-maxMetricSamples:]...)
	}
}

func (m *metricsRing) Snapshot() (ProbeResult, []MetricSample) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := append([]MetricSample(nil), m.samples...)
	return m.last, out
}

func (m *metricsRing) Summary() map[string]interface{} {
	last, hist := m.Snapshot()
	var okN, failN int
	var latSum int64
	var latN int
	var maxBlock int64
	cutoff := time.Now().Add(-15 * time.Minute).Unix()
	for _, s := range hist {
		if s.At < cutoff {
			continue
		}
		if s.OK {
			okN++
			latSum += s.LatencyMS
			latN++
		} else {
			failN++
		}
		if s.BlockNumber > maxBlock {
			maxBlock = s.BlockNumber
		}
	}
	avg := int64(0)
	if latN > 0 {
		avg = latSum / int64(latN)
	}
	uptime := 0.0
	if tot := okN + failN; tot > 0 {
		uptime = float64(okN) / float64(tot) * 100
	}
	return map[string]interface{}{
		"last":            last,
		"history":         hist,
		"samples":         len(hist),
		"ok_15m":          okN,
		"fail_15m":        failN,
		"uptime_pct_15m":  uptime,
		"avg_latency_ms":  avg,
		"tip_block_seen":  maxBlock,
		"window_minutes":  15,
	}
}
