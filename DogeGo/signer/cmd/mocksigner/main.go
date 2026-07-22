// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// mocksigner is a minimal HWI-compatible stdin/stdout JSON signer for offline tests.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func main() {
	fail := false
	sleep := time.Duration(0)
	for _, a := range os.Args[1:] {
		switch a {
		case "--fail":
			fail = true
		case "--sleep":
			sleep = 5 * time.Second
		default:
			if stringsHasPrefix(a, "--sleep=") {
				d, err := time.ParseDuration(a[len("--sleep="):])
				if err == nil {
					sleep = d
				}
			}
		}
	}
	if sleep > 0 {
		time.Sleep(sleep)
	}
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		os.Exit(1)
	}
	var req struct {
		Method string                 `json:"method"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
		fmt.Fprintf(os.Stderr, "bad json: %v\n", err)
		os.Exit(1)
	}
	if fail {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
			"id": 1, "error": map[string]string{"message": "mock signer failure"},
		})
		return
	}
	var result interface{}
	switch req.Method {
	case "enumerate":
		result = []map[string]interface{}{{"type": "mock", "model": "dogego-test"}}
	case "displayaddress":
		result = "DMockSignerAddr"
	case "signpsbt":
		if req.Params != nil {
			if psbt, ok := req.Params["psbt"].(string); ok {
				result = psbt
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown method %q\n", req.Method)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"id": 1, "result": result})
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
