// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ibdconvergence

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dogego/chain"
	"dogego/config"
	"dogego/store"
	"dogego/walletmigration"
)

// ProgressSnapshot captures IBD-relevant heights at one instant.
type ProgressSnapshot struct {
	Source       string `json:"source"`
	Headers      *int64 `json:"headers,omitempty"`
	Blocks       *int64 `json:"blocks,omitempty"`
	Contiguous   *int64 `json:"contiguous,omitempty"`
	RawProbe     *int64 `json:"raw_probe,omitempty"`
	ReplayTarget *int64 `json:"replay_target,omitempty"`
	IBD          *bool  `json:"ibd,omitempty"`
	ConnectBoost string `json:"connect_boost,omitempty"`
	RawInFlight  *int64 `json:"raw_in_flight,omitempty"`
}

// SnapshotOptions configures progress collection.
type SnapshotOptions struct {
	DiskOnly  bool
	ChainDir  string
	WebURL    string
	RPC       walletmigration.RPCClient
	RPCTimeout time.Duration
}

func (s ProgressSnapshot) FormatLine() string {
	parts := []string{"source=" + s.Source}
	if s.Headers != nil {
		parts = append(parts, fmt.Sprintf("headers=%d", *s.Headers))
	}
	if s.Blocks != nil {
		parts = append(parts, fmt.Sprintf("blocks=%d", *s.Blocks))
	}
	if s.Contiguous != nil {
		parts = append(parts, fmt.Sprintf("contiguous=%d", *s.Contiguous))
	}
	if s.ReplayTarget != nil {
		parts = append(parts, fmt.Sprintf("replay_target=%d", *s.ReplayTarget))
	}
	if s.RawProbe != nil {
		parts = append(parts, fmt.Sprintf("raw_probe=%d", *s.RawProbe))
	}
	if s.IBD != nil {
		parts = append(parts, fmt.Sprintf("ibd=%v", *s.IBD))
	}
	if s.ConnectBoost != "" {
		parts = append(parts, "connect_boost="+s.ConnectBoost)
	}
	return strings.Join(parts, " ")
}

// ResolveChainDir returns <datadir>/<network-subdir> for disk probes.
func ResolveChainDir(baseDataDir, network string) (string, error) {
	base, err := config.ResolveDataDir(strings.TrimSpace(baseDataDir))
	if err != nil {
		return "", err
	}
	net, err := chain.ParseNetwork(strings.TrimSpace(network))
	if err != nil {
		return "", err
	}
	sub, err := chain.ChainDataDirName(net)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, sub), nil
}

// CollectSnapshot reads RPC, optional web summary, then disk fallbacks.
func CollectSnapshot(opts SnapshotOptions) (ProgressSnapshot, error) {
	var snap ProgressSnapshot
	if opts.ChainDir != "" && snap.Source == "" {
		mergeDisk(&snap, opts.ChainDir)
	}
	if !opts.DiskOnly {
		if err := mergeRPC(&snap, opts.RPC, opts.RPCTimeout); err == nil && snap.Source == "" {
			snap.Source = "rpc"
		}
		if snap.Headers == nil || snap.Blocks == nil || snap.Contiguous == nil {
			mergeWeb(&snap, opts.WebURL)
		}
	}
	if opts.ChainDir != "" {
		mergeDisk(&snap, opts.ChainDir)
	}
	if snap.Source == "" {
		return snap, fmt.Errorf("no RPC, web, or disk progress visibility")
	}
	return snap, nil
}

