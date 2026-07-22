// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

// IntegrationGuide is one language-specific JSON-RPC example.
type IntegrationGuide struct {
	Language    string `json:"language"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Example     string `json:"example"`
	Notes       string `json:"notes,omitempty"`
}

// BuildIntegrationGuides returns copy-paste examples for common languages.
func BuildIntegrationGuides() []IntegrationGuide {
	return []IntegrationGuide{
		{
			Language:    "curl",
			Title:       "curl (shell)",
			Description: "HTTP POST with Basic auth; replace USER:PASS with rpc cookie or rpc_user/rpc_password.",
			Example: `curl -sS --user 'USER:PASS' \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"1.0","id":1,"method":"getblockchaininfo","params":[]}' \
  http://127.0.0.1:22557/`,
			Notes: "Batch: POST a JSON array of request objects.",
		},
		{
			Language:    "python",
			Title:       "Python (stdlib)",
			Description: "urllib.request works without extra packages.",
			Example: `import base64, json, urllib.request

url = "http://127.0.0.1:22557/"
auth = base64.b64encode(b"rpcuser:rpcpass").decode()
body = json.dumps({"jsonrpc": "1.0", "id": 1, "method": "getmempoolinfo", "params": []}).encode()
req = urllib.request.Request(url, data=body, method="POST")
req.add_header("Content-Type", "application/json")
req.add_header("Authorization", "Basic " + auth)
with urllib.request.urlopen(req) as resp:
    print(json.load(resp))`,
		},
		{
			Language:    "go",
			Title:       "Go (net/http)",
			Description: "Use any HTTP client; method list: rpc.SupportedMethods().",
			Example: `package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func main() {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "1.0", "id": 1, "method": "getnetworkinfo", "params": []any{},
	})
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:22557/", bytes.NewReader(body))
	req.SetBasicAuth("rpcuser", "rpcpass")
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil { panic(err) }
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	fmt.Println(out)
}`,
		},
		{
			Language:    "node",
			Title:       "Node.js (fetch)",
			Description: "Node 18+ global fetch; use rpcallowip + TLS proxy for remote hosts.",
			Example: `const auth = Buffer.from("rpcuser:rpcpass").toString("base64");
const res = await fetch("http://127.0.0.1:22557/", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    Authorization: "Basic " + auth,
  },
  body: JSON.stringify({
    jsonrpc: "1.0",
    id: 1,
    method: "getblockchaininfo",
    params: [],
  }),
});
console.log(await res.json());`,
		},
		{
			Language:    "rust",
			Title:       "Rust (reqwest)",
			Description: "Add reqwest + serde_json to Cargo.toml for async JSON-RPC.",
			Example: `use reqwest::Client;
use serde_json::json;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new();
    let body = json!({"jsonrpc":"1.0","id":1,"method":"getblockcount","params":[]});
    let res = client
        .post("http://127.0.0.1:22557/")
        .basic_auth("rpcuser", Some("rpcpass"))
        .json(&body)
        .send()
        .await?;
    println!("{}", res.text().await?);
    Ok(())
}`,
		},
	}
}
