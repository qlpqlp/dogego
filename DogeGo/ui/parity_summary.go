// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

// ParitySummary aggregates capability / gap / RPC counts for the Features tab.
type ParitySummary struct {
	StandaloneReady bool   `json:"standalone_ready"`
	ExitGateStatus  string `json:"exit_gate_status"` // partial, ready
	FeaturesLive    int    `json:"features_live"`
	FeaturesPartial int    `json:"features_partial"`
	FeaturesPlanned int    `json:"features_planned"`
	FeaturesStub    int    `json:"features_stub"`
	FeaturesNA      int    `json:"features_na"`
	GapsOpen        int    `json:"gaps_open"`
	GapsPartial     int    `json:"gaps_partial"`
	GapsDeclined    int    `json:"gaps_declined"`
	RoadmapDone     int    `json:"roadmap_done"`
	RoadmapTotal    int    `json:"roadmap_total"`
	RPCLive         int    `json:"rpc_live"`
	RPCPartial      int    `json:"rpc_partial"`
	RPCStub         int    `json:"rpc_stub"`
	ProtocolLockChecked bool `json:"protocol_lock_checked,omitempty"`
	ProtocolLockOK      bool `json:"protocol_lock_ok,omitempty"`
	OfflineCorpusChecked bool `json:"offline_corpus_checked,omitempty"`
	OfflineCorpusOK      bool `json:"offline_corpus_ok,omitempty"`
	OfflineCorpusPassed  int    `json:"offline_corpus_passed,omitempty"`
	OfflineCorpusTotal   int    `json:"offline_corpus_total,omitempty"`
}

// CoreGuidance tells operators when to prefer Core vs DogeGo.
type CoreGuidance struct {
	UseCoreWhen   []string   `json:"use_core_when"`
	UseDogeGoWhen []string   `json:"use_dogego_when"`
	Intentional   []string   `json:"intentional_diffs"`
	DocLinks      []DocsLink `json:"doc_links"`
}

func buildParitySummary(categories []CapabilityCategory, gaps []CoreParityGap, roadmap []RoadmapHighlight, rpc []RPCMethodRow) ParitySummary {
	s := ParitySummary{RoadmapTotal: len(roadmap)}
	for _, cat := range categories {
		for _, f := range cat.Features {
			switch f.Status {
			case "live":
				s.FeaturesLive++
			case "partial":
				s.FeaturesPartial++
			case "stub":
				s.FeaturesStub++
			case "na":
				s.FeaturesNA++
			default:
				s.FeaturesPlanned++
			}
		}
	}
	for _, g := range gaps {
		switch g.Status {
		case "open":
			s.GapsOpen++
		case "partial":
			s.GapsPartial++
		case "declined":
			s.GapsDeclined++
		}
	}
	for _, r := range roadmap {
		if r.Done {
			s.RoadmapDone++
		}
	}
	for _, row := range rpc {
		switch row.Class {
		case "live":
			s.RPCLive++
		case "stub":
			s.RPCStub++
		default:
			s.RPCPartial++
		}
	}
	s.StandaloneReady = s.GapsOpen == 0 && s.GapsPartial == 0
	if s.StandaloneReady {
		s.ExitGateStatus = "ready"
	} else {
		s.ExitGateStatus = "partial"
	}
	return s
}

// EnrichParitySummaryFromProbeCache adds live probe status when operator probes are cached.
func EnrichParitySummaryFromProbeCache(s *ParitySummary) {
	if s == nil {
		return
	}
	cert, ok := coreProbeCache.peek()
	if !ok {
		return
	}
	if cert.Probes.Compare.DeploymentChecked {
		s.ProtocolLockChecked = true
		s.ProtocolLockOK = cert.Probes.Compare.ProtocolLockOK
	}
	if oc := cert.Probes.MempoolParity.OfflineCorpus; oc != nil && oc.Total > 0 {
		s.OfflineCorpusChecked = true
		s.OfflineCorpusOK = oc.OK
		s.OfflineCorpusPassed = oc.Passed
		s.OfflineCorpusTotal = oc.Total
	}
}

// DefaultCoreGuidance is static operator copy for GET /api/capabilities.
func DefaultCoreGuidance() CoreGuidance {
	return CoreGuidance{
		UseCoreWhen: nil,
		UseDogeGoWhen: []string{
			"Beta: run a full Dogecoin node from the browser and help us test and tune.",
			"Fully autonomous operation without Dogecoin Core (solo protocol-lock sanity via GET /api/core-compare).",
			"CGNAT / Starlink outbound relay (classic | cgnat | both) and DGR QUIC relay.",
			"Built-in HD wallet on mainnet/testnet with Send/Receive (same keypool idea as Core, stored in wallet.json).",
			"Core wallet.dat import, mining GBT/aux, PQ OP_RETURN + carrier, and the Features cert probes.",
		},
		Intentional: []string{
			"Mainnet consensus rules match Dogecoin Core; no protocol forks (ROADMAP protocol lock).",
			"Native chain storage (headers/, rawblocks/, indexes/); not Core LevelDB layout.",
			"HD keypool lives in wallet.json (Core-like receive/change pool behavior; not wallet.dat BDB).",
			"BIP152 v1 compact blocks (code + offline soak; optional live HB soak is self-cert).",
			"No Litecoin parent chain store for AuxPoW reorg checks.",
			"Witness txs decoded for RPC but rejected at admission (Dogecoin policy).",
		},
		DocLinks: []DocsLink{
			{Label: "ROADMAP.md (protocol lock)", Path: "ROADMAP.md"},
			{Label: "CORE_PARITY_GAPS.md", Path: "docs/CORE_PARITY_GAPS.md"},
			{Label: "INTENTIONAL_DIFFERENCES.md", Path: "docs/INTENTIONAL_DIFFERENCES.md"},
			{Label: "STANDALONE_FULLNODE_ACCEPTANCE.md", Path: "docs/STANDALONE_FULLNODE_ACCEPTANCE.md"},
			{Label: "CORE_OPERATOR_RUNBOOK.md", Path: "docs/CORE_OPERATOR_RUNBOOK.md"},
		},
	}
}
