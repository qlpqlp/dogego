// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CookbookEntry is one copy-paste RPC example for integrators.
type CookbookEntry struct {
	Method  string `json:"method"`
	Summary string `json:"summary"`
	Help    string `json:"help,omitempty"`
	Params  []any  `json:"params"`
	Curl    string `json:"curl"`
	CLI     string `json:"cli"`
}

// DefaultRPCPort is the conventional DogeGo JSON-RPC port (mainnet).
const DefaultRPCPort = 22557

// BuildRPCCookbook returns curl + dogecoin-cli examples for every supported method.
func BuildRPCCookbook() []CookbookEntry {
	methods := SupportedMethods()
	sort.Strings(methods)
	out := make([]CookbookEntry, 0, len(methods))
	for _, m := range methods {
		help, _ := MethodHelp(m)
		params := CookbookExampleParams(m)
		out = append(out, CookbookEntry{
			Method:  m,
			Summary: firstSentence(help),
			Help:    strings.TrimSpace(help),
			Params:  params,
			Curl:    cookbookCurlExample(m, params),
			CLI:     cookbookCLIExample(m, params),
		})
	}
	return out
}

func cookbookCurlExample(method string, params []any) string {
	if params == nil {
		params = []any{}
	}
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "1.0",
		"id":      "dogego",
		"method":  method,
		"params":  params,
	})
	return fmt.Sprintf(
		`curl -sS --user RPCUSER:RPCPASS --data-binary '%s' -H 'content-type: application/json' http://127.0.0.1:%d/`,
		string(body), DefaultRPCPort,
	)
}

func cookbookCLIExample(method string, params []any) string {
	if params == nil {
		params = []any{}
	}
	args := make([]string, 0, len(params)+1)
	args = append(args, method)
	for _, p := range params {
		switch v := p.(type) {
		case string:
			args = append(args, fmt.Sprintf("%q", v))
		default:
			b, _ := json.Marshal(v)
			args = append(args, string(b))
		}
	}
	return fmt.Sprintf("dogego-cli %s", strings.Join(args, " "))
}
