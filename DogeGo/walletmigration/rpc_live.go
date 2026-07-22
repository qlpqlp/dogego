// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package walletmigration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"dogego/wallet/corewallet"
)

// RPCClient calls DogeGo JSON-RPC over HTTP (mirrors scripts/dogego_rpc.ps1 defaults).
type RPCClient struct {
	BaseURL string
	User    string
	Pass    string
	Timeout time.Duration
}

// LiveImportResult summarizes a live RPC probe/import attempt.
type LiveImportResult struct {
	Path                string                  `json:"path"`
	Probe               *corewallet.ProbeResult `json:"probe,omitempty"`
	KeysImported        int                     `json:"keys_imported,omitempty"`
	KeypoolHint         string                  `json:"keypool_hint,omitempty"`
	KeypoolRefillSize   *int                    `json:"keypool_refill_size,omitempty"`
	PoolUnmatchedHint   string                  `json:"pool_unmatched_hint,omitempty"`
	PoolIndicesReplayed *bool                   `json:"pool_indices_replayed,omitempty"`
	Status              string                  `json:"status"`
	Error               string                  `json:"error,omitempty"`
}

// DefaultRPCClient builds a client from DOGEGO_RPC_URI / DOGEGO_RPC_PORT and optional auth env.
func DefaultRPCClient() RPCClient {
	base := strings.TrimSpace(os.Getenv("DOGEGO_RPC_URI"))
	if base == "" {
		port := strings.TrimSpace(os.Getenv("DOGEGO_RPC_PORT"))
		if port == "" {
			port = "44556"
		}
		base = "http://127.0.0.1:" + port
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	return RPCClient{
		BaseURL: strings.TrimRight(base, "/"),
		User:    os.Getenv("DOGEGO_RPC_USER"),
		Pass:    os.Getenv("DOGEGO_RPC_PASS"),
		Timeout: 30 * time.Second,
	}
}

// CallRaw invokes a JSON-RPC method and returns the raw result JSON.
func (c RPCClient) CallRaw(method string, params []any) (json.RawMessage, error) {
	return c.call(method, params)
}

func (c RPCClient) call(method string, params []any) (json.RawMessage, error) {
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
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
	req, err := http.NewRequest(http.MethodPost, c.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.User != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.User+":"+c.Pass)))
	}
	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("rpc decode: %w", err)
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	return envelope.Result, nil
}

// LiveImportViaRPC probes and imports wallet.dat through a running DogeGo node.
func LiveImportViaRPC(c RPCClient, path, passphrase string) (*LiveImportResult, error) {
	out, err := liveProbeViaRPC(c, path)
	if err != nil {
		return nil, err
	}
	if out.Status == "probe_failed" || out.Status == "not_bdb" {
		return out, nil
	}
	return finishLiveImportViaRPC(c, out, path, passphrase)
}

// LiveProbeViaRPC probes wallet.dat through a running DogeGo node without importing.
func LiveProbeViaRPC(c RPCClient, path string) (*LiveImportResult, error) {
	out, err := liveProbeViaRPC(c, path)
	if err != nil {
		return nil, err
	}
	finalizeProbeStatus(out)
	return out, nil
}

func liveProbeViaRPC(c RPCClient, path string) (*LiveImportResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("wallet path required")
	}
	out := &LiveImportResult{Path: path}
	rawProbe, err := c.call("dogego_probewalletdat", []any{path})
	if err != nil {
		out.Status = "probe_failed"
		out.Error = err.Error()
		return out, nil
	}
	var probe corewallet.ProbeResult
	if err := json.Unmarshal(rawProbe, &probe); err != nil {
		out.Status = "probe_failed"
		out.Error = err.Error()
		return out, nil
	}
	out.Probe = &probe
	if !probe.IsBDB {
		out.Status = "not_bdb"
		return out, nil
	}
	return out, nil
}

func finalizeProbeStatus(out *LiveImportResult) {
	if out == nil || out.Probe == nil {
		return
	}
	probe := out.Probe
	switch {
	case probe.CanImport && !probe.NeedsPassphrase:
		out.Status = "probe_passed"
	case probe.NeedsPassphrase:
		out.Status = "probe_needs_passphrase"
	default:
		out.Status = "probe_blocked"
	}
}

func finishLiveImportViaRPC(c RPCClient, out *LiveImportResult, path, passphrase string) (*LiveImportResult, error) {
	probe := out.Probe
	switch {
	case probe.CanImport && !probe.NeedsPassphrase:
		rawImport, err := c.call("dogego_importwalletdat", []any{path, map[string]bool{"native_bdb": true}})
		if err != nil {
			out.Status = "import_failed"
			out.Error = err.Error()
			return out, nil
		}
		var imp map[string]any
		_ = json.Unmarshal(rawImport, &imp)
		out.KeysImported = intFromAny(imp["keys_imported"])
		applyImportPoolMeta(out, imp)
		out.Status = "passed"
	case probe.NeedsPassphrase && passphrase != "":
		rawImport, err := c.call("dogego_importwalletdat", []any{path, map[string]string{"passphrase": passphrase}})
		if err != nil {
			out.Status = "import_failed"
			out.Error = err.Error()
			return out, nil
		}
		var imp map[string]any
		_ = json.Unmarshal(rawImport, &imp)
		out.KeysImported = intFromAny(imp["keys_imported"])
		applyImportPoolMeta(out, imp)
		out.Status = "passed_encrypted"
	case probe.NeedsPassphrase:
		out.Status = "skipped_needs_passphrase"
	default:
		out.Status = "skipped_encrypted_or_blocked"
	}
	return out, nil
}

func applyImportPoolMeta(out *LiveImportResult, imp map[string]any) {
	if out == nil || imp == nil {
		return
	}
	if v, ok := imp["keypool_hint"].(string); ok {
		out.KeypoolHint = strings.TrimSpace(v)
	}
	if v, ok := imp["pool_unmatched_hint"].(string); ok {
		out.PoolUnmatchedHint = strings.TrimSpace(v)
	}
	if n := intFromAny(imp["keypool_refill_size"]); n > 0 {
		out.KeypoolRefillSize = &n
	}
	if v, ok := imp["pool_indices_replayed"].(bool); ok {
		out.PoolIndicesReplayed = &v
	}
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	default:
		return 0
	}
}
