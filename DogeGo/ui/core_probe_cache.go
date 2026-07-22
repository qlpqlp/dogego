// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"dogego/config"
)

const defaultCoreProbeCacheTTL = 90 * time.Second

type coreProbeCacheStore struct {
	mu   sync.Mutex
	cert CoreOperatorCertResult
	at   time.Time
	ttl  time.Duration
}

var coreProbeCache coreProbeCacheStore

func (c *coreProbeCacheStore) ttlOrDefault() time.Duration {
	if c.ttl > 0 {
		return c.ttl
	}
	return defaultCoreProbeCacheTTL
}

func (c *coreProbeCacheStore) peek() (CoreOperatorCertResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cert.CheckedAt == "" || time.Since(c.at) >= c.ttlOrDefault() {
		return CoreOperatorCertResult{}, false
	}
	out := c.cert
	out.Cached = true
	out.CacheAgeSec = int(time.Since(c.at).Seconds())
	return out, true
}

func (c *coreProbeCacheStore) operatorCert(network, dogeRPCAddr, chainDataDir string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}, refresh bool) CoreOperatorCertResult {
	c.mu.Lock()
	if !refresh && c.cert.CheckedAt != "" && time.Since(c.at) < c.ttlOrDefault() {
		out := c.cert
		c.mu.Unlock()
		out.Cached = true
		out.CacheAgeSec = int(time.Since(c.at).Seconds())
		return out
	}
	c.mu.Unlock()

	cert := RunCoreOperatorCert(network, dogeRPCAddr, chainDataDir, conf, invoke)

	c.mu.Lock()
	c.cert = cert
	c.at = time.Now()
	c.mu.Unlock()
	return cert
}

// WarmCoreProbeCache runs operator cert probes once after startup (best-effort; populates /api/summary cert fields).
func WarmCoreProbeCache(network, dogeRPCAddr, chainDataDir string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) {
	if invoke == nil {
		return
	}
	coreProbeCache.mu.Lock()
	fresh := coreProbeCache.cert.CheckedAt != "" && time.Since(coreProbeCache.at) < coreProbeCache.ttlOrDefault()
	coreProbeCache.mu.Unlock()
	if fresh {
		return
	}
	go warmCoreProbeCacheAsync(network, dogeRPCAddr, chainDataDir, conf, invoke)
}

func warmCoreProbeCacheAsync(network, dogeRPCAddr, chainDataDir string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if rpcInvokeReady(invoke) {
			break
		}
		time.Sleep(2 * time.Second)
	}
	time.Sleep(2 * time.Second)
	_ = coreProbeCache.operatorCert(network, dogeRPCAddr, chainDataDir, conf, invoke, false)
}

func rpcInvokeReady(invoke func(string, []json.RawMessage) map[string]interface{}) bool {
	if invoke == nil {
		return false
	}
	res := invoke("getblockchaininfo", nil)
	if res == nil {
		return false
	}
	if errObj, ok := res["error"].(map[string]interface{}); ok && errObj != nil {
		if code, ok := errObj["code"].(float64); ok && int(code) == -28 {
			return false
		}
		if msg, _ := errObj["message"].(string); strings.Contains(strings.ToLower(msg), "warming up") {
			return false
		}
	}
	_, ok := res["result"]
	return ok
}

// ResetCoreProbeCache clears cached probe results (tests).
func ResetCoreProbeCache() {
	coreProbeCache.mu.Lock()
	defer coreProbeCache.mu.Unlock()
	coreProbeCache.cert = CoreOperatorCertResult{}
	coreProbeCache.at = time.Time{}
}
