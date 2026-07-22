// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
)

type noopLogger struct{}

func (noopLogger) Infof(string, ...interface{})  {}
func (noopLogger) Errorf(string, ...interface{}) {}
func (noopLogger) Fatalf(string, ...interface{}) {}

// Store is the doginals Pebble DB (L1 index + L2 assets).
type Store struct {
	mu   sync.RWMutex
	db   *pebble.DB
	path string
}

// OpenStore opens doginals.db under dir.
func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "doginals.db")
	db, err := pebble.Open(path, &pebble.Options{Logger: noopLogger{}})
	if err != nil {
		return nil, err
	}
	return &Store{db: db, path: path}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.db.Close()
	s.db = nil
	return err
}

func keyIns(id string) []byte { return []byte("i/" + strings.ToLower(id)) }
func keyInsH(h int64, id string) []byte {
	return []byte(fmt.Sprintf("ih/%012d/%s", h, strings.ToLower(id)))
}
func keyTick(tick, id string) []byte {
	return []byte("t/" + strings.ToUpper(tick) + "/" + strings.ToLower(id))
}
func keyAsset(id string) []byte { return []byte("a/" + strings.ToLower(id)) }
func keyMeta(k string) []byte   { return []byte("m/" + k) }

func (s *Store) setMeta(k, v string) error {
	return s.db.Set(keyMeta(k), []byte(v), pebble.Sync)
}

func (s *Store) getMeta(k string) string {
	val, closer, err := s.db.Get(keyMeta(k))
	if err != nil {
		return ""
	}
	defer closer.Close()
	return string(val)
}

// IndexHeight is the last fully indexed L1 height (-1 if none).
func (s *Store) IndexHeight() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return -1
	}
	v := s.getMeta("index_height")
	if v == "" {
		return -1
	}
	var n int64
	_, _ = fmt.Sscanf(v, "%d", &n)
	return n
}

func (s *Store) SetIndexHeight(h int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setMeta("index_height", fmt.Sprintf("%d", h))
}

// PutInscription stores an L1 observation.
func (s *Store) PutInscription(ins Inscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("store closed")
	}
	if ins.RecordedUnix == 0 {
		ins.RecordedUnix = time.Now().Unix()
	}
	b, err := json.Marshal(ins)
	if err != nil {
		return err
	}
	batch := s.db.NewBatch()
	defer batch.Close()
	_ = batch.Set(keyIns(ins.ID), b, nil)
	_ = batch.Set(keyInsH(ins.Height, ins.ID), []byte{1}, nil)
	if ins.Kind == "drc20" && ins.Tick != "" {
		_ = batch.Set(keyTick(ins.Tick, ins.ID), []byte{1}, nil)
		if err := s.applyTokenToBatch(batch, ins); err != nil {
			return err
		}
	}
	return batch.Commit(pebble.Sync)
}

// applyTokenToBatch updates tk/ summary inside an open batch (caller holds s.mu).
func (s *Store) applyTokenToBatch(batch *pebble.Batch, ins Inscription) error {
	tick := strings.ToUpper(ins.Tick)
	sum := TokenSummary{Tick: tick}
	if val, closer, err := s.db.Get(keyToken(tick)); err == nil {
		_ = json.Unmarshal(val, &sum)
		closer.Close()
	}
	sum.EventCount++
	sum.LastHeight = ins.Height
	sum.LastOp = ins.Op
	sum.UpdatedUnix = time.Now().Unix()
	switch ins.Op {
	case "deploy":
		sum.DeployID = ins.ID
		sum.DeployHeight = ins.Height
		if raw, err := hexDecodePayload(ins.PayloadHex); err == nil {
			if p, ok := ParseDRC20JSON(raw); ok {
				if p.Max != "" {
					sum.Max = p.Max
				}
				if p.Lim != "" {
					sum.Lim = p.Lim
				}
			}
		} else if ins.Amount != "" {
			sum.Max = ins.Amount
		}
	case "mint":
		sum.MintCount++
	case "transfer":
		sum.TransferCount++
	}
	b, err := json.Marshal(sum)
	if err != nil {
		return err
	}
	return batch.Set(keyToken(tick), b, nil)
}

