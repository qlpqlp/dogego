// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dogego/applog"
)

// BlockPeerScoresFileName is stored under the per-network chain data directory.
const BlockPeerScoresFileName = "block_peer_scores.json"

const (
	blockPeerScoresFileVersion = 2
	maxBlockPeerScoresOnDisk   = 500
	blockPeerScoresSaveEvery   = 2 * time.Minute
)

type blockPeerScoresFile struct {
	Version int                          `json:"version"`
	Peers   map[string]blockPeerScoreDTO `json:"peers"`
}

type blockPeerScoreDTO struct {
	Score             int   `json:"score"`
	Blocks            int64 `json:"blocks"`
	Failures          int   `json:"failures"`
	LastOKUnix        int64 `json:"last_ok_unix,omitempty"`
	CooldownUntilUnix int64 `json:"cooldown_until_unix,omitempty"`
	Services          uint64 `json:"services,omitempty"`
	StartHeight       int32  `json:"start_height,omitempty"`
}

// LoadFromFile restores peer rankings from <chainDir>/block_peer_scores.json.
func (s *BlockPeerScorer) LoadFromFile(path string) error {
	if s == nil || path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var f blockPeerScoresFile
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("block_peer_scores.json: %w", err)
	}
	if f.Version != 0 && f.Version != blockPeerScoresFileVersion {
		return fmt.Errorf("block_peer_scores.json: unsupported version %d", f.Version)
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for addr, dto := range f.Peers {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(addr); err != nil {
			continue
		}
		e := s.entry(addr)
		e.score = dto.Score
		e.blocks = dto.Blocks
		e.failures = dto.Failures
		if dto.LastOKUnix > 0 {
			e.lastOK = time.Unix(dto.LastOKUnix, 0)
		}
		if dto.CooldownUntilUnix > 0 {
			cd := time.Unix(dto.CooldownUntilUnix, 0)
			if cd.After(now) {
				e.cooldownTo = cd
			}
		}
		e.services = dto.Services
		e.startHeight = dto.StartHeight
		n++
		if n >= maxBlockPeerScoresOnDisk {
			break
		}
	}
	if n > 0 {
		applog.Line("block", fmt.Sprintf("loaded block peer scores for %d address(es)", n))
	}
	return nil
}

// SaveToFile writes block_peer_scores.json atomically.
func (s *BlockPeerScorer) SaveToFile(path string) error {
	if s == nil || path == "" {
		return nil
	}
	s.mu.Lock()
	peers := make(map[string]blockPeerScoreDTO, len(s.entries))
	for addr, e := range s.entries {
		if e == nil {
			continue
		}
		if _, _, err := net.SplitHostPort(addr); err != nil {
			continue
		}
		dto := blockPeerScoreDTO{
			Score:    e.score,
			Blocks:   e.blocks,
			Failures: e.failures,
		}
		if !e.lastOK.IsZero() {
			dto.LastOKUnix = e.lastOK.Unix()
		}
		if !e.cooldownTo.IsZero() {
			dto.CooldownUntilUnix = e.cooldownTo.Unix()
		}
		dto.Services = e.services
		dto.StartHeight = e.startHeight
		peers[addr] = dto
		if len(peers) >= maxBlockPeerScoresOnDisk {
			break
		}
	}
	s.dirty = false
	s.mu.Unlock()

	body, err := json.MarshalIndent(blockPeerScoresFile{
		Version: blockPeerScoresFileVersion,
		Peers:   peers,
	}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *BlockPeerScorer) markDirty() {
	if s != nil {
		s.dirty = true
	}
}

// RunSaveLoop periodically flushes scores while the node runs.
func (s *BlockPeerScorer) RunSaveLoop(ctx context.Context, path string) {
	if s == nil || path == "" {
		return
	}
	tick := time.NewTicker(blockPeerScoresSaveEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = s.SaveToFile(path)
			return
		case <-tick.C:
			s.mu.Lock()
			dirty := s.dirty
			s.mu.Unlock()
			if !dirty {
				continue
			}
			if err := s.SaveToFile(path); err != nil {
				applog.Line("block", "block_peer_scores save: "+err.Error())
				s.markDirty()
			}
		}
	}
}

// KnownAddresses returns scored peer host:ports (best score first) for discovery seeding.
func (s *BlockPeerScorer) KnownAddresses() []string {
	if s == nil {
		return nil
	}
	type item struct {
		addr  string
		score int
	}
	s.mu.Lock()
	items := make([]item, 0, len(s.entries))
	for addr, e := range s.entries {
		if e == nil {
			continue
		}
		if _, _, err := net.SplitHostPort(addr); err != nil {
			continue
		}
		items = append(items, item{addr: addr, score: e.score})
	}
	s.mu.Unlock()
	sort.Slice(items, func(i, j int) bool { return items[i].score > items[j].score })
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.addr)
	}
	return out
}

// MergeDiscoveryCandidates prepends known productive peers then appends fresh discovery results.
// wantBlockHeight >= 0 deprioritizes pruned NODE_NETWORK_LIMITED peers for ancient block fetch.
func (s *BlockPeerScorer) MergeDiscoveryCandidates(discovered []string, wantBlockHeight int64) []string {
	known := s.KnownAddresses()
	seen := make(map[string]struct{}, len(known)+len(discovered))
	var out []string
	for _, a := range known {
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	for _, a := range SpreadHostPortsByGroup16(discovered) {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	if wantBlockHeight >= 0 {
		out = preferArchivalPeers(out, wantBlockHeight, s.peerMeta)
	}
	return out
}
