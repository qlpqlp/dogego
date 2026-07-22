// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func invokeJSONRPCRaw(host string, port int, user, pass, method string, params []any, timeout time.Duration) (json.RawMessage, error) {
	addr := fmt.Sprintf("http://%s:%d/", strings.TrimSpace(host), port)
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "1.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, addr, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	client := &http.Client{Timeout: timeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	if wrap.Error != nil {
		return nil, fmt.Errorf("%s", wrap.Error.Message)
	}
	return wrap.Result, nil
}

func invokeJSONRPC(host string, port int, user, pass, method string, params []any, timeout time.Duration) (map[string]any, error) {
	raw, err := invokeJSONRPCRaw(host, port, user, pass, method, params, timeout)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func invokeJSONRPCString(host string, port int, user, pass, method string, params []any, timeout time.Duration) (string, error) {
	raw, err := invokeJSONRPCRaw(host, port, user, pass, method, params, timeout)
	if err != nil {
		return "", err
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

func invokeJSONRPCStringSlice(host string, port int, user, pass, method string, params []any, timeout time.Duration) ([]string, error) {
	raw, err := invokeJSONRPCRaw(host, port, user, pass, method, params, timeout)
	if err != nil {
		return nil, err
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func chainSnap(m map[string]any) (blocks int64, chain string) {
	if m == nil {
		return 0, ""
	}
	if v, ok := m["chain"].(string); ok {
		chain = v
	}
	switch b := m["blocks"].(type) {
	case float64:
		blocks = int64(b)
	case int64:
		blocks = b
	case json.Number:
		blocks, _ = b.Int64()
	}
	return blocks, chain
}
