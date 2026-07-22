// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dogego/autostart"
	"dogego/config"
	"dogego/store"
)

// CoreRestartResumeCheck is one restart-resume workflow row (Milestone E).
type CoreRestartResumeCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warning, issue, skipped
	Value  any    `json:"value,omitempty"`
	Note   string `json:"note,omitempty"`
}

// CoreRestartResumeResult is returned by GET /api/core-restart-resume.
type CoreRestartResumeResult struct {
	OK               bool                     `json:"ok"`
	Network          string                   `json:"network"`
	Headers          int64                    `json:"headers,omitempty"`
	ContiguousRaw    int64                    `json:"contiguous_raw,omitempty"`
	CheckpointProbe  int64                    `json:"checkpoint_probe,omitempty"`
	BodyLag          int64                    `json:"body_lag,omitempty"`
	AssistPeerPool   int64                    `json:"assist_peer_pool,omitempty"`
	AssistSessions   int64                    `json:"assist_sessions,omitempty"`
	ConnectLag       int64                    `json:"connect_lag,omitempty"`
	ConnectLagMax    int64                    `json:"connect_lag_max,omitempty"`
	ConnectCatchUpPasses     int   `json:"connect_catch_up_passes,omitempty"`
	ConnectCatchUpBatch      int   `json:"connect_catch_up_batch,omitempty"`
	ConnectCatchUpIntervalMs int64 `json:"connect_catch_up_interval_ms,omitempty"`
	AutostartWant    bool                     `json:"autostart_want,omitempty"`
	AutostartOK      bool                     `json:"autostart_ok,omitempty"`
	IBD              bool                     `json:"ibd"`
	CheckedAt        string                   `json:"checked_at"`
	Checks           []CoreRestartResumeCheck `json:"checks"`
	Issues           []string                 `json:"issues,omitempty"`
	Warnings         []string                 `json:"warnings,omitempty"`
	Notes            []string                 `json:"notes,omitempty"`
	Hint             string                   `json:"hint,omitempty"`
}

