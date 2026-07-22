// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"testing"
	"time"

	"dogego/config"
)

func TestCoreOperatorCertMatrixNoCache(t *testing.T) {
	out := CoreOperatorCertMatrix(nil)
	if !out.MatrixOnly || len(out.Rows) < 20 {
		t.Fatalf("matrix: %+v", out)
	}
	for _, r := range out.Rows {
		if r.WebProbe && r.OK != nil {
			t.Fatalf("uncached matrix should not have ok: %s", r.ID)
		}
	}
}

func TestCoreProbeCacheHit(t *testing.T) {
	ResetCoreProbeCache()
	coreProbeCache.ttl = 2 * time.Minute
	invoke := func(method string, params []json.RawMessage) map[string]interface{} {
		return map[string]interface{}{"error": map[string]interface{}{"code": float64(-28), "message": "warmup"}}
	}
	first := coreProbeCache.operatorCert("mainnet", "", "", config.File{}, invoke, true)
	if first.Cached {
		t.Fatal("first run should not be cached")
	}
	second := coreProbeCache.operatorCert("mainnet", "", "", config.File{}, invoke, false)
	if !second.Cached {
		t.Fatal("second run should be cached")
	}
	if second.CheckedAt != first.CheckedAt {
		t.Fatal("cached checked_at mismatch")
	}
	ResetCoreProbeCache()
	coreProbeCache.ttl = 0
}

func TestCoreOperatorCertMatrixWithCache(t *testing.T) {
	rows := ApplyCoreOperatorCertProbes(DefaultCoreOperatorCertRows(), CoreProbesBundle{})
	cached := CoreOperatorCertResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		LiveOK:    true,
		OK:        true,
		Rows:      rows,
		Cached:    true,
	}
	out := CoreOperatorCertMatrix(&cached)
	if !out.MatrixOnly || !out.LiveOK {
		t.Fatalf("matrix+cache: %+v", out)
	}
}

func TestWarmCoreProbeCacheRunsWhenStale(t *testing.T) {
	ResetCoreProbeCache()
	calls := 0
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		calls++
		if method == "getblockchaininfo" {
			return map[string]interface{}{"result": map[string]interface{}{"blocks": int64(1), "headers": int64(1)}}
		}
		return map[string]interface{}{"result": map[string]interface{}{}}
	}
	WarmCoreProbeCache("mainnet", "", t.TempDir(), config.File{}, invoke)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && calls == 0 {
		time.Sleep(50 * time.Millisecond)
	}
	if calls == 0 {
		t.Fatal("expected probe invoke when cache empty")
	}
}

func TestWarmCoreProbeCacheSkipsWhenFresh(t *testing.T) {
	ResetCoreProbeCache()
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		if method == "getblockchaininfo" {
			return map[string]interface{}{"result": map[string]interface{}{"blocks": int64(1)}}
		}
		return map[string]interface{}{"result": map[string]interface{}{}}
	}
	coreProbeCache.operatorCert("mainnet", "", t.TempDir(), config.File{}, invoke, true)
	calls := 0
	wrap := func(method string, p []json.RawMessage) map[string]interface{} {
		calls++
		return invoke(method, p)
	}
	WarmCoreProbeCache("mainnet", "", t.TempDir(), config.File{}, wrap)
	if calls != 0 {
		t.Fatalf("expected no invoke when cache fresh, calls=%d", calls)
	}
}
