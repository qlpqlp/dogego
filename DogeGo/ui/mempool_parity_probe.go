// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/config"
	"dogego/consensus"
)

// MempoolParityProbeRow is one live testmempoolaccept result vs the corpus expectation.
type MempoolParityProbeRow struct {
	Name              string `json:"name"`
	Template          string `json:"template"`
	WantAccept        bool   `json:"want_accept"`
	WantRejectReason  string `json:"want_reject_reason,omitempty"`
	GotAccept         bool   `json:"got_accept"`
	GotRejectReason   string `json:"got_reject_reason,omitempty"`
	Match             bool   `json:"match"`
	Error             string `json:"error,omitempty"`
	CoreGotAccept     *bool  `json:"core_got_accept,omitempty"`
	CoreGotRejectReason string `json:"core_got_reject_reason,omitempty"`
	CoreMatch         *bool  `json:"core_match,omitempty"`
	CoreError         string `json:"core_error,omitempty"`
}

// MempoolOfflineStatefulSummary reports offline stateful corpus eval (Milestone D; no live wallet needed).
type MempoolOfflineStatefulSummary struct {
	OK     bool `json:"ok"`
	Total  int  `json:"total"`
	Passed int  `json:"passed"`
}

// MempoolOfflineCorpusSummary reports offline eval of all core_mempool_vectors.json templates.
type MempoolOfflineCorpusSummary struct {
	OK     bool `json:"ok"`
	Total  int  `json:"total"`
	Passed int  `json:"passed"`
}

// MempoolStatefulLiveSummary reports Milestone D stateful offline + live reboottestnet gate status (read-only).
type MempoolStatefulLiveSummary struct {
	RebootTestnet        bool     `json:"reboot_testnet"`
	OfflineOK            bool     `json:"offline_ok"`
	OfflineTotal         int      `json:"offline_total"`
	OfflinePassed        int      `json:"offline_passed"`
	LiveScenarios        int      `json:"live_scenarios"`
	CoreCompareEnabled   bool     `json:"core_compare_enabled"`
	SetupParityOK        bool     `json:"setup_parity_ok,omitempty"`
	SetupParitySkipped   bool     `json:"setup_parity_skipped,omitempty"`
	SetupParityProbe     string   `json:"setup_parity_probe,omitempty"`
	SetupParityCLI       string   `json:"setup_parity_cli,omitempty"`
	SetupParityDogeGoBal *float64 `json:"setup_parity_dogego_balance,omitempty"`
	SetupParityCoreBal   *float64 `json:"setup_parity_core_balance,omitempty"`
	OfflineCorpusOK      bool     `json:"offline_corpus_ok,omitempty"`
	OfflineCorpusTotal   int      `json:"offline_corpus_total,omitempty"`
	OfflineCorpusPassed  int      `json:"offline_corpus_passed,omitempty"`
	CLILive              string   `json:"cli_live,omitempty"`
	CLICoreGate          string   `json:"cli_core_gate,omitempty"`
	ScriptLive           string   `json:"script_live,omitempty"`
	ScriptCoreGate       string   `json:"script_core_gate,omitempty"`
	Hint                 string   `json:"hint,omitempty"`
}

// MempoolStatefulStatusResult is returned by GET /api/mempool/stateful-status.
type MempoolStatefulStatusResult struct {
	OfflineCorpus   *MempoolOfflineCorpusSummary   `json:"offline_corpus,omitempty"`
	OfflineStateful *MempoolOfflineStatefulSummary `json:"offline_stateful,omitempty"`
	StatefulLive    *MempoolStatefulLiveSummary    `json:"stateful_live,omitempty"`
}

// MempoolParityProbeResult is returned by GET /api/mempool/parity-probe.
type MempoolParityProbeResult struct {
	OK              bool                           `json:"ok"`
	Total           int                            `json:"total"`
	Passed          int                            `json:"passed"`
	Failed          int                            `json:"failed"`
	Rows            []MempoolParityProbeRow        `json:"rows"`
	Hint            string                         `json:"hint,omitempty"`
	Skipped         bool                           `json:"skipped,omitempty"`
	Reason          string                         `json:"reason,omitempty"`
	CoreConfigured  bool                           `json:"core_configured,omitempty"`
	CoreAvailable   bool                           `json:"core_available,omitempty"`
	CoreRPCAddr     string                         `json:"core_rpc_addr,omitempty"`
	CoreAligned     bool                           `json:"core_aligned,omitempty"`
	OfflineCorpus   *MempoolOfflineCorpusSummary   `json:"offline_corpus,omitempty"`
	OfflineStateful *MempoolOfflineStatefulSummary `json:"offline_stateful,omitempty"`
	StatefulLive    *MempoolStatefulLiveSummary    `json:"stateful_live,omitempty"`
}

