// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ChainPolicy persists operator chain preferences (Core invalidateblock / preciousblock).
type ChainPolicy struct {
	path     string
	mu       sync.Mutex
	invalid  map[string]struct{} // display block hashes (lowercase hex)
	precious string                // display hex of precious block, or ""
}

type chainPolicyFile struct {
	Invalid  []string `json:"invalid"`
	Precious string   `json:"precious,omitempty"`
}

// LoadChainPolicy loads or creates chain_policy.json under chainDataDir.
func LoadChainPolicy(chainDataDir string) (*ChainPolicy, error) {
	p := &ChainPolicy{
		path:    filepath.Join(chainDataDir, "chain_policy.json"),
		invalid: make(map[string]struct{}),
	}
	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return p, nil
		}
		return nil, err
	}
	var f chainPolicyFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("chain_policy.json: %w", err)
	}
	for _, h := range f.Invalid {
		h = normDisplayHash(h)
		if len(h) == 64 {
			p.invalid[h] = struct{}{}
		}
	}
	p.precious = normDisplayHash(f.Precious)
	return p, nil
}

func normDisplayHash(s string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.ToLower(s), "0x"))
}

// IsInvalid reports whether a display block hash is marked invalid.
func (p *ChainPolicy) IsInvalid(displayHex string) bool {
	if p == nil {
		return false
	}
	displayHex = normDisplayHash(displayHex)
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.invalid[displayHex]
	return ok
}

// PreciousHash returns the marked precious block display hash, or "".
func (p *ChainPolicy) PreciousHash() string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.precious
}

// AddInvalid marks a block hash invalid and persists.
func (p *ChainPolicy) AddInvalid(displayHex string) error {
	if p == nil {
		return fmt.Errorf("chain policy: nil")
	}
	displayHex = normDisplayHash(displayHex)
	if len(displayHex) != 64 {
		return fmt.Errorf("invalid block hash length")
	}
	p.mu.Lock()
	p.invalid[displayHex] = struct{}{}
	p.mu.Unlock()
	return p.save()
}

// RemoveInvalid clears invalid status (reconsiderblock).
func (p *ChainPolicy) RemoveInvalid(displayHex string) error {
	if p == nil {
		return fmt.Errorf("chain policy: nil")
	}
	displayHex = normDisplayHash(displayHex)
	p.mu.Lock()
	delete(p.invalid, displayHex)
	p.mu.Unlock()
	return p.save()
}

// SetPrecious records preciousblock preference.
func (p *ChainPolicy) SetPrecious(displayHex string) error {
	if p == nil {
		return fmt.Errorf("chain policy: nil")
	}
	displayHex = normDisplayHash(displayHex)
	if len(displayHex) != 64 {
		return fmt.Errorf("invalid block hash length")
	}
	p.mu.Lock()
	p.precious = displayHex
	p.mu.Unlock()
	return p.save()
}

func (p *ChainPolicy) save() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	inv := make([]string, 0, len(p.invalid))
	for h := range p.invalid {
		inv = append(inv, h)
	}
	f := chainPolicyFile{Invalid: inv, Precious: p.precious}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := p.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p.path)
}
