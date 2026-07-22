// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
)

// TokenSummary aggregates indexed DRC-20 events for one ticker.
type TokenSummary struct {
	Tick           string `json:"tick"`
	Max            string `json:"max,omitempty"`
	Lim            string `json:"lim,omitempty"`
	Dec            string `json:"dec,omitempty"`
	DeployID       string `json:"deploy_id,omitempty"`
	DeployHeight   int64  `json:"deploy_height,omitempty"`
	MintCount      int    `json:"mint_count"`
	TransferCount  int    `json:"transfer_count"`
	EventCount     int    `json:"event_count"`
	LastHeight     int64  `json:"last_height"`
	LastOp         string `json:"last_op,omitempty"`
	UpdatedUnix    int64  `json:"updated_unix"`
}

func keyToken(tick string) []byte {
	return []byte("tk/" + strings.ToUpper(strings.TrimSpace(tick)))
}

func hexDecodePayload(hx string) ([]byte, error) {
	hx = strings.TrimSpace(hx)
	if hx == "" {
		return nil, fmt.Errorf("empty")
	}
	out := make([]byte, len(hx)/2)
	for i := 0; i+1 < len(hx); i += 2 {
		var v byte
		for _, c := range []byte{hx[i], hx[i+1]} {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= c - '0'
			case c >= 'a' && c <= 'f':
				v |= c - 'a' + 10
			case c >= 'A' && c <= 'F':
				v |= c - 'A' + 10
			default:
				return nil, fmt.Errorf("bad hex")
			}
		}
		out[i/2] = v
	}
	return out, nil
}

// GetToken returns summary for a tick.
func (s *Store) GetToken(tick string) (TokenSummary, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var z TokenSummary
	if s.db == nil {
		return z, false, fmt.Errorf("store closed")
	}
	tick = strings.ToUpper(strings.TrimSpace(tick))
	val, closer, err := s.db.Get(keyToken(tick))
	if err == pebble.ErrNotFound {
		return z, false, nil
	}
	if err != nil {
		return z, false, err
	}
	defer closer.Close()
	if err := json.Unmarshal(val, &z); err != nil {
		return z, false, err
	}
	return z, true, nil
}

// ListTokens returns recent token summaries (newest updated first, approximate).
func (s *Store) ListTokens(limit int) ([]TokenSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("store closed")
	}
	if limit <= 0 {
		limit = 50
	}
	it, err := s.db.NewIter(&pebble.IterOptions{LowerBound: []byte("tk/"), UpperBound: prefixEnd([]byte("tk/"))})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var out []TokenSummary
	for ok := it.Last(); ok && len(out) < limit; ok = it.Prev() {
		var t TokenSummary
		if json.Unmarshal(it.Value(), &t) == nil {
			out = append(out, t)
		}
	}
	return out, nil
}

// ListByTick returns recent inscriptions for a ticker.
func (s *Store) ListByTick(tick string, limit int) ([]Inscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("store closed")
	}
	if limit <= 0 {
		limit = 40
	}
	tick = strings.ToUpper(strings.TrimSpace(tick))
	prefix := []byte("t/" + tick + "/")
	it, err := s.db.NewIter(&pebble.IterOptions{LowerBound: prefix, UpperBound: prefixEnd(prefix)})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var ids []string
	for ok := it.Last(); ok && len(ids) < limit; ok = it.Prev() {
		k := string(it.Key())
		parts := strings.SplitN(k, "/", 3)
		if len(parts) == 3 {
			ids = append(ids, parts[2])
		}
	}
	out := make([]Inscription, 0, len(ids))
	for _, id := range ids {
		val, closer, err := s.db.Get(keyIns(id))
		if err != nil {
			continue
		}
		var ins Inscription
		_ = json.Unmarshal(val, &ins)
		closer.Close()
		out = append(out, ins)
	}
	return out, nil
}

// ExtConfig is local extension preferences (stored in doginals.db).
type ExtConfig struct {
	WalletRPCEnabled bool   `json:"wallet_rpc_enabled"`
	PreferredAddress string `json:"preferred_address,omitempty"`
	AutoBroadcast    bool   `json:"auto_broadcast"`
	Note             string `json:"note,omitempty"`
}

// DefaultExtConfig returns safe defaults (wallet ops off until operator enables).
func DefaultExtConfig() ExtConfig {
	return ExtConfig{
		WalletRPCEnabled: false,
		AutoBroadcast:    false,
		Note:             "Unlock the wallet from the DogeGo dashboard (authenticated session / walletpassphrase) before mint or deploy broadcast.",
	}
}

func (s *Store) GetConfig() ExtConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := DefaultExtConfig()
	if s.db == nil {
		return cfg
	}
	val, closer, err := s.db.Get(keyMeta("config"))
	if err != nil {
		return cfg
	}
	defer closer.Close()
	_ = json.Unmarshal(val, &cfg)
	return cfg
}

func (s *Store) SetConfig(cfg ExtConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("store closed")
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.db.Set(keyMeta("config"), b, pebble.Sync)
}

// ExportBackup writes settings JSON under data/backups/ and returns the payload.
func (s *Store) ExportBackup() (map[string]interface{}, error) {
	cfg := s.GetConfig()
	dir := filepath.Dir(s.path)
	backupDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"extension":   ExtensionID,
		"version":     "0.3.0",
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"config":      cfg,
		"note":        "Restoring applies config only. Index/DB files already live under data/ and survive upgrades.",
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	path := filepath.Join(backupDir, "config-"+stamp+".json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return nil, err
	}
	_ = os.WriteFile(filepath.Join(backupDir, "latest.json"), b, 0o600)
	payload["path"] = path
	return payload, nil
}

// ImportBackup restores ExtConfig from a backup object (or nested config key).
func (s *Store) ImportBackup(raw map[string]interface{}) (ExtConfig, error) {
	if raw == nil {
		return ExtConfig{}, fmt.Errorf("empty backup")
	}
	cfgMap, _ := raw["config"].(map[string]interface{})
	if cfgMap == nil {
		cfgMap = raw
	}
	b, err := json.Marshal(cfgMap)
	if err != nil {
		return ExtConfig{}, err
	}
	cfg := DefaultExtConfig()
	if err := json.Unmarshal(b, &cfg); err != nil {
		return ExtConfig{}, err
	}
	if err := s.SetConfig(cfg); err != nil {
		return ExtConfig{}, err
	}
	return cfg, nil
}

func (s *Store) CountTokens() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return 0
	}
	it, err := s.db.NewIter(&pebble.IterOptions{LowerBound: []byte("tk/"), UpperBound: prefixEnd([]byte("tk/"))})
	if err != nil {
		return 0
	}
	defer it.Close()
	n := 0
	for ok := it.First(); ok; ok = it.Next() {
		n++
	}
	return n
}
