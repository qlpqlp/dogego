// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"
	"time"
)

func TestEnrichParitySummaryFromProbeCache(t *testing.T) {
	ResetCoreProbeCache()
	coreProbeCache.ttl = 2 * time.Minute
	coreProbeCache.mu.Lock()
	coreProbeCache.cert = CoreOperatorCertResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Probes: CoreProbesBundle{
			Compare: CoreCompareResult{
				DeploymentChecked: true,
				ProtocolLockOK:    true,
			},
			MempoolParity: MempoolParityProbeResult{
				OfflineCorpus: &MempoolOfflineCorpusSummary{OK: true, Total: 58, Passed: 58},
			},
		},
	}
	coreProbeCache.at = time.Now()
	coreProbeCache.mu.Unlock()

	s := ParitySummary{}
	EnrichParitySummaryFromProbeCache(&s)
	if !s.ProtocolLockChecked || !s.ProtocolLockOK {
		t.Fatalf("protocol lock summary: %+v", s)
	}
	if !s.OfflineCorpusChecked || !s.OfflineCorpusOK || s.OfflineCorpusTotal != 58 {
		t.Fatalf("offline corpus summary: %+v", s)
	}
	ResetCoreProbeCache()
	coreProbeCache.ttl = 0
}
