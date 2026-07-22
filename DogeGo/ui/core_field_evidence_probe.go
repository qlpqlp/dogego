// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"fmt"
	"strings"
	"time"

	"dogego/consensus"
)

// CoreFieldEvidenceCheck is one Milestone A field evidence row.
type CoreFieldEvidenceCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warning, issue, skipped
	Value  any    `json:"value,omitempty"`
	Note   string `json:"note,omitempty"`
}

// CoreFieldEvidenceProbeResult is returned by GET /api/core-field-evidence-probe.
type CoreFieldEvidenceProbeResult struct {
	OK         bool                           `json:"ok"`
	LiveOK     bool                           `json:"live_ok"`
	CheckedAt  string                         `json:"checked_at"`
	Status     consensus.MainnetFieldDiskStatus `json:"status"`
	Checks     []CoreFieldEvidenceCheck       `json:"checks"`
	Notes      []string                       `json:"notes,omitempty"`
	Hint       string                         `json:"hint,omitempty"`
}

// ProbeCoreFieldEvidence reports offline committed corpus availability and live datadir readiness.
func ProbeCoreFieldEvidence(network string) CoreFieldEvidenceProbeResult {
	out := CoreFieldEvidenceProbeResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		OK:        true,
		Hint:      "Milestone A: committed mainnet field block/header corpus ships offline (dogego cert field-evidence or field_evidence_cert.ps1). Live TestCoreMainnetFieldHeaderPoW needs synced dogedata/mainnet.",
		Notes:     []string{"offline_corpus_committed"},
	}
	net := strings.TrimSpace(network)
	if net != "" && net != "mainnet" {
		out.Notes = append(out.Notes, "milestone_a_mainnet_only")
		out.Hint = "Milestone A live field evidence is mainnet-only. Offline corpus is still available in the binary; sync dogedata/mainnet for live header PoW and disk connect probes."
	}
	st := consensus.ProbeMainnetFieldDiskStatus()
	out.Status = st
	out.Checks = append(out.Checks, CoreFieldEvidenceCheck{
		Name: "offline_corpus", Status: "ok",
		Note: "mainnet_field_blocks.json + core_header_vectors field_header rows",
	})
	if !st.HeadersPresent {
		out.Checks = append(out.Checks, CoreFieldEvidenceCheck{
			Name: "live_header_pow", Status: "skipped",
			Note: "no operator headers under " + st.ChainDir,
		})
		out.Checks = append(out.Checks, CoreFieldEvidenceCheck{
			Name: "live_disk_connect", Status: "skipped",
			Note: "set DOGEGO_FIELD_DATADIR or sync dogedata/mainnet",
		})
		out.Notes = append(out.Notes, "live_field_evidence_unavailable")
		return out
	}
	if st.Error != "" {
		out.Checks = append(out.Checks, CoreFieldEvidenceCheck{
			Name: "live_header_pow", Status: "issue", Note: st.Error,
		})
		return out
	}
	hdrNote := fmt.Sprintf("tip=%d layout=%s", st.TipHeight, st.HeaderLayout)
	if st.HasAuxJournal {
		hdrNote += " aux=ok"
	}
	out.Checks = append(out.Checks, CoreFieldEvidenceCheck{
		Name: "live_header_pow", Status: "ok",
		Value: st.TipHeight, Note: hdrNote,
	})
	out.LiveOK = st.LiveHeaderPoWReady
	if st.LiveDiskConnectReady {
		out.Checks = append(out.Checks, CoreFieldEvidenceCheck{
			Name: "live_disk_connect", Status: "ok",
			Value: st.ContiguousRaw,
			Note:  fmt.Sprintf("contiguous_raw=%d (field_disk_connect_cert.ps1)", st.ContiguousRaw),
		})
		out.Notes = append(out.Notes, "live_disk_connect_ready")
	} else if st.HasRawBlocks {
		out.Checks = append(out.Checks, CoreFieldEvidenceCheck{
			Name: "live_disk_connect", Status: "warning",
			Note: "rawblocks present but bundled connect probe not ready (tx index or contiguous gap)",
		})
	} else {
		out.Checks = append(out.Checks, CoreFieldEvidenceCheck{
			Name: "live_disk_connect", Status: "skipped",
			Note: "no rawblocks yet - header PoW live only",
		})
	}
	if st.TipHeight >= 371337 && !st.HasAuxJournal {
		out.Checks = append(out.Checks, CoreFieldEvidenceCheck{
			Name: "auxpow_journal", Status: "warning",
			Note: "tip past aux activation but headers_aux.bin missing",
		})
	}
	return out
}
