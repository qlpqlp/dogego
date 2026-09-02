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

// peersRPCTimeout bounds getpeerinfo under IBD so /api/peers returns before the
// Analytics UI aborts (~8s). Heavy peer rows are best-effort; P2P snapshot fills counts.
const peersRPCTimeout = 2500 * time.Millisecond

// BuildPeersDashboardResponse assembles live peer rows for GET /api/peers.
func BuildPeersDashboardResponse(cfg StartConfig) map[string]any {
	out := map[string]any{
		"ok":           false,
		"generated_at": time.Now().Unix(),
		"peers":        []any{},
		"added_nodes":  []any{},
	}
	if cfg.RPCInvoke != nil {
		rpcOut, timedOut := invokeRPCWithTimeout(cfg.RPCInvoke, "getpeerinfo", nil, peersRPCTimeout)
		if timedOut {
			out["error"] = "peer info timed out (sync busy); showing connection counts"
			out["peers_partial"] = true
		} else if errObj, ok := rpcOut["error"].(map[string]interface{}); ok && errObj != nil {
			if msg, _ := errObj["message"].(string); msg != "" {
				out["error"] = msg
			}
		} else if res := rpcOut["result"]; res != nil {
			out["ok"] = true
			out["peers"] = normalizePeerInfoResult(res)
		} else {
			// Dispatch returns result:null when PeerInfo is not wired yet.
			out["error"] = "peer info not ready yet"
		}
		if !timedOut {
			addedOut, addedTimedOut := invokeRPCWithTimeout(cfg.RPCInvoke, "getaddednodeinfo", nil, peersRPCTimeout)
			if addedTimedOut {
				out["added_nodes_error"] = "added nodes timed out (sync busy)"
			} else if errObj, ok := addedOut["error"].(map[string]interface{}); ok && errObj != nil {
				// keep peers ok; surface added_nodes_error separately
				if msg, _ := errObj["message"].(string); msg != "" {
					out["added_nodes_error"] = msg
				}
			} else if res := addedOut["result"]; res != nil {
				out["added_nodes"] = res
			}
		}
	} else {
		out["error"] = "RPC not available"
	}
	if cfg.P2PSnapshot != nil {
		if snap := cfg.P2PSnapshot(); snap != nil {
			healP2PSnapConnectionCounts(snap)
			out["p2p"] = snap
			// Prefer live session counts from getpeerinfo; fall back to P2P snapshot
			// so Analytics does not show "0 peers" while Overview already sees dials.
			if peerListLen(out["peers"]) == 0 {
				if v, ok := snapInt(snap["connections_outbound"]); ok {
					out["connections_outbound"] = v
				}
				if v, ok := snapInt(snap["connections_inbound"]); ok {
					out["connections_inbound"] = v
				}
				if v, ok := snapInt(snap["connections_total"]); ok {
					out["connections_total"] = v
				}
				// Assist/primary IBD links are not always in getpeerinfo (dedicated TCP sessions).
				if synth := synthesizePeersFromP2PSnap(snap); len(synth) > 0 {
					out["peers"] = synth
					out["peers_from_p2p"] = true
					delete(out, "error")
					out["note"] = "Showing IBD sync links (block-assist / primary). Full getpeerinfo rows still warming up."
					out["ok"] = true
				} else if cout, _ := out["connections_outbound"].(int); cout > 0 {
					delete(out, "error")
					out["note"] = "Peers are connected; detailed peer rows are still warming up."
					out["ok"] = true
				}
			} else {
				// Keep dock/header totals authoritative when the P2P snap has live counts.
				// getpeerinfo can list more rows (cooling, relay, short-lived dials) than the
				// active sync mesh — that caused Analytics "Out 51" vs dock "18/0".
				if v, ok := snapInt(snap["connections_outbound"]); ok {
					out["connections_outbound"] = v
				}
				if v, ok := snapInt(snap["connections_inbound"]); ok {
					out["connections_inbound"] = v
				}
				if v, ok := snapInt(snap["connections_total"]); ok {
					out["connections_total"] = v
				}
			}
		}
	}
	if cfg.DGRSnapshot != nil {
		if snap := cfg.DGRSnapshot(); snap != nil {
			out["dgr"] = snap
		}
	}
	if out["connections_outbound"] == nil && peerListLen(out["peers"]) > 0 {
		countInbound, countOutbound := peerDirectionCounts(out["peers"])
		out["connections_inbound"] = countInbound
		out["connections_outbound"] = countOutbound
		out["connections_total"] = countInbound + countOutbound
	} else if out["connections_outbound"] == nil {
		out["connections_inbound"] = 0
		out["connections_outbound"] = 0
		out["connections_total"] = 0
	} else if out["connections_total"] == nil {
		cin, _ := out["connections_inbound"].(int)
		cout, _ := out["connections_outbound"].(int)
		out["connections_total"] = cin + cout
	}
	return out
}

