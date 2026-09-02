// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"dogego/extensions"
	"dogego/ui/websecurity"
)

// registerExtensionHTTPGateway mounts a generic proxy at /api/ext/{extension.id}/...
// so packages can own REST APIs without dogego-specific host routes.
//
// Contract: enabled extension implements RPC method "httphandle" with one object param:
//
//	{ "method": "GET", "path": "v1/status", "query": {"k":"v"}, "body": {…} | null }
//
// Response may be the JSON body directly, or:
//
//	{ "status": 200, "json": {…}, "public": true }
//
// GET/HEAD/OPTIONS are public (CORS *). POST/PUT/PATCH/DELETE require dashboard unlock.
func registerExtensionHTTPGateway(mux *http.ServeMux, invoke func(method string, params []json.RawMessage) map[string]interface{}, webGate *websecurity.Gate) {
	if mux == nil || invoke == nil {
		return
	}
	mux.HandleFunc("/api/ext/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/ext/")
		rest = strings.Trim(rest, "/")
		if rest == "" {
			http.Error(w, "extension id required", http.StatusBadRequest)
			return
		}
		id, path := rest, ""
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			id = rest[:i]
			path = strings.Trim(rest[i+1:], "/")
		}
		id = strings.TrimSpace(id)
		if id == "" || strings.Contains(id, "..") {
			http.Error(w, "bad extension id", http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		writeMethod := r.Method != http.MethodGet && r.Method != http.MethodHead
		if writeMethod {
			if webGate != nil && !webGate.RequireUnlocked(w, r) {
				return
			}
		}

		var body interface{}
		if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
			raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err == nil && len(raw) > 0 {
				_ = json.Unmarshal(raw, &body)
			}
		}
		query := map[string]string{}
		for k, vs := range r.URL.Query() {
			if len(vs) > 0 {
				query[k] = vs[0]
			}
		}
		req := map[string]interface{}{
			"method": r.Method,
			"path":   path,
			"query":  query,
			"body":   body,
		}
		param, _ := json.Marshal(req)
		rpc := extensions.ExtRPCPrefix(id) + "httphandle"
		res := invoke(rpc, []json.RawMessage{param})
		if errMsg, code := rpcResultErr(res); code != 0 {
			status := http.StatusBadGateway
			if code == -32601 {
				status = http.StatusServiceUnavailable
			}
			http.Error(w, errMsg, status)
			return
		}
		payload := res["result"]
		status := http.StatusOK
		if m, ok := payload.(map[string]interface{}); ok {
			if st, ok := asHTTPStatus(m["status"]); ok {
				status = st
			}
			if j, ok := m["json"]; ok {
				payload = j
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if !writeMethod {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(payload)
	})
}

func asHTTPStatus(v interface{}) (int, bool) {
	switch x := v.(type) {
	case float64:
		if x >= 100 && x < 600 {
			return int(x), true
		}
	case int:
		if x >= 100 && x < 600 {
			return x, true
		}
	}
	return 0, false
}
