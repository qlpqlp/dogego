// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"net/http"
	"strings"

	"dogego/config"
	"dogego/founder"
)

type setupFounderPreflightRequest struct {
	Network         string `json:"network"`
	NodeMode        string `json:"node_mode"`
	DataDir         string `json:"datadir"`
	P2PConnectivity string `json:"p2p_connectivity"`
	Mine            bool   `json:"mine"`
	NoWallet        bool   `json:"nowallet"`
	MiningAddress   string `json:"miningaddress"`
}

func registerSetupFounderPreflight(mux *http.ServeMux) {
	mux.HandleFunc("/api/setup/founder-preflight", handleSetupFounderPreflight)
}

func handleSetupFounderPreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req setupFounderPreflightRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	resp := buildSetupFounderPreflight(req)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func buildSetupFounderPreflight(req setupFounderPreflightRequest) setupPreflightResponse {
	net := strings.TrimSpace(req.Network)
	if net == "" {
		net = "testnet"
	}
	if !config.IsRebootTestnetNetwork(net) {
		return setupPreflightResponse{
			OK: true,
			Checks: []setupPreflightCheck{{
				ID: "founder_skip", Status: "ok", Title: "Founder preflight",
				Message: "Not reboot testnet - skipped",
			}},
		}
	}
	f := config.File{
		Network:         net,
		NodeMode:        strings.TrimSpace(req.NodeMode),
		DataDir:         strings.TrimSpace(req.DataDir),
		P2PConnectivity: strings.TrimSpace(req.P2PConnectivity),
		Mine:            req.Mine,
		NoWallet:        req.NoWallet,
		MiningAddress:   strings.TrimSpace(req.MiningAddress),
	}
	if f.P2PConnectivity == "" {
		f.P2PConnectivity = "both"
	}
	vr := founder.Verify(f)
	checks := make([]setupPreflightCheck, 0, len(vr.Checks))
	for _, c := range vr.Checks {
		st := c.Status
		if st == "issue" {
			st = "err"
		}
		checks = append(checks, setupPreflightCheck{
			ID:      c.ID,
			Status:  st,
			Title:   founderCheckTitle(c.ID),
			Message: c.Message,
			Fix:     c.Fix,
		})
	}
	ok := vr.OK
	hint := "Reboot testnet founder checks passed."
	if !ok {
		hint = "Fix errors before sharing addnode with joiners, or adjust settings and re-run."
	} else if len(vr.Warnings) > 0 {
		hint = "Passed with warnings - review before advertising HOST:44556 to joiners."
	}
	_ = hint
	return setupPreflightResponse{OK: ok, Checks: checks}
}

func founderCheckTitle(id string) string {
	switch id {
	case "network":
		return "Network"
	case "node_mode":
		return "Node mode"
	case "mine":
		return "Solo mining"
	case "p2p_inbound":
		return "Inbound P2P"
	case "p2p_port":
		return "P2P port"
	case "legacy_core_layout":
		return "Legacy datadir"
	case "datadir_fresh", "datadir_existing":
		return "Chain data"
	default:
		return id
	}
}
