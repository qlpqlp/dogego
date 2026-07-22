// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"sync"
)

// SharedStore holds one read-write analytics DB for the embedded sidecar and dashboard API.
// Pebble on Windows cannot open the same directory twice, so the sidecar and /api/analytics/*
// must share this handle.
type SharedStore struct {
	mu   sync.RWMutex
	db   *DB
	path string
}

// OpenShared opens (creating if needed) the analytics Pebble store for shared use.
func OpenShared(dbPath string) (*SharedStore, error) {
	db, err := Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &SharedStore{db: db, path: dbPath}, nil
}

// Writer returns the shared read-write DB for the embedded sidecar (nil after Close).
func (s *SharedStore) Writer() *DB {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db
}

// ReadDetail loads dashboard detail from the shared DB, or from disk when the store is closed.
func (s *SharedStore) ReadDetail() (*SideDetail, error) {
	if s == nil {
		return &SideDetail{Exists: false}, nil
	}
	s.mu.RLock()
	db := s.db
	path := s.path
	s.mu.RUnlock()
	if db != nil {
		return readDetailFromOpenDB(db, path)
	}
	return ReadSideDetail(path)
}

// Close releases the shared analytics DB (sidecar must be stopped first).
func (s *SharedStore) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}