// RunMempoolParityProbe executes stateless mempool_parity_rpc.json rows via in-process testmempoolaccept.
func RunMempoolParityProbe(network string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) (out MempoolParityProbeResult) {
	defer func() {
		s := mempoolStatefulLiveSummary(network, conf, out.OfflineStateful, out.OfflineCorpus)
		out.StatefulLive = &s
	}()
	out = MempoolParityProbeResult{
		Hint:           "Stateless corpus rows safe for live nodes. Core side-by-side optional (Settings → Advanced). Full 58-vector gate: go test ./consensus -run TestCoreMempoolDifferential.",
		CoreConfigured: CoreCompareEnabled(network, conf),
	}
	ep := ResolveCoreParityEndpoints(network, conf)
	out.CoreRPCAddr = ep.Addr
	if corpus, stateful := evalMempoolOfflineSummaries(); corpus != nil {
		out.OfflineCorpus = corpus
		out.OfflineStateful = stateful
	}
	if invoke == nil {
		out.Skipped = true
		out.Reason = "RPC not ready"
		return out
	}
	rows, err := consensus.LoadMempoolParityRPCRows()
	if err != nil {
		out.Skipped = true
		out.Reason = err.Error()
		return out
	}
	coreOK := false
	if out.CoreConfigured {
		coreOK = probeCoreReachable(ep)
		out.CoreAvailable = coreOK
		if !coreOK {
			out.Hint = "Core RPC configured but unreachable. DogeGo rows still evaluated; set Core running on " + ep.Addr + " for side-by-side cert."
		}
	} else {
		out.Hint = "Optional Core compare: set core_rpc_addr in Settings → Advanced for Milestone D side-by-side cert."
	}
	out.Total = len(rows)
	coreAligned := true
	for _, row := range rows {
		pr := MempoolParityProbeRow{
			Name:             row.Name,
			Template:         row.Template,
			WantAccept:       row.WantAccept,
			WantRejectReason: row.WantRejectReason,
		}
		hexTx := strings.TrimSpace(row.Hex)
		if hexTx == "" {
			pr.Error = "empty hex"
			out.Failed++
			out.Rows = append(out.Rows, pr)
			continue
		}
		if _, err := hex.DecodeString(hexTx); err != nil {
			pr.Error = "bad hex: " + err.Error()
			out.Failed++
			out.Rows = append(out.Rows, pr)
			continue
		}
		gotAccept, gotReason, dgErr := invokeTestMempoolAccept(invoke, hexTx)
		if dgErr != "" {
			pr.Error = dgErr
			out.Failed++
			out.Rows = append(out.Rows, pr)
			continue
		}
		pr.GotAccept = gotAccept
		pr.GotRejectReason = gotReason
		pr.Match = gotAccept == pr.WantAccept
		if !pr.WantAccept && pr.WantRejectReason != "" && pr.GotRejectReason != "" {
			pr.Match = pr.Match && rejectReasonMatches(pr.GotRejectReason, pr.WantRejectReason)
		}
		if coreOK {
			coreAccept, coreReason, coreErr := invokeCoreTestMempoolAccept(ep, hexTx)
			if coreErr != "" {
				pr.CoreError = coreErr
				coreAligned = false
			} else {
				pr.CoreGotAccept = &coreAccept
				pr.CoreGotRejectReason = coreReason
				cm := coreAccept == pr.GotAccept
				if !coreAccept && !gotAccept {
					cm = cm || rejectReasonMatches(coreReason, gotReason) || rejectReasonMatches(gotReason, coreReason)
				}
				pr.CoreMatch = &cm
				if !cm {
					coreAligned = false
				}
			}
		}
		if pr.Match {
			out.Passed++
		} else {
			out.Failed++
		}
		out.Rows = append(out.Rows, pr)
	}
	out.OK = out.Failed == 0 && out.Total > 0
	out.CoreAligned = coreOK && coreAligned
	return out
}

