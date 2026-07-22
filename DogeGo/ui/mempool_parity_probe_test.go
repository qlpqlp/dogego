// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"dogego/config"
	"testing"
)

func TestRunMempoolStatefulStatusProbe(t *testing.T) {
	r := RunMempoolStatefulStatusProbe("testnet", config.File{})
	if r.StatefulLive == nil {
		t.Fatal("expected stateful_live")
	}
	if !r.StatefulLive.RebootTestnet {
		t.Fatal("expected reboot testnet")
	}
	if r.StatefulLive.LiveScenarios < 24 {
		t.Fatalf("live scenarios=%d want >=24", r.StatefulLive.LiveScenarios)
	}
	if r.StatefulLive.SetupParityProbe != "/api/core-setup-parity" {
		t.Fatalf("setup probe=%q", r.StatefulLive.SetupParityProbe)
	}
	if r.StatefulLive.SetupParityCLI == "" {
		t.Fatal("expected setup parity CLI hint")
	}
	if r.OfflineCorpus == nil || !r.OfflineCorpus.OK {
		t.Fatalf("offline corpus: %+v", r.OfflineCorpus)
	}
	if r.OfflineCorpus.Total < 58 {
		t.Fatalf("offline corpus total=%d want >=58", r.OfflineCorpus.Total)
	}
	if r.OfflineStateful == nil || !r.OfflineStateful.OK {
		t.Fatalf("offline stateful: %+v", r.OfflineStateful)
	}
}

func TestRunMempoolStatefulStatusProbeMainnetOfflineOnly(t *testing.T) {
	r := RunMempoolStatefulStatusProbe("mainnet", config.File{})
	if r.StatefulLive == nil || r.StatefulLive.RebootTestnet {
		t.Fatalf("expected non-reboot: %+v", r.StatefulLive)
	}
}

func TestRunMempoolParityProbeOfflineCorpus(t *testing.T) {
	out := RunMempoolParityProbe("testnet", config.File{}, nil)
	if out.OfflineCorpus == nil {
		t.Fatal("expected offline_corpus")
	}
	if out.OfflineCorpus.Total < 58 {
		t.Fatalf("offline corpus total=%d want >=58", out.OfflineCorpus.Total)
	}
	if !out.OfflineCorpus.OK {
		t.Fatalf("offline corpus failed passed=%d total=%d", out.OfflineCorpus.Passed, out.OfflineCorpus.Total)
	}
	if out.OfflineStateful == nil || !out.OfflineStateful.OK {
		t.Fatalf("offline stateful: %+v", out.OfflineStateful)
	}
}

func TestRunMempoolParityStatefulCorpusProbe(t *testing.T) {
	r := RunMempoolParityStatefulCorpusProbe()
	if r.Total < 10 {
		t.Fatalf("stateful total=%d want >=10", r.Total)
	}
	if !r.OK {
		t.Fatalf("stateful corpus failed passed=%d failed=%d", r.Passed, r.Failed)
	}
	if r.Stateful != r.Total {
		t.Fatalf("stateful count=%d total=%d", r.Stateful, r.Total)
	}
}
