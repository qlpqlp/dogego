// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// BanManager backs setban / listbanned / clearbanned when assigned to DataPaths.BanManager.
type BanManager interface {
	ListBanned() []map[string]interface{}
	ClearBanned()
	SetBan(subnet string, command string, banTime int64, absolute bool) error
	IsBanned(ip net.IP) bool
}

type memoryBanEntry struct {
	address     string
	bannedUntil int64
	banCreated  int64
	banReason   string
}

// MemoryBanManager is a small in-process ban map (Core-shaped list entries) for tests or embedded nodes.
type MemoryBanManager struct {
	mu      sync.Mutex
	entries map[string]memoryBanEntry // canonical key -> entry
}

// banStillActive reports whether a ban entry is in force (banned_until 0 = never expires, Core-style permanent).
func banStillActive(bannedUntil, now int64) bool {
	if bannedUntil == 0 {
		return true
	}
	return bannedUntil > now
}

// NewMemoryBanManager returns an empty ban manager.
func NewMemoryBanManager() *MemoryBanManager {
	return &MemoryBanManager{entries: make(map[string]memoryBanEntry)}
}

func (m *MemoryBanManager) canonicalSubnet(subnet string) (key, display string, err error) {
	s := strings.TrimSpace(subnet)
	if s == "" {
		return "", "", fmt.Errorf("empty subnet")
	}
	if strings.Contains(s, "/") {
		_, ipnet, e := net.ParseCIDR(s)
		if e != nil {
			return "", "", fmt.Errorf("invalid subnet")
		}
		display := ipnet.String()
		return strings.ToLower(display), display, nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return "", "", fmt.Errorf("invalid IP")
	}
	// Single host: use IP string as key (no port in setban)
	key = strings.ToLower(ip.String())
	return key, ip.String(), nil
}

// PurgeExpired removes ban entries whose banned_until is in the past.
func (m *MemoryBanManager) PurgeExpired() int {
	if m == nil {
		return 0
	}
	now := time.Now().Unix()
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for k, e := range m.entries {
		if !banStillActive(e.bannedUntil, now) {
			delete(m.entries, k)
			n++
		}
	}
	return n
}

// ListBanned implements BanManager (active bans only; expired entries are purged).
func (m *MemoryBanManager) ListBanned() []map[string]interface{} {
	if m == nil {
		return nil
	}
	m.PurgeExpired()
	return m.bannedEntriesSnapshot()
}

// bannedEntriesSnapshot returns active ban rows without pruning (caller may PurgeExpired first).
func (m *MemoryBanManager) bannedEntriesSnapshot() []map[string]interface{} {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.entries))
	for k := range m.entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]interface{}, 0, len(keys))
	for _, k := range keys {
		e := m.entries[k]
		out = append(out, map[string]interface{}{
			"address":      e.address,
			"banned_until": e.bannedUntil,
			"ban_created":  e.banCreated,
			"ban_reason":   e.banReason,
		})
	}
	return out
}

// ClearBanned implements BanManager.
func (m *MemoryBanManager) ClearBanned() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]memoryBanEntry)
}

// IsBanned reports whether ip matches an active ban entry (host or CIDR).
func (m *MemoryBanManager) IsBanned(ip net.IP) bool {
	if m == nil || ip == nil {
		return false
	}
	now := time.Now().Unix()
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToLower(ip.String())
	if e, ok := m.entries[key]; ok && banStillActive(e.bannedUntil, now) {
		return true
	}
	for k, e := range m.entries {
		if !banStillActive(e.bannedUntil, now) {
			continue
		}
		if !strings.Contains(k, "/") {
			continue
		}
		_, ipnet, err := net.ParseCIDR(k)
		if err == nil && ipnet.Contains(ip) {
			return true
		}
	}
	return false
}

// SetBan implements BanManager (add / remove).
func (m *MemoryBanManager) SetBan(subnet string, command string, banTime int64, absolute bool) error {
	key, display, err := m.canonicalSubnet(subnet)
	if err != nil {
		return fmt.Errorf("Error: Invalid IP/Subnet")
	}
	cmd := strings.ToLower(strings.TrimSpace(command))
	now := time.Now().Unix()
	m.mu.Lock()
	defer m.mu.Unlock()
	switch cmd {
	case "add":
		if _, ok := m.entries[key]; ok {
			return fmt.Errorf("Error: IP/Subnet already banned")
		}
		var until int64
		if absolute {
			until = banTime
		} else {
			d := banTime
			if d <= 0 {
				d = 86400 // Core default bantime when omitted / zero
			}
			until = now + d
		}
		m.entries[key] = memoryBanEntry{
			address:     display,
			bannedUntil: until,
			banCreated:  now,
			banReason:   "manually added",
		}
		return nil
	case "remove":
		if _, ok := m.entries[key]; !ok {
			return fmt.Errorf("Error: Unban failed. Requested address/subnet was not previously banned.")
		}
		delete(m.entries, key)
		return nil
	default:
		return fmt.Errorf("unknown setban command")
	}
}
