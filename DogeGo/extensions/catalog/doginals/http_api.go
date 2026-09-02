// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"encoding/json"
	"fmt"
	"strings"

	"dogego/extensions"
)

const HTTPAPIBase = "/api/ext/dogego.doginals"

// handleHTTP is the extension-owned REST wallet read API.
// Host proxies /api/ext/dogego.doginals/* → RPC httphandle; core has no doginals routes.
func (e *Extension) handleHTTP(host extensions.Host, st *Store, params []json.RawMessage) (interface{}, error) {
	raw, err := parseMapParam(params)
	if err != nil {
		return nil, err
	}
	method, _ := raw["method"].(string)
	path, _ := raw["path"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.Trim(strings.TrimSpace(path), "/")

	query := map[string]string{}
	if q, ok := raw["query"].(map[string]interface{}); ok {
		for k, v := range q {
			query[k] = fmt.Sprint(v)
		}
	}
	body, _ := raw["body"].(map[string]interface{})

	jsonOK := func(v interface{}) map[string]interface{} {
		return map[string]interface{}{"status": 200, "json": v, "public": true}
	}
	jsonErr := func(status int, msg string) map[string]interface{} {
		return map[string]interface{}{"status": status, "json": map[string]string{"error": msg}}
	}

	switch {
	case method == "GET" && (path == "" || path == "v1"):
		return jsonOK(e.apiManifest()), nil
	case method == "GET" && path == "v1/status":
		net := ""
		tip := int64(-1)
		if host != nil {
			net = host.Network()
			tip, _ = host.TipHeight()
		}
		idx := st.IndexHeight()
		lag := int64(0)
		if tip >= 0 && idx >= 0 {
			lag = tip - idx
		}
		return jsonOK(map[string]interface{}{
			"height": idx, "proof": "", "blockhash": "",
			"chain_tip": tip, "index_lag": lag,
			"protocol_id": ProtocolID, "network": net,
			"api_base": HTTPAPIBase + "/v1",
		}), nil
	case method == "GET" && path == "v1/tokens":
		toks, err := st.ListTokens(100)
		if err != nil {
			return jsonErr(500, err.Error()), nil
		}
		return jsonOK(toks), nil
	case method == "GET" && strings.HasPrefix(path, "v1/address/"):
		rest := strings.TrimPrefix(path, "v1/address/")
		parts := strings.Split(strings.Trim(rest, "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			return jsonErr(400, "address required"), nil
		}
		addr := parts[0]
		if len(parts) >= 2 && parts[1] == "history" {
			rows, err := st.GetAddressHistory(addr, query["tick"], 40)
			if err != nil {
				return jsonErr(500, err.Error()), nil
			}
			return jsonOK(rows), nil
		}
		rows, err := st.GetAddressBalances(addr, 100)
		if err != nil {
			return jsonErr(500, err.Error()), nil
		}
		return jsonOK(rows), nil
	case method == "GET" && strings.HasPrefix(path, "v1/txid/"):
		txid := strings.Trim(strings.TrimPrefix(path, "v1/txid/"), "/")
		if txid == "" {
			return jsonErr(400, "txid required"), nil
		}
		rows, err := st.ListByTxID(txid)
		if err != nil {
			return jsonErr(500, err.Error()), nil
		}
		return jsonOK(rows), nil
	case method == "POST" && path == "v1/mint/l2":
		if body == nil {
			return jsonErr(400, "json body required"), nil
		}
		b, _ := json.Marshal(body)
		return e.HandleRPC("mintl2", []json.RawMessage{b}, host)
	case method == "POST" && path == "v1/inscribe":
		if body == nil {
			return jsonErr(400, "json body required"), nil
		}
		b, _ := json.Marshal(body)
		return e.HandleRPC("inscribe", []json.RawMessage{b}, host)
	default:
		return jsonErr(404, "unknown route"), nil
	}
}

func (e *Extension) apiManifest() map[string]interface{} {
	return map[string]interface{}{
		"version": "0.5.0",
		"compat":  []string{"doginals-wallet-v1"},
		"api_base": HTTPAPIBase + "/v1",
		"note":    "Owned by dogego.doginals via httphandle; host only proxies /api/ext/{id}/…",
		"routes": []string{
			"GET " + HTTPAPIBase + "/v1/status",
			"GET " + HTTPAPIBase + "/v1/tokens",
			"GET " + HTTPAPIBase + "/v1/address/{address}",
			"GET " + HTTPAPIBase + "/v1/address/{address}/history?tick=",
			"GET " + HTTPAPIBase + "/v1/txid/{txid}",
			"POST " + HTTPAPIBase + "/v1/mint/l2 (dashboard unlock)",
			"POST " + HTTPAPIBase + "/v1/inscribe (dashboard unlock)",
		},
	}
}
