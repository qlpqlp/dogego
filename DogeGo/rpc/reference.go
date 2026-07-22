// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"html"
	"sort"
	"strings"
)

// BuildRPCReferenceHTML returns a self-contained HTML RPC catalog from help.go.
func BuildRPCReferenceHTML() string {
	methods := SupportedMethods()
	sort.Strings(methods)
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8">`)
	b.WriteString(`<title>DogeGo JSON-RPC reference</title>`)
	b.WriteString(`<style>body{font-family:system-ui,sans-serif;max-width:960px;margin:2rem auto;padding:0 1rem}`)
	b.WriteString(`h1{font-size:1.4rem}table{width:100%;border-collapse:collapse;margin:1rem 0}`)
	b.WriteString(`th,td{border:1px solid #ccc;padding:.45rem .6rem;text-align:left;vertical-align:top}`)
	b.WriteString(`th{background:#f4f4f4}code{font-size:.9em}tr:nth-child(even){background:#fafafa}</style></head><body>`)
	b.WriteString(`<h1>DogeGo JSON-RPC reference</h1>`)
	b.WriteString(`<p>Auto-generated from <code>help.go</code>. Cookbooks: <code>GET /api/rpc/cookbook</code>. OpenRPC: <code>GET /api/openrpc.json</code>.</p>`)
	b.WriteString(`<table><thead><tr><th>Method</th><th>Help</th><th>curl</th></tr></thead><tbody>`)
	for _, m := range methods {
		help, _ := MethodHelp(m)
		b.WriteString("<tr><td><code>")
		b.WriteString(html.EscapeString(m))
		b.WriteString("</code></td><td>")
		b.WriteString(html.EscapeString(strings.TrimSpace(help)))
		b.WriteString("</td><td><code>")
		b.WriteString(html.EscapeString(cookbookCurlExample(m, nil)))
		b.WriteString("</code></td></tr>")
	}
	b.WriteString("</tbody></table></body></html>")
	return b.String()
}
