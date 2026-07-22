// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// BuildPeersDashboardResponse assembles live peer rows for GET /api/peers.
func BuildPeersDashboardResponse(cfg StartConfig) map[string]any {
	out := map[string]any{
		"ok":           false,
		"generated_at": time.Now().Unix(),
		"peers":        []any{},
		"added_nodes":  []any{},
	}
	if cfg.RPCInvoke != nil {
		rpcOut := cfg.RPCInvoke("getpeerinfo", nil)
		if errObj, ok := rpcOut["error"].(map[string]interface{}); ok && errObj != nil {
			if msg, _ := errObj["message"].(string); msg != "" {
				out["error"] = msg
			}
		} else if res := rpcOut["result"]; res != nil {
			out["ok"] = true
			out["peers"] = res
		}
		addedOut := cfg.RPCInvoke("getaddednodeinfo", nil)
		if errObj, ok := addedOut["error"].(map[string]interface{}); ok && errObj != nil {
			// keep peers ok; surface added_nodes_error separately
			if msg, _ := errObj["message"].(string); msg != "" {
				out["added_nodes_error"] = msg
			}
		} else if res := addedOut["result"]; res != nil {
			out["added_nodes"] = res
		}
	} else {
		out["error"] = "RPC not available"
	}
	if cfg.P2PSnapshot != nil {
		if snap := cfg.P2PSnapshot(); snap != nil {
			out["p2p"] = snap
		}
	}
	if cfg.DGRSnapshot != nil {
		if snap := cfg.DGRSnapshot(); snap != nil {
			out["dgr"] = snap
		}
	}
	countInbound, countOutbound := peerDirectionCounts(out["peers"])
	out["connections_inbound"] = countInbound
	out["connections_outbound"] = countOutbound
	out["connections_total"] = countInbound + countOutbound
	return out
}

func peerDirectionCounts(peers any) (inbound, outbound int) {
	list, ok := peers.([]any)
	if !ok {
		if typed, ok := peers.([]map[string]interface{}); ok {
			for _, row := range typed {
				if peerRowInbound(row) {
					inbound++
				} else {
					outbound++
				}
			}
			return inbound, outbound
		}
		return 0, 0
	}
	for _, item := range list {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if peerRowInbound(row) {
			inbound++
		} else {
			outbound++
		}
	}
	return inbound, outbound
}

func peerRowInbound(row map[string]interface{}) bool {
	if v, ok := row["inbound"].(bool); ok {
		return v
	}
	return false
}

type peersActionBody struct {
	Action string `json:"action"`
	Node   string `json:"node"`
}

func applyPeersAction(cfg StartConfig, action, node string) (map[string]any, int) {
	action = strings.ToLower(strings.TrimSpace(action))
	node = strings.TrimSpace(node)
	if node == "" {
		return map[string]any{"ok": false, "error": "node address required"}, http.StatusBadRequest
	}
	if cfg.RPCInvoke == nil {
		return map[string]any{"ok": false, "error": "RPC not available"}, http.StatusServiceUnavailable
	}
	var method string
	var params []json.RawMessage
	switch action {
	case "add", "remove", "onetry":
		method = "addnode"
		nb, _ := json.Marshal(node)
		cb, _ := json.Marshal(action)
		params = []json.RawMessage{nb, cb}
	case "disconnect":
		method = "disconnectnode"
		nb, _ := json.Marshal(node)
		params = []json.RawMessage{nb}
	default:
		return map[string]any{"ok": false, "error": "action must be add, remove, onetry, or disconnect"}, http.StatusBadRequest
	}
	rpcOut := cfg.RPCInvoke(method, params)
	if errObj, ok := rpcOut["error"].(map[string]interface{}); ok && errObj != nil {
		msg, _ := errObj["message"].(string)
		if msg == "" {
			msg = "RPC error"
		}
		return map[string]any{"ok": false, "error": msg, "rpc": rpcOut}, http.StatusBadRequest
	}
	dash := BuildPeersDashboardResponse(cfg)
	dash["ok"] = true
	dash["action"] = action
	dash["node"] = node
	return dash, http.StatusOK
}

func registerPeersAPI(mux *http.ServeMux, cfg StartConfig, readAuth func(http.ResponseWriter, *http.Request) bool) {
	mux.HandleFunc("/api/peers", func(w http.ResponseWriter, r *http.Request) {
		if !readAuth(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(BuildPeersDashboardResponse(cfg))
		case http.MethodPost:
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			var req peersActionBody
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			out, code := applyPeersAction(cfg, req.Action, req.Node)
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(out)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
