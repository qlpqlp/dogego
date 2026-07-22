// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/cockroachdb/pebble"
)

type noopLogger struct{}

func (noopLogger) Infof(string, ...interface{})  {}
func (noopLogger) Errorf(string, ...interface{}) {}
func (noopLogger) Fatalf(string, ...interface{}) {}

// Store persists L2 blocks and L1 anchor index in Pebble.
type Store struct {
	mu   sync.RWMutex
	db   *pebble.DB
	path string
}

// OpenStore opens or creates the zkl2 Pebble database.
func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "zkl2.db")
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

func keyL2(height uint64) []byte {
	return []byte(fmt.Sprintf("l2/%020d", height))
}

func keyAnchor(hash string) []byte {
	return []byte("a/" + hash)
}

func keyAnchorByHeight(height int64, hash string) []byte {
	return []byte(fmt.Sprintf("ah/%012d/%s", height, hash))
}

func keyProofRoot(blockHash string) []byte {
	return []byte("pr/" + strings.ToLower(blockHash))
}

func keyProof(hash string) []byte {
	return []byte("p/" + strings.ToLower(hash))
}

func keyProofByTx(txid, hash string) []byte {
	return []byte(fmt.Sprintf("pt/%s/%s", strings.ToLower(txid), strings.ToLower(hash)))
}

func keyProofByBlock(blockHash, hash string) []byte {
	return []byte(fmt.Sprintf("pb/%s/%s", strings.ToLower(blockHash), strings.ToLower(hash)))
}

func keyProofByHeight(height int64, hash string) []byte {
	return []byte(fmt.Sprintf("ph/%012d/%s", height, strings.ToLower(hash)))
}

// PutProof stores a validated proof and indexes.
func (s *Store) PutProof(p Proof) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("zkl2 store closed")
	}
	if err := VerifyProof(p); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.db.Set(keyProof(p.ProofHash), raw, nil); err != nil {
		return err
	}
	_ = s.db.Set(keyProofByTx(p.TransactionID, p.ProofHash), raw, nil)
	_ = s.db.Set(keyProofByBlock(p.BlockHash, p.ProofHash), raw, nil)
	_ = s.db.Set(keyProofByHeight(p.BlockHeight, p.ProofHash), raw, pebble.Sync)
	return nil
}

// GetProof loads a proof by hash.
func (s *Store) GetProof(proofHash string) (Proof, bool, error) {
	if s == nil || s.db == nil {
		return Proof{}, false, fmt.Errorf("zkl2 store closed")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, closer, err := s.db.Get(keyProof(proofHash))
	if err == pebble.ErrNotFound {
		return Proof{}, false, nil
	}
	if err != nil {
		return Proof{}, false, err
	}
	defer closer.Close()
	var p Proof
	if err := json.Unmarshal(val, &p); err != nil {
		return Proof{}, false, err
	}
	return p, true, nil
}

// ListProofsByBlock returns proofs for a block hash.
func (s *Store) ListProofsByBlock(blockHash string, limit int) ([]Proof, error) {
	if limit <= 0 {
		limit = 100
	}
	prefix := []byte("pb/" + strings.ToLower(blockHash) + "/")
	return s.listProofsPrefix(prefix, limit)
}

// ProofCount returns the number of primary proof records (p/ keys).
func (s *Store) ProofCount() (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("zkl2 store closed")
	}
	prefix := []byte("p/")
	upper := []byte("q")
	s.mu.RLock()
	defer s.mu.RUnlock()
	iter, err := s.db.NewIter(&pebble.IterOptions{LowerBound: prefix, UpperBound: upper})
	if err != nil {
		return 0, err
	}
	defer iter.Close()
	n := 0
	for iter.First(); iter.Valid(); iter.Next() {
		n++
	}
	return n, nil
}

