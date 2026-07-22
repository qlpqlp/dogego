// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "dogego/config"

// CoreStatusResult is returned by GET /api/core-status (cached probe snapshot, no fresh run).
type CoreStatusResult struct {
	CoreRPCAddr          string                       `json:"core_rpc_addr"`
	CoreRPCConfigured    bool                         `json:"core_rpc_configured"`
	OperatorCert         *OperatorCertStatus          `json:"operator_cert,omitempty"`
	MempoolOfflineCorpus *MempoolOfflineCorpusSummary `json:"mempool_offline_corpus,omitempty"`
	MempoolParityOK      bool                         `json:"mempool_parity_ok,omitempty"`
	MempoolParityPassed  int                          `json:"mempool_parity_passed,omitempty"`
	MempoolParityTotal   int                          `json:"mempool_parity_total,omitempty"`
	ProbeCacheFresh      bool                         `json:"probe_cache_fresh"`
	ProbeCacheAgeSec     int                          `json:"probe_cache_age_sec,omitempty"`
	Hint                 string                       `json:"hint,omitempty"`
}

// OperatorCertStatus summarizes live web gates from the cached operator cert run.
type OperatorCertStatus struct {
	LiveOK            bool   `json:"live_ok"`
	SoloOK            bool   `json:"solo_ok,omitempty"`
	Pass              int    `json:"pass"`
	SoloPass          int    `json:"solo_pass,omitempty"`
	Total             int    `json:"total"`
	CheckedAt         string `json:"checked_at,omitempty"`
	Cached            bool   `json:"cached,omitempty"`
	ProtocolLockOK    bool   `json:"protocol_lock_ok,omitempty"`
	DeploymentChecked bool   `json:"deployment_checked,omitempty"`
	CoreCompareAvail  bool   `json:"core_compare_available,omitempty"`
}

// OperatorCertStatusFromResult derives a compact status from a cert result.
func OperatorCertStatusFromResult(cert CoreOperatorCertResult) OperatorCertStatus {
	web := 0
	pass := 0
	for _, r := range cert.Rows {
		if !r.WebProbe {
			continue
		}
		web++
		if r.OK != nil && *r.OK {
			pass++
		}
	}
	soloPass, _, soloOK := operatorCertSoloMetrics(cert.Rows)
	if cert.SoloPass > 0 {
		soloPass = cert.SoloPass
		soloOK = cert.SoloOK
	}
	st := OperatorCertStatus{
		LiveOK:    cert.LiveOK,
		SoloOK:    soloOK,
		Pass:      pass,
		SoloPass:  soloPass,
		Total:     web,
		CheckedAt: cert.CheckedAt,
		Cached:    cert.Cached,
	}
	if cert.Probes.Compare.DeploymentChecked {
		st.DeploymentChecked = true
		st.ProtocolLockOK = cert.Probes.Compare.ProtocolLockOK
		st.CoreCompareAvail = cert.Probes.Compare.Available
	}
	return st
}

// BuildCoreStatus returns Core parity config + cached operator cert (no probe execution).
func BuildCoreStatus(network string, conf config.File) CoreStatusResult {
	ep := ResolveCoreParityEndpoints(network, conf)
	out := CoreStatusResult{
		CoreRPCAddr:       ep.Addr,
		CoreRPCConfigured: CoreRPCExplicitlyConfigured(network, conf),
		Hint:              "Cached operator cert when available. Run GET /api/core-operator-cert?refresh=1 or Features → Run all probes for a fresh check.",
	}
	if cert, ok := coreProbeCache.peek(); ok {
		st := OperatorCertStatusFromResult(cert)
		out.OperatorCert = &st
		out.ProbeCacheFresh = true
		out.ProbeCacheAgeSec = cert.CacheAgeSec
		if oc := cert.Probes.MempoolParity.OfflineCorpus; oc != nil && oc.Total > 0 {
			out.MempoolOfflineCorpus = oc
		}
		if mp := cert.Probes.MempoolParity; !mp.Skipped && mp.Total > 0 {
			out.MempoolParityOK = mp.OK
			out.MempoolParityPassed = mp.Passed
			out.MempoolParityTotal = mp.Total
		}
	}
	return out
}