// RunMempoolStatefulStatusProbe returns offline stateful corpus + live gate hints without live RPC rows.
func RunMempoolStatefulStatusProbe(network string, conf config.File) MempoolStatefulStatusResult {
	corpus, offline := evalMempoolOfflineSummaries()
	s := mempoolStatefulLiveSummary(network, conf, offline, corpus)
	return MempoolStatefulStatusResult{OfflineCorpus: corpus, OfflineStateful: offline, StatefulLive: &s}
}

func evalMempoolOfflineSummaries() (corpus *MempoolOfflineCorpusSummary, stateful *MempoolOfflineStatefulSummary) {
	evals, err := consensus.EvalMempoolCorpus()
	if err != nil || len(evals) == 0 {
		return nil, nil
	}
	passed, failed, ok := consensus.SummarizeMempoolCorpusEval(evals)
	corpus = &MempoolOfflineCorpusSummary{OK: ok, Total: len(evals), Passed: passed}
	_ = failed
	var statefulPassed int
	for _, r := range evals {
		if !r.Stateful {
			continue
		}
		if r.Match {
			statefulPassed++
		}
	}
	statefulTotal := 0
	for _, r := range evals {
		if r.Stateful {
			statefulTotal++
		}
	}
	if statefulTotal > 0 {
		stateful = &MempoolOfflineStatefulSummary{
			OK:     statefulPassed == statefulTotal,
			Total:  statefulTotal,
			Passed: statefulPassed,
		}
	}
	return corpus, stateful
}

func mempoolStatefulLiveSummary(network string, conf config.File, offline *MempoolOfflineStatefulSummary, corpus *MempoolOfflineCorpusSummary) MempoolStatefulLiveSummary {
	reboot := config.IsRebootTestnetNetwork(network)
	liveN := len(consensus.StatefulMempoolLiveScenarios())
	coreOn := CoreCompareEnabled(network, conf)
	out := MempoolStatefulLiveSummary{
		RebootTestnet:      reboot,
		LiveScenarios:      liveN,
		CoreCompareEnabled: coreOn,
		CLILive:            "scripts/mempool_stateful_parity_reboottestnet.ps1 -Scenario all",
		CLICoreGate:        "scripts/mempool_stateful_core_gate.ps1",
		ScriptLive:         "scripts/mempool_stateful_parity_reboottestnet.ps1",
		ScriptCoreGate:     "scripts/mempool_stateful_core_gate.ps1",
	}
	if corpus != nil {
		out.OfflineCorpusOK = corpus.OK
		out.OfflineCorpusTotal = corpus.Total
		out.OfflineCorpusPassed = corpus.Passed
	}
	if offline != nil {
		out.OfflineOK = offline.OK
		out.OfflineTotal = offline.Total
		out.OfflinePassed = offline.Passed
	}
	if reboot {
		sp := ProbeSetupParity(network)
		out.SetupParityProbe = "/api/core-setup-parity"
		out.SetupParityCLI = sp.CLI
		out.SetupParitySkipped = sp.Skipped
		out.SetupParityOK = sp.OK
		if sp.Setup.DogeGoBalance > 0 {
			bal := sp.Setup.DogeGoBalance
			out.SetupParityDogeGoBal = &bal
		}
		if sp.Setup.CoreBalance != nil {
			out.SetupParityCoreBal = sp.Setup.CoreBalance
		}
	}
	switch {
	case !reboot:
		out.Hint = "Offline stateful corpus runs on any network. Live " + fmt.Sprintf("%d", liveN) + "/24 wallet-anchored scenarios require reboot testnet (BIP125 rule 2/5 templates are offline-only in the 58-vector corpus)."
	case !out.OfflineOK:
		out.Hint = "Fix offline stateful corpus first (go test ./consensus -run TestEvalMempoolCorpusStateful)."
	case reboot && !out.SetupParityOK && !out.SetupParitySkipped:
		out.Hint = "Offline OK. Run " + out.SetupParityCLI + " (or GET " + out.SetupParityProbe + ") before live 24/24 gate; then " + out.CLILive + " and " + out.CLICoreGate + "."
	case coreOn:
		out.Hint = "Offline OK. Run " + out.CLILive + " then " + out.CLICoreGate + " for Milestone D Core 24/24 cert."
	default:
		out.Hint = "Offline OK. Run " + out.CLILive + " on reboot testnet; set core_rpc_addr for Core side-by-side 24/24 gate."
	}
	return out
}