// ProbeCoreRestartResume verifies checkpoint vs contiguous bodies and IBD assist pool health.
func ProbeCoreRestartResume(network, chainDataDir string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) CoreRestartResumeResult {
	out := CoreRestartResumeResult{
		Network:   strings.TrimSpace(network),
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Hint:      "Milestone E restart resume - mirrors scripts/core_restart_resume_check.ps1. Reads rawblocks_sync.json and getblockchaininfo.",
	}
	var checkpointProbe int64 = -1
	if chainDataDir != "" {
		cp, err := store.LoadRawBlockSyncCheckpoint(chainDataDir)
		if err != nil {
			out.Warnings = append(out.Warnings, "rawblocks_sync_read_error")
			out.Checks = append(out.Checks, CoreRestartResumeCheck{
				Name: "rawblocks_sync.json", Status: "warning", Note: err.Error(),
			})
		} else if cp.NextProbeHeight >= 0 {
			checkpointProbe = cp.NextProbeHeight
			out.CheckpointProbe = checkpointProbe
			out.Checks = append(out.Checks, CoreRestartResumeCheck{
				Name: "checkpoint_probe", Status: "ok", Value: checkpointProbe,
				Note: fmt.Sprintf("contiguous_on_disk=%d", cp.ContiguousRawHeight),
			})
		}
	} else {
		out.Notes = append(out.Notes, "chain_data_dir_unavailable")
	}

	if invoke == nil {
		out.Issues = append(out.Issues, "dogego_rpc_not_ready")
		return out
	}

	info, err := invokeDogeGoRPC(invoke, "getblockchaininfo", nil)
	if err != nil {
		out.Issues = append(out.Issues, "rpc_unreachable")
		out.Checks = append(out.Checks, CoreRestartResumeCheck{Name: "getblockchaininfo", Status: "issue", Note: err.Error()})
		out.OK = false
		return out
	}
	out.Checks = append(out.Checks, CoreRestartResumeCheck{Name: "getblockchaininfo", Status: "ok"})

	if hdr, ok := intFromAny(info["headers"]); ok {
		out.Headers = hdr
	}
	if cont, ok := intFromAny(info["dogego_contiguous_raw_height"]); ok {
		out.ContiguousRaw = cont
	}
	if ibd, ok := info["initialblockdownload"].(bool); ok {
		out.IBD = ibd
	}
	appendConnectLagCheck(&out, info)

	var assistPool, assistSessions int64 = -1, -1
	if rs, ok := info["dogego_raw_sync"].(map[string]interface{}); ok {
		if v, ok := intFromAny(rs["assist_peer_pool"]); ok {
			assistPool = v
			out.AssistPeerPool = v
		}
		if v, ok := intFromAny(rs["assist_active_sessions"]); ok {
			assistSessions = v
			out.AssistSessions = v
		}
	}

	if cont := out.ContiguousRaw; checkpointProbe >= 0 && cont >= 0 && checkpointProbe > cont+64 {
		out.Warnings = append(out.Warnings, "checkpoint_ahead_of_contiguous")
		out.Notes = append(out.Notes, "initProgressiveRawAtStartup_should_realign")
		out.Checks = append(out.Checks, CoreRestartResumeCheck{
			Name: "checkpoint_vs_contiguous", Status: "warning",
			Value: map[string]any{"checkpoint": checkpointProbe, "contiguous": cont},
			Note:  fmt.Sprintf("delta=%d (max 64)", checkpointProbe-cont),
		})
	} else if checkpointProbe >= 0 && out.ContiguousRaw >= 0 {
		out.Checks = append(out.Checks, CoreRestartResumeCheck{
			Name: "checkpoint_vs_contiguous", Status: "ok",
			Value: map[string]any{"checkpoint": checkpointProbe, "contiguous": out.ContiguousRaw},
		})
	}

	if out.ContiguousRaw < 0 && out.Headers > 1000 {
		out.Warnings = append(out.Warnings, "no_contiguous_bodies_yet")
		out.Checks = append(out.Checks, CoreRestartResumeCheck{
			Name: "contiguous_bodies", Status: "warning", Note: "headers high but contiguous unknown",
		})
	}

	if out.Headers > out.ContiguousRaw && out.ContiguousRaw >= 0 {
		out.BodyLag = out.Headers - out.ContiguousRaw
	}
	if out.IBD && out.BodyLag > 5000 {
		if assistPool <= 0 {
			out.Issues = append(out.Issues, "assist_peer_pool_empty_during_ibd")
			out.Checks = append(out.Checks, CoreRestartResumeCheck{
				Name: "assist_ibd_pool", Status: "issue", Value: assistPool,
			})
		} else if assistSessions <= 0 {
			out.Warnings = append(out.Warnings, "assist_pool_ready_no_active_sessions")
			out.Checks = append(out.Checks, CoreRestartResumeCheck{
				Name: "assist_ibd_sessions", Status: "warning",
				Value: map[string]any{"pool": assistPool, "sessions": assistSessions},
			})
		} else {
			out.Notes = append(out.Notes, "assist_ibd_healthy")
			out.Checks = append(out.Checks, CoreRestartResumeCheck{
				Name: "assist_ibd", Status: "ok",
				Value: map[string]any{"pool": assistPool, "sessions": assistSessions, "body_lag": out.BodyLag},
			})
		}
	}

	zmqResp := invoke("getzmqnotifications", nil)
	if errObj, ok := zmqResp["error"].(map[string]interface{}); ok && errObj != nil {
		msg := strings.ToLower(fmt.Sprint(errObj["message"]))
		code, _ := errObj["code"].(float64)
		if code == -32601 || strings.Contains(msg, "not implemented") || strings.Contains(msg, "unknown method") {
			out.Issues = append(out.Issues, "getzmqnotifications_missing")
			out.Checks = append(out.Checks, CoreRestartResumeCheck{Name: "getzmqnotifications", Status: "issue", Note: msg})
		} else {
			out.Warnings = append(out.Warnings, "getzmqnotifications_error")
			out.Checks = append(out.Checks, CoreRestartResumeCheck{Name: "getzmqnotifications", Status: "warning", Note: msg})
		}
	} else if zmqResp["result"] == nil {
		out.Warnings = append(out.Warnings, "getzmqnotifications_null")
		out.Checks = append(out.Checks, CoreRestartResumeCheck{Name: "getzmqnotifications", Status: "warning", Note: "null result"})
	} else {
		out.Checks = append(out.Checks, CoreRestartResumeCheck{Name: "getzmqnotifications", Status: "ok", Value: zmqResp["result"]})
	}

	appendAutostartCheck(&out, conf)

	out.ConnectLagMax = connectLagMax()
	out.OK = len(out.Issues) == 0
	return out
}

