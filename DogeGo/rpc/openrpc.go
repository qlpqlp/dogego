// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"sort"
	"strings"
)

// OpenRPCDocument is a minimal OpenRPC 1.3.2 manifest for DogeGo JSON-RPC.
// See https://spec.open-rpc.org/
type OpenRPCDocument struct {
	OpenRPC    string           `json:"openrpc"`
	Info       OpenRPCInfo      `json:"info"`
	Servers    []OpenRPCServer  `json:"servers"`
	Methods    []OpenRPCMethod  `json:"methods"`
	Components map[string]any   `json:"components,omitempty"`
}

// OpenRPCInfo describes the RPC surface.
type OpenRPCInfo struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

// OpenRPCServer is one JSON-RPC HTTP endpoint.
type OpenRPCServer struct {
	Name string       `json:"name"`
	URL  string       `json:"url"`
}

// OpenRPCMethod is one JSON-RPC method entry.
type OpenRPCMethod struct {
	Name        string            `json:"name"`
	Summary     string            `json:"summary,omitempty"`
	Description string            `json:"description,omitempty"`
	Params      []OpenRPCParam    `json:"params"`
	Result      *OpenRPCResult    `json:"result,omitempty"`
	Examples    []OpenRPCExample  `json:"examples,omitempty"`
}

// OpenRPCParam is a positional JSON-RPC parameter (Dogecoin Core style).
type OpenRPCParam struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Schema      any    `json:"schema"`
}

// OpenRPCResult describes the method return value.
type OpenRPCResult struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Schema      any    `json:"schema"`
}

// OpenRPCExample is a copy-paste curl example.
type OpenRPCExample struct {
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Params          []any  `json:"params"`
	Result          any    `json:"result,omitempty"`
	ExternalDocsURL string `json:"externalDocsUrl,omitempty"`
}

var openRPCAnySchema = map[string]string{"type": "object", "description": "JSON-RPC result (method-specific)"}

// BuildOpenRPCDocument returns the machine-readable RPC catalog for integrators.
func BuildOpenRPCDocument() OpenRPCDocument {
	methods := SupportedMethods()
	sort.Strings(methods)
	out := make([]OpenRPCMethod, 0, len(methods))
	for _, m := range methods {
		help, _ := MethodHelp(m)
		help = strings.TrimSpace(help)
		params := CookbookExampleParams(m)
		entry := OpenRPCMethod{
			Name:        m,
			Summary:     firstSentence(help),
			Description: help,
			Params: []OpenRPCParam{
				{
					Name:        "params",
					Description: "Positional JSON-RPC parameters (Dogecoin Core order).",
					Required:    false,
					Schema:      openRPCAnySchema,
				},
			},
			Result: &OpenRPCResult{
				Name:        "result",
				Description: "JSON-RPC result on success.",
				Schema:      openRPCAnySchema,
			},
			Examples: []OpenRPCExample{
				{
					Name:        "curl",
					Description: cookbookCurlExample(m, params),
					Params:      params,
				},
			},
		}
		out = append(out, entry)
	}
	return OpenRPCDocument{
		OpenRPC: "1.3.2",
		Info: OpenRPCInfo{
			Title:       "DogeGo JSON-RPC",
			Description: "Dogecoin Core-compatible JSON-RPC subset implemented by DogeGo.",
			Version:     "1.0.0",
		},
		Servers: []OpenRPCServer{
			{Name: "local", URL: "http://127.0.0.1:22557/"},
		},
		Methods: out,
		Components: map[string]any{
			"securitySchemes": map[string]any{
				"httpBasic": map[string]any{
					"type": "http",
					"scheme": "basic",
				},
			},
		},
	}
}

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, ".!?"); i >= 0 {
		return strings.TrimSpace(s[:i+1])
	}
	if len(s) > 120 {
		return s[:117] + "..."
	}
	return s
}
