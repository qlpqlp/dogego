// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dgr

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	learnedRelaysFileName = "dgr_learned_relays.json"
	maxLearnedRelays      = 32
)

type learnedRelaysFile struct {
	Version   int      `json:"version"`
	UpdatedAt string   `json:"updated_at,omitempty"`
	Relays    []string `json:"relays"`
}

// LearnedRelayStore persists DGR operators discovered via P2P NODE_DOGEGO_RELAY_CGNAT.
// Kept separate from operator-edited relay_seeds until merged into discovery / conf.
type LearnedRelayStore struct {
	mu   sync.Mutex
	path string
	list []string
}

// OpenLearnedRelayStore loads or creates a learned-relay list under dir.
func OpenLearnedRelayStore(dir string) *LearnedRelayStore {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return &LearnedRelayStore{}
	}
	s := &LearnedRelayStore{path: filepath.Join(dir, learnedRelaysFileName)}
	_ = s.load()
	return s
}

func (s *LearnedRelayStore) load() error {
	if s == nil || s.path == "" {
		return nil
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var f learnedRelaysFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	s.list = normalizeRelayList(f.Relays, maxLearnedRelays)
	return nil
}

// List returns a copy of learned QUIC host:port targets.
func (s *LearnedRelayStore) List() []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.list...)
}

// Note adds a QUIC relay target learned from a DGR operator handshake.
func (s *LearnedRelayStore) Note(hostport string, relayPort int) bool {
	if s == nil {
		return false
	}
	hostport = normalizeHostPort(hostport, relayPort)
	if hostport == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.list {
		if strings.EqualFold(x, hostport) {
			return false
		}
	}
	s.list = append([]string{hostport}, s.list...)
	if len(s.list) > maxLearnedRelays {
		s.list = s.list[:maxLearnedRelays]
	}
	_ = s.saveLocked()
	return true
}

// MergeMany notes several targets; returns how many were newly added.
func (s *LearnedRelayStore) MergeMany(hostports []string, relayPort int) int {
	n := 0
	for _, hp := range hostports {
		if s.Note(hp, relayPort) {
			n++
		}
	}
	return n
}

func (s *LearnedRelayStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	f := learnedRelaysFile{
		Version:   1,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Relays:    append([]string(nil), s.list...),
	}
	body, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func normalizeRelayList(in []string, max int) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range in {
		hp := normalizeHostPort(raw, 24433)
		if hp == "" {
			continue
		}
		key := strings.ToLower(hp)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, hp)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

// MergeRelaySeedLists merges preferred then extra without duplicates (cap max).
func MergeRelaySeedLists(preferred, extra []string, max int) []string {
	if max <= 0 {
		max = maxLearnedRelays
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(list []string) {
		for _, raw := range list {
			hp := normalizeHostPort(raw, 24433)
			if hp == "" {
				continue
			}
			key := strings.ToLower(hp)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, hp)
			if len(out) >= max {
				return
			}
		}
	}
	add(preferred)
	add(extra)
	return out
}

// ShuffleSecure randomly permutes targets with crypto/rand (peer-selection privacy).
func ShuffleSecure(targets []string) {
	if len(targets) < 2 {
		return
	}
	for i := len(targets) - 1; i > 0; i-- {
		j := secureIntn(i + 1)
		targets[i], targets[j] = targets[j], targets[i]
	}
}

func secureIntn(n int) int {
	if n <= 1 {
		return 0
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return int(time.Now().UnixNano() % int64(n))
	}
	return int(binary.LittleEndian.Uint64(b[:]) % uint64(n))
}