// ListRecentProofs returns proofs sorted by created timestamp (newest first).
func (s *Store) ListRecentProofs(limit int) ([]Proof, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("zkl2 store closed")
	}
	if limit <= 0 {
		limit = 50
	}
	prefix := []byte("p/")
	upper := []byte("q")
	s.mu.RLock()
	defer s.mu.RUnlock()
	iter, err := s.db.NewIter(&pebble.IterOptions{LowerBound: prefix, UpperBound: upper})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	var out []Proof
	for iter.First(); iter.Valid(); iter.Next() {
		var p Proof
		if json.Unmarshal(iter.Value(), &p) != nil {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedTimestamp != out[j].CreatedTimestamp {
			return out[i].CreatedTimestamp > out[j].CreatedTimestamp
		}
		return out[i].ProofHash > out[j].ProofHash
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) listProofsPrefix(prefix []byte, limit int) ([]Proof, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("zkl2 store closed")
	}
	upper := append(append([]byte(nil), prefix...), 0xff)
	s.mu.RLock()
	defer s.mu.RUnlock()
	iter, err := s.db.NewIter(&pebble.IterOptions{LowerBound: prefix, UpperBound: upper})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	var out []Proof
	seen := make(map[string]struct{})
	for iter.First(); iter.Valid(); iter.Next() {
		var p Proof
		if json.Unmarshal(iter.Value(), &p) != nil {
			continue
		}
		if _, ok := seen[p.ProofHash]; ok {
			continue
		}
		seen[p.ProofHash] = struct{}{}
		out = append(out, p)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// PutProofRoot stores overlay proof root for a Dogecoin block (not in L1).
func (s *Store) PutProofRoot(blockHash, root string, count int) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("zkl2 store closed")
	}
	raw, _ := json.Marshal(map[string]interface{}{
		"block_hash": blockHash, "proof_root": root, "count": count,
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Set(keyProofRoot(blockHash), raw, pebble.Sync)
}

// GetProofRoot loads overlay proof root for a block.
func (s *Store) GetProofRoot(blockHash string) (root string, count int, ok bool, err error) {
	if s == nil || s.db == nil {
		return "", 0, false, fmt.Errorf("zkl2 store closed")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, closer, e := s.db.Get(keyProofRoot(blockHash))
	if e == pebble.ErrNotFound {
		return "", 0, false, nil
	}
	if e != nil {
		return "", 0, false, e
	}
	defer closer.Close()
	var m map[string]interface{}
	if json.Unmarshal(val, &m) != nil {
		return "", 0, false, nil
	}
	root, _ = m["proof_root"].(string)
	if c, ok := m["count"].(float64); ok {
		count = int(c)
	}
	return root, count, true, nil
}

// PutL2Block stores an L2 block by height.
func (s *Store) PutL2Block(b L2Block) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("zkl2 store closed")
	}
	if err := ValidateL2Block(b); err != nil {
		return err
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Set(keyL2(b.Header.L2Height), raw, pebble.Sync)
}

// GetL2Block loads an L2 block by height.
func (s *Store) GetL2Block(height uint64) (L2Block, bool, error) {
	if s == nil || s.db == nil {
		return L2Block{}, false, fmt.Errorf("zkl2 store closed")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, closer, err := s.db.Get(keyL2(height))
	if err == pebble.ErrNotFound {
		return L2Block{}, false, nil
	}
	if err != nil {
		return L2Block{}, false, err
	}
	defer closer.Close()
	var b L2Block
	if err := json.Unmarshal(val, &b); err != nil {
		return L2Block{}, false, err
	}
	return b, true, nil
}

// PutAnchor indexes an L1 ZKDG anchor.
func (s *Store) PutAnchor(rec AnchorRecord) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("zkl2 store closed")
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.db.Set(keyAnchor(rec.AnchorHash), raw, nil); err != nil {
		return err
	}
	return s.db.Set(keyAnchorByHeight(rec.Height, rec.AnchorHash), raw, pebble.Sync)
}

// ListAnchors returns anchor records (newest heights first, capped).
func (s *Store) ListAnchors(limit int) ([]AnchorRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("zkl2 store closed")
	}
	if limit <= 0 {
		limit = 50
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	iter, err := s.db.NewIter(&pebble.IterOptions{LowerBound: []byte("ah/"), UpperBound: []byte("ai")})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	var out []AnchorRecord
	for iter.Last(); iter.Valid(); iter.Prev() {
		var rec AnchorRecord
		if json.Unmarshal(iter.Value(), &rec) == nil {
			out = append(out, rec)
		}
		if len(out) >= limit {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Height > out[j].Height })
	return out, nil
}

// TipL2Height returns highest stored L2 height (-1 if empty).
func (s *Store) TipL2Height() (int64, error) {
	if s == nil || s.db == nil {
		return -1, fmt.Errorf("zkl2 store closed")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	iter, err := s.db.NewIter(&pebble.IterOptions{LowerBound: []byte("l2/"), UpperBound: []byte("l3")})
	if err != nil {
		return -1, err
	}
	defer iter.Close()
	if !iter.Last() {
		return -1, nil
	}
	key := string(iter.Key())
	var h uint64
	if _, err := fmt.Sscanf(key, "l2/%020d", &h); err != nil {
		return -1, err
	}
	return int64(h), nil
}

// ListL2Blocks returns recent L2 blocks (highest first).
func (s *Store) ListL2Blocks(limit int) ([]L2Block, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("zkl2 store closed")
	}
	if limit <= 0 {
		limit = 20
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	iter, err := s.db.NewIter(&pebble.IterOptions{LowerBound: []byte("l2/"), UpperBound: []byte("l3")})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	var out []L2Block
	for iter.Last(); iter.Valid(); iter.Prev() {
		var b L2Block
		if json.Unmarshal(iter.Value(), &b) == nil {
			out = append(out, b)
		}
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// ProofHeightSummary returns heights with local proof counts (newest first, capped).
func (s *Store) ProofHeightSummary(limit int) ([]int64, []uint32, error) {
	if s == nil || s.db == nil {
		return nil, nil, fmt.Errorf("zkl2 store closed")
	}
	if limit <= 0 {
		limit = 256
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	iter, err := s.db.NewIter(&pebble.IterOptions{LowerBound: []byte("ph/"), UpperBound: []byte("pi")})
	if err != nil {
		return nil, nil, err
	}
	defer iter.Close()
	counts := make(map[int64]uint32)
	for iter.First(); iter.Valid(); iter.Next() {
		key := string(iter.Key())
		var h int64
		var hash string
		if _, err := fmt.Sscanf(key, "ph/%012d/%s", &h, &hash); err != nil {
			continue
		}
		counts[h]++
	}
	heights := make([]int64, 0, len(counts))
	for h := range counts {
		heights = append(heights, h)
	}
	sort.Slice(heights, func(i, j int) bool { return heights[i] > heights[j] })
	if len(heights) > limit {
		heights = heights[:limit]
	}
	outH := make([]int64, len(heights))
	outC := make([]uint32, len(heights))
	for i, h := range heights {
		outH[i] = h
		outC[i] = counts[h]
	}
	return outH, outC, nil
}

// ListProofHashesAtHeight returns proof hashes indexed at a block height.
func (s *Store) ListProofHashesAtHeight(height int64, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 1000
	}
	prefix := []byte(fmt.Sprintf("ph/%012d/", height))
	proofs, err := s.listProofsPrefix(prefix, limit)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(proofs))
	for _, p := range proofs {
		out = append(out, p.ProofHash)
	}
	return out, nil
}