// GetInscription loads by id.
func (s *Store) GetInscription(id string) (Inscription, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var z Inscription
	if s.db == nil {
		return z, false, fmt.Errorf("store closed")
	}
	val, closer, err := s.db.Get(keyIns(id))
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

// ListInscriptions returns newest-first up to limit (scan by height index reverse).
func (s *Store) ListInscriptions(limit int) ([]Inscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("store closed")
	}
	if limit <= 0 {
		limit = 50
	}
	it, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte("ih/"),
		UpperBound: prefixEnd([]byte("ih/")),
	})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var ids []string
	for ok := it.Last(); ok && len(ids) < limit; ok = it.Prev() {
		k := string(it.Key())
		// ih/000000000123/txid...
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

// CountInscriptions approximates by iterating keys (ok for research sizes).
func (s *Store) CountInscriptions() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return 0
	}
	it, err := s.db.NewIter(&pebble.IterOptions{LowerBound: []byte("i/"), UpperBound: prefixEnd([]byte("i/"))})
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

// PutAsset stores or updates an L2 asset.
func (s *Store) PutAsset(a Asset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("store closed")
	}
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return s.db.Set(keyAsset(a.ID), b, pebble.Sync)
}

func (s *Store) GetAsset(id string) (Asset, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var z Asset
	if s.db == nil {
		return z, false, fmt.Errorf("store closed")
	}
	val, closer, err := s.db.Get(keyAsset(id))
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

func (s *Store) ListAssets(limit int) ([]Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("store closed")
	}
	if limit <= 0 {
		limit = 50
	}
	it, err := s.db.NewIter(&pebble.IterOptions{LowerBound: []byte("a/"), UpperBound: prefixEnd([]byte("a/"))})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var out []Asset
	for ok := it.Last(); ok && len(out) < limit; ok = it.Prev() {
		var a Asset
		if json.Unmarshal(it.Value(), &a) == nil {
			out = append(out, a)
		}
	}
	return out, nil
}

func (s *Store) CountAssets() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return 0
	}
	it, err := s.db.NewIter(&pebble.IterOptions{LowerBound: []byte("a/"), UpperBound: prefixEnd([]byte("a/"))})
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

// ListAssetIDs returns all L2 asset ids (for P2P inventory).
func (s *Store) ListAssetIDs(limit int) ([]string, error) {
	assets, err := s.ListAssets(limit)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(assets))
	for _, a := range assets {
		ids = append(ids, a.ID)
	}
	return ids, nil
}

func prefixEnd(p []byte) []byte {
	end := make([]byte, len(p))
	copy(end, p)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil
}

// EncodeAssetWire is JSON for P2P dasset payloads.
func EncodeAssetWire(a Asset) []byte {
	b, _ := json.Marshal(a)
	return b
}

func DecodeAssetWire(b []byte) (Asset, error) {
	var a Asset
	err := json.Unmarshal(b, &a)
	return a, err
}

// EncodeInv encodes id list as length-prefixed utf8 ids.
func EncodeInv(ids []string) []byte {
	var out []byte
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		b := []byte(id)
		var lb [2]byte
		binary.BigEndian.PutUint16(lb[:], uint16(len(b)))
		out = append(out, lb[:]...)
		out = append(out, b...)
	}
	return out
}

func DecodeInv(b []byte) []string {
	var ids []string
	i := 0
	for i+2 <= len(b) {
		n := int(binary.BigEndian.Uint16(b[i : i+2]))
		i += 2
		if n <= 0 || i+n > len(b) {
			break
		}
		ids = append(ids, string(b[i:i+n]))
		i += n
	}
	return ids
}