func invokeRPCWithTimeout(
	invoke func(method string, params []json.RawMessage) map[string]interface{},
	method string,
	params []json.RawMessage,
	limit time.Duration,
) (map[string]interface{}, bool) {
	if invoke == nil {
		return map[string]interface{}{"error": map[string]interface{}{"message": "RPC not available"}}, false
	}
	if limit <= 0 {
		return invoke(method, params), false
	}
	ch := make(chan map[string]interface{}, 1)
	go func() {
		ch <- invoke(method, params)
	}()
	select {
	case out := <-ch:
		return out, false
	case <-time.After(limit):
		return nil, true
	}
}

func normalizePeerInfoResult(res any) any {
	switch v := res.(type) {
	case []map[string]interface{}:
		out := make([]any, len(v))
		for i := range v {
			out[i] = v[i]
		}
		return out
	case []any:
		return v
	default:
		return []any{}
	}
}

func peerListLen(peers any) int {
	switch v := peers.(type) {
	case []any:
		return len(v)
	case []map[string]interface{}:
		return len(v)
	default:
		return 0
	}
}

func snapInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
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

// healP2PSnapConnectionCounts rebuilds outbound totals from assist/primary when a
// live snapshot briefly has connections_*=0. Never runs on cold disk bootstrap.
func healP2PSnapConnectionCounts(snap map[string]any) {
	if snap == nil || snap["from_disk_snapshot"] == true {
		return
	}
	outN, haveOut := snapInt(snap["connections_outbound"])
	if haveOut && outN > 0 {
		return
	}
	assist, _ := snapInt(snap["block_assist_connections"])
	hdr, _ := snapInt(snap["dedicated_header_connections"])
	relay, _ := snapInt(snap["connections_outbound_relay"])
	rebuilt := assist + hdr + relay
	if primary, _ := snap["primary_peer"].(string); primary != "" && !strings.HasPrefix(strings.TrimSpace(primary), "(") {
		rebuilt++
	}
	if rebuilt <= 0 {
		return
	}
	inN, _ := snapInt(snap["connections_inbound"])
	snap["connections_outbound"] = rebuilt
	snap["connections_total"] = rebuilt + inN
}

// synthesizePeersFromP2PSnap builds Analytics peer cards from IBD assist/primary links when
// getpeerinfo is empty (common during download-first IBD — assist sessions are outside PeerMgr).
func synthesizePeersFromP2PSnap(snap map[string]any) []any {
	if snap == nil || snap["from_disk_snapshot"] == true {
		return nil
	}
	seen := make(map[string]struct{})
	var peers []any
	id := 1
	add := func(addr, role string, extras map[string]interface{}) {
		addr = strings.TrimSpace(addr)
		if addr == "" || strings.HasPrefix(addr, "(") {
			return
		}
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		row := map[string]interface{}{
			"id":              id,
			"addr":            addr,
			"inbound":         false,
			"dogego_role":     role,
			"connection_type": role,
			"dogego_note":     "IBD sync link (assist/primary); getpeerinfo detail still warming",
		}
		for k, v := range extras {
			row[k] = v
		}
		peers = append(peers, row)
		id++
	}
	if primary, ok := snap["primary_peer"].(string); ok {
		add(primary, "primary", nil)
	}
	appendAssist := func(addr string, lane any, recv, sent any) {
		extras := map[string]interface{}{}
		if lane != nil {
			extras["dogego_assist_lane"] = lane
		}
		if recv != nil {
			extras["bytesrecv"] = recv
		}
		if sent != nil {
			extras["bytessent"] = sent
		}
		add(addr, "block-assist", extras)
	}
	switch rows := snap["block_assist_peers"].(type) {
	case []any:
		for _, item := range rows {
			m, _ := item.(map[string]any)
			if m == nil {
				if mi, ok := item.(map[string]interface{}); ok {
					addr, _ := mi["addr"].(string)
					appendAssist(addr, mi["lane"], mi["bytes_recv"], mi["bytes_sent"])
				}
				continue
			}
			addr, _ := m["addr"].(string)
			appendAssist(addr, m["lane"], m["bytes_recv"], m["bytes_sent"])
		}
	case []map[string]any:
		for _, m := range rows {
			addr, _ := m["addr"].(string)
			appendAssist(addr, m["lane"], m["bytes_recv"], m["bytes_sent"])
		}
	}
	return peers
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
