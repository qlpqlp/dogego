// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"dogego/applog"
)

const (
	learnedAddrsFileVersion   = 1
	learnedAddrsFileVersionV2 = 2
	learnedAddrsFileVersionV3 = 3
	maxLearnedAddrsOnDisk     = maxAddrBookTotal
)

type learnedAddrsFileV3 struct {
	Version int          `json:"version"`
	NKey    string       `json:"n_key,omitempty"`
	Entries []AddrRecord `json:"entries"`
}

type learnedAddrsFile struct {
	Version int      `json:"version"`
	Addrs   []string `json:"addrs"`
}

type learnedAddrsFileV2 struct {
	Version int          `json:"version"`
	Entries []AddrRecord `json:"entries"`
}

// LoadLearnedAddrs reads <chainDir>/learned_addrs.json (missing file → nil, nil error).
func LoadLearnedAddrs(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f learnedAddrsFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("learned_addrs.json: %w", err)
	}
	if f.Version != 0 && f.Version != learnedAddrsFileVersion {
		return nil, fmt.Errorf("learned_addrs.json: unsupported version %d", f.Version)
	}
	out := make([]string, 0, len(f.Addrs))
	seen := make(map[string]struct{})
	for _, a := range f.Addrs {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(a); err != nil {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
		if len(out) >= maxLearnedAddrsOnDisk {
			break
		}
	}
	return out, nil
}

// SaveLearnedAddrs atomically writes learned_addrs.json (up to maxLearnedAddrsOnDisk entries).
func SaveLearnedAddrs(path string, addrs []string) error {
	if path == "" {
		return nil
	}
	seen := make(map[string]struct{}, len(addrs))
	unique := make([]string, 0, len(addrs))
	for _, a := range addrs {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(a); err != nil {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		unique = append(unique, a)
		if len(unique) >= maxLearnedAddrsOnDisk {
			break
		}
	}
	body, err := json.MarshalIndent(learnedAddrsFile{
		Version: learnedAddrsFileVersion,
		Addrs:   unique,
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

// LoadAddrBook reads learned_addrs.json (v1 string list or v2 AddrRecord metadata).
func LoadAddrBook(path string) (*AddrBook, error) {
	b := NewAddrBook()
	if path == "" {
		return b, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return b, nil
		}
		return nil, err
	}
	var hdr struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &hdr); err != nil {
		return nil, fmt.Errorf("learned_addrs.json: %w", err)
	}
	switch hdr.Version {
	case learnedAddrsFileVersionV3:
		var meta learnedAddrsFileV3
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, err
		}
		if k, ok := parseAddrmanKeyHex(meta.NKey); ok {
			b.mu.Lock()
			b.nKey = k
			b.mu.Unlock()
		} else {
			b.mu.Lock()
			b.nKey = newAddrmanKey()
			b.mu.Unlock()
		}
		b.loadRecords(meta.Entries)
		return b, nil
	case learnedAddrsFileVersionV2:
		var meta learnedAddrsFileV2
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, err
		}
		b.mu.Lock()
		b.nKey = newAddrmanKey()
		b.mu.Unlock()
		b.loadRecords(meta.Entries)
		return b, nil
	case 0, learnedAddrsFileVersion:
		var legacy learnedAddrsFile
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return nil, err
		}
		for _, a := range legacy.Addrs {
			b.AddSeen(a)
		}
		return b, nil
	default:
		return nil, fmt.Errorf("learned_addrs.json: unsupported version %d", hdr.Version)
	}
}

// MaybeSaveAddrBookIfDirty persists learned_addrs.json when the book changed (startup probe handshakes).
func MaybeSaveAddrBookIfDirty(path string, b *AddrBook) {
	if path == "" || b == nil {
		return
	}
	if b.takeDirty() {
		if err := SaveAddrBook(path, b); err != nil {
			applog.Line("net", "learned_addrs save: "+err.Error())
			b.markDirty()
		}
	}
}

// SaveAddrBook atomically writes v3 learned_addrs.json (Core-style nKey + bucket metadata).
func SaveAddrBook(path string, b *AddrBook) error {
	if path == "" || b == nil {
		return nil
	}
	b.mu.Lock()
	b.enforceAllBucketSlotCapsLocked()
	b.mu.Unlock()
	entries := b.cloneForSave()
	nKey := b.nKeyHex()
	body, err := json.MarshalIndent(learnedAddrsFileV3{
		Version: learnedAddrsFileVersionV3,
		NKey:    nKey,
		Entries: entries,
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