// MempoolParityFullCorpusResult is returned by GET /api/mempool/parity-probe?corpus=full.
type MempoolParityFullCorpusResult struct {
	OK         bool                             `json:"ok"`
	Mode       string                           `json:"mode"`
	Total      int                              `json:"total"`
	Passed     int                              `json:"passed"`
	Failed     int                              `json:"failed"`
	Stateful   int                              `json:"stateful"`
	Stateless  int                              `json:"stateless"`
	Rows       []consensus.MempoolCorpusEvalResult `json:"rows"`
	Hint       string                           `json:"hint,omitempty"`
}

// RunMempoolParityFullCorpusProbe evaluates all core_mempool_vectors.json rows offline (Milestone D).
func RunMempoolParityFullCorpusProbe() MempoolParityFullCorpusResult {
	out := MempoolParityFullCorpusResult{Mode: "full_corpus"}
	statelessRows := 31
	if rows, err := consensus.LoadMempoolParityRPCRows(); err == nil {
		statelessRows = len(rows)
	}
	out.Hint = consensus.MempoolCorpusEvalHint(statelessRows)
	evals, err := consensus.EvalMempoolCorpus()
	if err != nil {
		out.Hint = err.Error()
		return out
	}
	out.Rows = evals
	out.Total = len(evals)
	for _, r := range evals {
		if r.Stateful {
			out.Stateful++
		} else {
			out.Stateless++
		}
	}
	out.Passed, out.Failed, out.OK = consensus.SummarizeMempoolCorpusEval(evals)
	return out
}

// RunMempoolParityStatefulCorpusProbe evaluates stateful-only rows from core_mempool_vectors.json offline.
func RunMempoolParityStatefulCorpusProbe() MempoolParityFullCorpusResult {
	out := MempoolParityFullCorpusResult{Mode: "stateful_corpus"}
	out.Hint = "Offline stateful mempool templates (package/RBF/dust prep stubs). Live dust probe on reboottestnet: scripts/mempool_stateful_parity_reboottestnet.ps1"
	evals, err := consensus.EvalMempoolCorpus()
	if err != nil {
		out.Hint = err.Error()
		return out
	}
	for _, r := range evals {
		if !r.Stateful {
			continue
		}
		out.Rows = append(out.Rows, r)
	}
	out.Total = len(out.Rows)
	out.Stateful = out.Total
	for _, r := range out.Rows {
		if r.Match {
			out.Passed++
		} else {
			out.Failed++
		}
	}
	out.OK = out.Failed == 0 && out.Total > 0
	return out
}

func probeCoreReachable(ep CoreParityEndpoints) bool {
	_, err := invokeExternalRPC(ep.Addr, ep.User, ep.Pass, "getblockchaininfo", nil)
	return err == nil
}

func invokeTestMempoolAccept(invoke func(string, []json.RawMessage) map[string]interface{}, hexTx string) (accept bool, reason string, errMsg string) {
	params, _ := json.Marshal([]any{[]string{hexTx}})
	resp := invoke("testmempoolaccept", []json.RawMessage{params})
	if errObj, ok := resp["error"].(map[string]interface{}); ok && errObj != nil {
		return false, "", fmt.Sprint(errObj["message"])
	}
	results, ok := resp["result"].([]interface{})
	if !ok || len(results) == 0 {
		return false, "", "empty testmempoolaccept result"
	}
	entry, ok := results[0].(map[string]interface{})
	if !ok {
		return false, "", "bad result shape"
	}
	accept, _ = entry["allowed"].(bool)
	if v, ok := entry["reject-reason"].(string); ok {
		reason = v
	}
	return accept, reason, ""
}

func invokeCoreTestMempoolAccept(ep CoreParityEndpoints, hexTx string) (accept bool, reason string, errMsg string) {
	v, err := invokeExternalRPCAny(ep.Addr, ep.User, ep.Pass, "testmempoolaccept", []any{[]string{hexTx}})
	if err != nil {
		return false, "", err.Error()
	}
	results, ok := v.([]interface{})
	if !ok || len(results) == 0 {
		return false, "", "empty core result"
	}
	entry, ok := results[0].(map[string]interface{})
	if !ok {
		return false, "", "bad core result shape"
	}
	accept, _ = entry["allowed"].(bool)
	if r, ok := entry["reject-reason"].(string); ok {
		reason = r
	}
	return accept, reason, ""
}

func rejectReasonMatches(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	if got == "" {
		return false
	}
	return strings.EqualFold(got, want) || strings.HasPrefix(strings.ToLower(got), strings.ToLower(want))
}