// appendAutostartCheck verifies OS login autostart when dogecoinconf.json requests autostart=login.
func appendAutostartCheck(out *CoreRestartResumeResult, conf config.File) {
	if out == nil {
		return
	}
	want := conf.AutostartOnLogin()
	out.AutostartWant = want
	vr := autostart.VerifyLogin(want)
	out.Warnings = append(out.Warnings, vr.Warnings...)
	out.Notes = append(out.Notes, vr.Notes...)
	out.Issues = append(out.Issues, vr.Issues...)
	st := vr.Status
	if !want {
		if st.Installed {
			out.Checks = append(out.Checks, CoreRestartResumeCheck{
				Name: "os_autostart", Status: "warning",
				Note: "config autostart=disable but OS registration is still present",
				Value: map[string]any{"method": st.Method, "installed": true},
			})
		} else {
			out.Checks = append(out.Checks, CoreRestartResumeCheck{
				Name: "os_autostart", Status: "skipped", Note: "autostart=disable",
			})
		}
		return
	}
	if !st.Supported {
		out.Checks = append(out.Checks, CoreRestartResumeCheck{
			Name: "os_autostart", Status: "warning", Note: st.Detail,
		})
		return
	}
	if !st.Installed {
		out.Checks = append(out.Checks, CoreRestartResumeCheck{
			Name: "os_autostart", Status: "issue",
			Note: "autostart=login in config but OS sign-in task is missing - re-save Settings or finish setup wizard",
			Value: map[string]any{"platform": st.Platform, "method": st.Method},
		})
		return
	}
	out.AutostartOK = true
	out.Checks = append(out.Checks, CoreRestartResumeCheck{
		Name: "os_autostart", Status: "ok",
		Value: map[string]any{"platform": st.Platform, "method": st.Method, "detail": st.Detail},
	})
}

// AnnotateRestartResumeSummary adds checkpoint probe and lightweight restart-resume warnings to /api/summary.
func AnnotateRestartResumeSummary(summary map[string]any, chainDataDir string, headerTip, contiguous int64, ibd bool, assistPeerPool int) {
	if summary == nil {
		return
	}
	if chainDataDir == "" {
		return
	}
	cp, err := store.LoadRawBlockSyncCheckpoint(chainDataDir)
	if err != nil {
		summary["dogego_checkpoint_read_error"] = err.Error()
		return
	}
	if cp.NextProbeHeight >= 0 {
		summary["dogego_checkpoint_probe"] = cp.NextProbeHeight
	}
	var warnings []string
	if cp.NextProbeHeight >= 0 && contiguous >= 0 && cp.NextProbeHeight > contiguous+64 {
		warnings = append(warnings, "checkpoint_ahead_of_contiguous")
	}
	var bodyLag int64
	if headerTip > contiguous && contiguous >= 0 {
		bodyLag = headerTip - contiguous
		summary["dogego_body_lag_headers"] = bodyLag
	}
	if ibd && bodyLag > 5000 && assistPeerPool <= 0 {
		warnings = append(warnings, "assist_peer_pool_empty_during_ibd")
	}
	if lag, ok := intFromAny(summary["dogego_connect_lag"]); ok && lag > connectLagMax() {
		warnings = append(warnings, "connect_lag_above_threshold")
		summary["dogego_connect_lag_warn"] = true
	}
	if len(warnings) > 0 {
		summary["dogego_restart_resume_warnings"] = warnings
	}
}
