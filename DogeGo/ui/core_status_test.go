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

func TestBuildCoreStatusNoCache(t *testing.T) {
	ResetCoreProbeCache()
	st := BuildCoreStatus("mainnet", config.File{CoreRPCAddr: "127.0.0.1:22555"})
	if st.CoreRPCAddr != "127.0.0.1:22555" || !st.CoreRPCConfigured || st.OperatorCert != nil {
		t.Fatalf("status: %+v", st)
	}
}

func TestBuildCoreStatusMempoolFromCache(t *testing.T) {
	ResetCoreProbeCache()
	coreProbeCache.ttl = 2 * time.Minute
	coreProbeCache.mu.Lock()
	coreProbeCache.cert = CoreOperatorCertResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		LiveOK:    true,
		Rows:      DefaultCoreOperatorCertRows(),
		Probes: CoreProbesBundle{
			MempoolParity: MempoolParityProbeResult{
				OK: true, Total: 32, Passed: 32,
				OfflineCorpus: &MempoolOfflineCorpusSummary{OK: true, Total: 58, Passed: 58},
			},
		},
	}
	coreProbeCache.at = time.Now()
	coreProbeCache.mu.Unlock()

	st := BuildCoreStatus("mainnet", config.File{})
	if st.MempoolOfflineCorpus == nil || st.MempoolOfflineCorpus.Total != 58 {
		t.Fatalf("offline corpus: %+v", st.MempoolOfflineCorpus)
	}
	if st.MempoolParityTotal != 32 || !st.MempoolParityOK {
		t.Fatalf("parity: ok=%v passed=%d total=%d", st.MempoolParityOK, st.MempoolParityPassed, st.MempoolParityTotal)
	}
	ResetCoreProbeCache()
	coreProbeCache.ttl = 0
}

func TestEnrichLiveCoreStatus(t *testing.T) {
	ResetCoreProbeCache()
	live := map[string]any{}
	EnrichLiveCoreStatus(live, "mainnet", config.File{CoreRPCAddr: "127.0.0.1:22555"})
	if live["core_rpc_configured"] != true {
		t.Fatalf("live: %+v", live)
	}
}

func TestAnnotateCoreOperatorCertSummaryProtocolLock(t *testing.T) {
	ResetCoreProbeCache()
	coreProbeCache.ttl = 2 * time.Minute
	coreProbeCache.mu.Lock()
	coreProbeCache.cert = CoreOperatorCertResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		LiveOK:    true,
		SoloOK:    true,
		Rows:      DefaultCoreOperatorCertRows(),
		Probes: CoreProbesBundle{
			Compare: CoreCompareResult{
				DeploymentChecked: true,
				ProtocolLockOK:    true,
			},
		},
	}
	coreProbeCache.at = time.Now()
	coreProbeCache.mu.Unlock()

	s := map[string]any{}
	AnnotateCoreOperatorCertSummary(s)
	if s["dogego_protocol_lock_ok"] != true || s["dogego_deployment_checked"] != true {
		t.Fatalf("summary: %+v", s)
	}
	ResetCoreProbeCache()
	coreProbeCache.ttl = 0
}

func TestAnnotateCoreOperatorCertSummaryOfflineCorpus(t *testing.T) {
	ResetCoreProbeCache()
	coreProbeCache.ttl = 2 * time.Minute
	coreProbeCache.mu.Lock()
	coreProbeCache.cert = CoreOperatorCertResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		LiveOK:    true,
		Rows:      DefaultCoreOperatorCertRows(),
		Probes: CoreProbesBundle{
			MempoolParity: MempoolParityProbeResult{
				OK: true, Total: 32, Passed: 32,
				OfflineCorpus: &MempoolOfflineCorpusSummary{OK: true, Total: 58, Passed: 58},
			},
		},
	}
	coreProbeCache.at = time.Now()
	coreProbeCache.mu.Unlock()

	s := map[string]any{}
	AnnotateCoreOperatorCertSummary(s)
	if s["dogego_mempool_offline_corpus_total"] != 58 || s["dogego_mempool_offline_corpus_ok"] != true {
		t.Fatalf("offline corpus summary: %+v", s)
	}
	if s["dogego_mempool_parity_total"] != 32 || s["dogego_mempool_parity_ok"] != true {
		t.Fatalf("parity summary: %+v", s)
	}
	ResetCoreProbeCache()
	coreProbeCache.ttl = 0
}

func TestAnnotateCoreOperatorCertSummary(t *testing.T) {
	ResetCoreProbeCache()
	coreProbeCache.ttl = 2 * time.Minute
	invoke := func(method string, params []json.RawMessage) map[string]interface{} {
		return map[string]interface{}{"error": map[string]interface{}{"code": float64(-28), "message": "warmup"}}
	}
	_ = coreProbeCache.operatorCert("mainnet", "", "", config.File{}, invoke, true)
	s := map[string]any{}
	AnnotateCoreOperatorCertSummary(s)
	if s["dogego_operator_cert_total"] == nil {
		t.Fatalf("summary: %+v", s)
	}
	if _, ok := s["dogego_operator_cert_solo_pass"]; !ok {
		t.Fatalf("missing solo_pass: %+v", s)
	}
	ResetCoreProbeCache()
	coreProbeCache.ttl = 0
}