func mergeRPC(snap *ProgressSnapshot, client walletmigration.RPCClient, timeout time.Duration) error {
	if strings.TrimSpace(client.BaseURL) == "" {
		return fmt.Errorf("rpc url empty")
	}
	if timeout > 0 {
		client.Timeout = timeout
	} else if client.Timeout <= 0 {
		client.Timeout = 45 * time.Second
	}
	raw, err := client.CallRaw("getblockchaininfo", nil)
	if err != nil {
		return err
	}
	var info map[string]any
	if err := json.Unmarshal(raw, &info); err != nil {
		return err
	}
	if snap.Source == "" {
		snap.Source = "rpc"
	}
	if v, ok := int64Field(info, "headers"); ok {
		snap.Headers = &v
	}
	if v, ok := int64Field(info, "blocks"); ok {
		snap.Blocks = &v
	}
	if v, ok := int64Field(info, "dogego_contiguous_raw_height"); ok {
		snap.Contiguous = &v
	}
	if v, ok := int64Field(info, "dogego_utxo_replay_target"); ok {
		snap.ReplayTarget = &v
	} else if v, ok := int64Field(info, "dogego_utxo_chain_active"); ok {
		snap.ReplayTarget = &v
	}
	if v, ok := boolField(info, "initialblockdownload"); ok {
		snap.IBD = &v
	}
	if boost := formatConnectBoost(info); boost != "" {
		snap.ConnectBoost = boost
	}
	if rs, ok := info["dogego_raw_sync"].(map[string]any); ok {
		if v, ok := int64Field(rs, "in_flight_batches"); ok {
			snap.RawInFlight = &v
		}
	}
	return nil
}

func mergeWeb(snap *ProgressSnapshot, webURL string) {
	webURL = strings.TrimSpace(webURL)
	if webURL == "" {
		webURL = strings.TrimSpace(strings.TrimRight(os.Getenv("DOGEGO_WEB_URI"), "/"))
	}
	if webURL == "" {
		port := strings.TrimSpace(os.Getenv("DOGEGO_WEB_PORT"))
		if port == "" {
			port = "2013"
		}
		webURL = "http://127.0.0.1:" + port
	}
	url := strings.TrimRight(webURL, "/") + "/api/summary"
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var sum map[string]any
	if err := json.Unmarshal(body, &sum); err != nil {
		return
	}
	if snap.Source == "" {
		snap.Source = "webui"
	}
	if snap.Headers == nil {
		if v, ok := int64Field(sum, "tip_height"); ok {
			snap.Headers = &v
		}
	}
	if snap.Blocks == nil {
		if v, ok := int64Field(sum, "chain_active_height"); ok {
			snap.Blocks = &v
		}
	}
	if snap.Contiguous == nil {
		if v, ok := int64Field(sum, "contiguous_raw_height"); ok {
			snap.Contiguous = &v
		}
	}
	if snap.ConnectBoost == "" {
		boost := formatConnectBoost(sum)
		if boost != "" {
			snap.ConnectBoost = boost
		}
	}
}

func mergeDisk(snap *ProgressSnapshot, chainDir string) {
	if tip, ok := store.ReadSegmentManifestTip(chainDir); ok {
		if snap.Headers == nil {
			snap.Headers = &tip
		}
		if snap.Source == "" {
			snap.Source = "disk"
		}
	}
	cp, err := store.LoadRawBlockSyncCheckpoint(chainDir)
	if err == nil {
		if snap.RawProbe == nil && cp.NextProbeHeight >= 0 {
			v := cp.NextProbeHeight
			snap.RawProbe = &v
			if snap.Source == "" {
				snap.Source = "disk"
			}
		}
		if snap.Contiguous == nil && cp.ContiguousRawHeight >= 0 {
			v := cp.ContiguousRawHeight
			snap.Contiguous = &v
		}
	}
}

func formatConnectBoost(m map[string]any) string {
	passes, okP := int64Field(m, "dogego_connect_catch_up_passes")
	batch, okB := int64Field(m, "dogego_connect_catch_up_batch")
	interval, okI := int64Field(m, "dogego_connect_catch_up_interval_ms")
	if !okP && !okB && !okI {
		return ""
	}
	return fmt.Sprintf("passes=%d batch=%d interval_ms=%d", passes, batch, interval)
}

func int64Field(m map[string]any, key string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case int64:
		return x, true
	case int:
		return int64(x), true
	case json.Number:
		n, err := x.Int64()
		return n, err == nil
	default:
		return 0, false
	}
}

func boolField(m map[string]any, key string) (bool, bool) {
	if m == nil {
		return false, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}