// AnnotateCoreOperatorCertSummary adds cached operator cert fields to /api/summary when a probe cache exists.
func AnnotateCoreOperatorCertSummary(summary map[string]any) {
	if summary == nil {
		return
	}
	cert, ok := coreProbeCache.peek()
	if !ok {
		return
	}
	st := OperatorCertStatusFromResult(cert)
	summary["dogego_operator_cert_live_ok"] = st.LiveOK
	summary["dogego_operator_cert_pass"] = st.Pass
	summary["dogego_operator_cert_total"] = st.Total
	summary["dogego_operator_cert_solo_ok"] = st.SoloOK
	summary["dogego_operator_cert_solo_pass"] = st.SoloPass
	if st.CheckedAt != "" {
		summary["dogego_operator_cert_checked_at"] = st.CheckedAt
	}
	if st.Cached {
		summary["dogego_operator_cert_cached"] = true
		if cert.CacheAgeSec > 0 {
			summary["dogego_operator_cert_cache_age_sec"] = cert.CacheAgeSec
		}
	}
	if cert.Probes.Compare.DeploymentChecked {
		summary["dogego_protocol_lock_ok"] = cert.Probes.Compare.ProtocolLockOK
		summary["dogego_deployment_checked"] = true
		summary["dogego_core_compare_available"] = cert.Probes.Compare.Available
	}
	if oc := cert.Probes.MempoolParity.OfflineCorpus; oc != nil && oc.Total > 0 {
		summary["dogego_mempool_offline_corpus_ok"] = oc.OK
		summary["dogego_mempool_offline_corpus_passed"] = oc.Passed
		summary["dogego_mempool_offline_corpus_total"] = oc.Total
	}
	if mp := cert.Probes.MempoolParity; !mp.Skipped && mp.Total > 0 {
		summary["dogego_mempool_parity_ok"] = mp.OK
		summary["dogego_mempool_parity_passed"] = mp.Passed
		summary["dogego_mempool_parity_total"] = mp.Total
	}
}

// EnrichLiveCoreStatus adds Core RPC + cached operator cert fields to a capabilities live map.
func EnrichLiveCoreStatus(live map[string]any, network string, conf config.File) {
	if live == nil {
		return
	}
	st := BuildCoreStatus(network, conf)
	live["core_rpc_addr"] = st.CoreRPCAddr
	live["core_rpc_configured"] = st.CoreRPCConfigured
	if st.OperatorCert != nil {
		live["operator_cert_live_ok"] = st.OperatorCert.LiveOK
		live["operator_cert_pass"] = st.OperatorCert.Pass
		live["operator_cert_total"] = st.OperatorCert.Total
		live["operator_cert_solo_ok"] = st.OperatorCert.SoloOK
		live["operator_cert_solo_pass"] = st.OperatorCert.SoloPass
		if st.OperatorCert.Cached {
			live["operator_cert_cached"] = true
		}
		if st.OperatorCert.DeploymentChecked {
			live["protocol_lock_ok"] = st.OperatorCert.ProtocolLockOK
			live["deployment_checked"] = true
			live["core_compare_available"] = st.OperatorCert.CoreCompareAvail
		}
		if cert, ok := coreProbeCache.peek(); ok {
			if oc := cert.Probes.MempoolParity.OfflineCorpus; oc != nil && oc.Total > 0 {
				live["mempool_offline_corpus_ok"] = oc.OK
				live["mempool_offline_corpus_passed"] = oc.Passed
				live["mempool_offline_corpus_total"] = oc.Total
			}
			if mp := cert.Probes.MempoolParity; !mp.Skipped && mp.Total > 0 {
				live["mempool_parity_ok"] = mp.OK
				live["mempool_parity_passed"] = mp.Passed
				live["mempool_parity_total"] = mp.Total
			}
		}
		if st.ProbeCacheAgeSec > 0 {
			live["operator_cert_cache_age_sec"] = st.ProbeCacheAgeSec
		}
	}
}
