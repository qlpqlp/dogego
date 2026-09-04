// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dogeos

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const HTTPAPIBase = "/api/ext/dogego.dogeos"

// HandleHTTP serves REST under /api/ext/dogego.dogeos/* via host httphandle.
func (s *Service) HandleHTTP(params []json.RawMessage) (interface{}, error) {
	raw := map[string]interface{}{}
	if len(params) > 0 {
		_ = json.Unmarshal(params[0], &raw)
		if len(raw) == 0 {
			var arr []json.RawMessage
			if json.Unmarshal(params[0], &arr) == nil && len(arr) > 0 {
				_ = json.Unmarshal(arr[0], &raw)
			}
		}
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

	jsonOK := func(v interface{}) map[string]interface{} {
		return map[string]interface{}{"status": 200, "json": v, "public": true}
	}
	jsonErr := func(status int, msg string) map[string]interface{} {
		return map[string]interface{}{"status": status, "json": map[string]string{"error": msg}}
	}

	switch {
	case method == "GET" && (path == "" || path == "v1"):
		return jsonOK(s.apiManifest()), nil
	case method == "GET" && path == "v1/status":
		return jsonOK(s.publicStatus()), nil
	case method == "GET" && path == "v1/networks":
		return jsonOK(BuiltInNetworks()), nil
	case method == "GET" && path == "v1/helpers":
		n, rpc, _ := s.store.EffectiveRPC()
		return jsonOK(Helpers(n, rpc)), nil
	case method == "GET" && path == "v1/metrics":
		return jsonOK(s.metrics.Summary()), nil
	case method == "GET" && path == "v1/probe":
		ctx, cancel := context.WithTimeout(context.Background(), defaultRPCTimeout)
		defer cancel()
		p, err := s.ProbeNow(ctx)
		if err != nil && !p.OK {
			return jsonOK(p), nil
		}
		return jsonOK(p), nil
	case method == "GET" && strings.HasPrefix(path, "v1/balance/"):
		addr := strings.TrimPrefix(path, "v1/balance/")
		c, n, err := s.client()
		if err != nil {
			return jsonErr(503, err.Error()), nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultRPCTimeout)
		defer cancel()
		wei, doge, err := c.GetBalance(ctx, addr)
		if err != nil {
			return jsonErr(400, err.Error()), nil
		}
		return jsonOK(map[string]interface{}{
			"address": normalizeAddress(addr), "wei": wei, "doge": doge,
			"explorer": ExplorerAddressURL(n, addr),
		}), nil
	case method == "GET" && strings.HasPrefix(path, "v1/tx/"):
		hash := strings.TrimPrefix(path, "v1/tx/")
		c, n, err := s.client()
		if err != nil {
			return jsonErr(503, err.Error()), nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultRPCTimeout)
		defer cancel()
		rcpt, err := c.GetTransactionReceipt(ctx, hash)
		if err != nil {
			return jsonErr(400, err.Error()), nil
		}
		return jsonOK(map[string]interface{}{
			"receipt": rcpt, "explorer": ExplorerTxURL(n, hash),
		}), nil
	case method == "GET" && path == "v1/block":
		num := query["number"]
		if num == "" {
			num = "latest"
		}
		c, _, err := s.client()
		if err != nil {
			return jsonErr(503, err.Error()), nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultRPCTimeout)
		defer cancel()
		block, err := c.GetBlockByNumber(ctx, num, false)
		if err != nil {
			return jsonErr(400, err.Error()), nil
		}
		return jsonOK(block), nil
	default:
		return jsonErr(404, "unknown path"), nil
	}
}

func (s *Service) apiManifest() map[string]interface{} {
	return map[string]interface{}{
		"extension": ExtensionID,
		"api_base":  HTTPAPIBase + "/v1",
		"docs":      "https://docs.dogeos.com/en/developers",
		"endpoints": []string{
			"GET /v1/status",
			"GET /v1/networks",
			"GET /v1/helpers",
			"GET /v1/metrics",
			"GET /v1/probe",
			"GET /v1/balance/{address}",
			"GET /v1/tx/{hash}",
			"GET /v1/block?number=latest",
		},
		"note": "DogeOS EVM bridge for DogeGo. Not Dogecoin L1. Not Doginals L2 overlay.",
	}
}

func (s *Service) publicStatus() map[string]interface{} {
	cfg := s.store.Get()
	n, rpc, err := s.store.EffectiveRPC()
	last, _ := s.metrics.Snapshot()
	out := map[string]interface{}{
		"extension":     ExtensionID,
		"network_id":    cfg.NetworkID,
		"network":       n,
		"rpc_url":       rpc,
		"last_probe":    last,
		"api_base":      HTTPAPIBase + "/v1",
		"recorded_unix": time.Now().Unix(),
	}
	if err != nil {
		out["rpc_error"] = err.Error()
	}
	return out
}
