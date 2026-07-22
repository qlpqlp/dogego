// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// InvokeCoreJSONRPC calls Dogecoin Core (or any Bitcoin-style JSON-RPC) and returns the result raw JSON.
func InvokeCoreJSONRPC(addr, user, pass, method string, params []json.RawMessage) (json.RawMessage, int, string) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, -1, "Core RPC address not configured (core_rpc_addr)"
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	if !strings.HasSuffix(addr, "/") {
		addr += "/"
	}
	var paramAny []interface{}
	if len(params) > 0 {
		paramAny = make([]interface{}, len(params))
		for i, p := range params {
			var v interface{}
			if err := json.Unmarshal(p, &v); err != nil {
				return nil, -8, "Core RPC: bad param json"
			}
			paramAny[i] = v
		}
	}
	body, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "1.0",
		"id":      1,
		"method":  method,
		"params":  paramAny,
	})
	if err != nil {
		return nil, -1, err.Error()
	}
	req, err := http.NewRequest(http.MethodPost, addr, bytes.NewReader(body))
	if err != nil {
		return nil, -1, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, -1, "Core RPC unreachable: " + err.Error()
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, -1, err.Error()
	}
	var wrap struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, -1, "Core RPC: bad response"
	}
	if wrap.Error != nil {
		code := wrap.Error.Code
		if code == 0 {
			code = -1
		}
		return nil, code, "Core RPC: " + wrap.Error.Message
	}
	if len(wrap.Result) == 0 {
		return json.RawMessage("null"), 0, ""
	}
	return wrap.Result, 0, ""
}

func coreRPCFromPaths(paths *DataPaths) (addr, user, pass string, ok bool) {
	if paths == nil {
		return "", "", "", false
	}
	addr = strings.TrimSpace(paths.CoreRPCAddr)
	if addr == "" {
		return "", "", "", false
	}
	return addr, paths.CoreRPCUser, paths.CoreRPCPassword, true
}

func defaultCoreRPCAddr(chainName string) string {
	if chainName == "main" || chainName == "mainnet" {
		return "127.0.0.1:22555"
	}
	return "127.0.0.1:44556"
}

func resolveCoreRPC(paths *DataPaths, chainName string) (addr, user, pass string) {
	if a, u, p, ok := coreRPCFromPaths(paths); ok {
		return a, u, p
	}
	return defaultCoreRPCAddr(chainName), "", ""
}

func coreDumpWallet(paths *DataPaths, chainName, destPath string) (int, string) {
	addr, user, pass := resolveCoreRPC(paths, chainName)
	destJ, _ := json.Marshal(destPath)
	_, code, msg := InvokeCoreJSONRPC(addr, user, pass, "dumpwallet", []json.RawMessage{destJ})
	if code != 0 {
		return code, msg
	}
	return 0, ""
}
