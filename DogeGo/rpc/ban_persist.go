// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// banFileEntry is the on-disk shape for banlist.json (Core banlist.dat subset).
type banFileEntry struct {
	Address     string `json:"address"`
	BannedUntil int64  `json:"banned_until"`
	BanCreated  int64  `json:"ban_created"`
	BanReason   string `json:"ban_reason"`
}

// FileBanManager wraps MemoryBanManager and persists bans to banlist.json under the chain datadir.
type FileBanManager struct {
	*MemoryBanManager
	path string
}

// LoadFileBanManager loads active bans from path (missing file is OK) and saves after each change.
func LoadFileBanManager(path string) *FileBanManager {
	f := &FileBanManager{
		MemoryBanManager: NewMemoryBanManager(),
		path:             path,
	}
	f.load()
	return f
}

func (f *FileBanManager) load() {
	if f == nil || f.path == "" {
		return
	}
	b, err := os.ReadFile(f.path)
	if err != nil {
		return
	}
	var rows []banFileEntry
	if err := json.Unmarshal(b, &rows); err != nil {
		return
	}
	now := time.Now().Unix()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range rows {
		if r.BannedUntil <= now {
			continue
		}
		key, display, err := f.canonicalSubnet(r.Address)
		if err != nil {
			continue
		}
		reason := r.BanReason
		if reason == "" {
			reason = "loaded from banlist"
		}
		f.entries[key] = memoryBanEntry{
			address:     display,
			bannedUntil: r.BannedUntil,
			banCreated:  r.BanCreated,
			banReason:   reason,
		}
	}
}

// ListBanned implements BanManager and persists after pruning expired entries.
func (f *FileBanManager) ListBanned() []map[string]interface{} {
	if f == nil {
		return nil
	}
	if n := f.MemoryBanManager.PurgeExpired(); n > 0 {
		_ = f.save()
	}
	return f.MemoryBanManager.ListBanned()
}

func (f *FileBanManager) save() error {
	if f == nil || f.path == "" {
		return nil
	}
	f.MemoryBanManager.PurgeExpired()
	list := f.MemoryBanManager.bannedEntriesSnapshot()
	rows := make([]banFileEntry, 0, len(list))
	for _, m := range list {
		addr, _ := m["address"].(string)
		until, _ := m["banned_until"].(int64)
		created, _ := m["ban_created"].(int64)
		reason, _ := m["ban_reason"].(string)
		rows = append(rows, banFileEntry{
			Address:     addr,
			BannedUntil: until,
			BanCreated:  created,
			BanReason:   reason,
		})
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

// SetBan implements BanManager and persists the ban list.
func (f *FileBanManager) SetBan(subnet string, command string, banTime int64, absolute bool) error {
	if err := f.MemoryBanManager.SetBan(subnet, command, banTime, absolute); err != nil {
		return err
	}
	return f.save()
}

// ClearBanned implements BanManager and persists the empty list.
func (f *FileBanManager) ClearBanned() {
	f.MemoryBanManager.ClearBanned()
	_ = f.save()
}
