// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type slowStubExt struct {
	id     string
	block  chan struct{}
	done   chan struct{}
	once   sync.Once
}

func (s *slowStubExt) Manifest() Manifest {
	return Manifest{ID: s.id, Name: "slow", Version: "0", Entry: Entry{Type: EntryBuiltin, Module: s.id}}
}

func (s *slowStubExt) OnEnable(ctx context.Context, host Host) error {
	if s.block != nil {
		select {
		case <-s.block:
		case <-time.After(5 * time.Second):
			return context.DeadlineExceeded
		}
	}
	return nil
}

func (s *slowStubExt) OnDisable() error {
	s.once.Do(func() {
		if s.done != nil {
			close(s.done)
		}
	})
	return nil
}

func (s *slowStubExt) HandleRPC(string, []json.RawMessage, Host) (interface{}, error) {
	return nil, nil
}

func TestEnableDoesNotBlockList(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "testnet", nil)
	block := make(chan struct{})
	done := make(chan struct{})
	m.RegisterBuiltin("slow.test", func(Manifest) (Extension, error) {
		return &slowStubExt{id: "slow.test", block: block, done: done}, nil
	})
	instDir := filepath.Join(dir, "extensions", "slow.test")
	if err := os.MkdirAll(instDir, 0o755); err != nil {
		t.Fatal(err)
	}
	man := Manifest{
		ManifestVersion: ManifestVersion,
		ID:              "slow.test",
		Name:            "slow",
		Version:         "0.0.1",
		Entry:           Entry{Type: EntryBuiltin, Module: "slow.test"},
	}
	raw, _ := json.Marshal(man)
	if err := os.WriteFile(filepath.Join(instDir, ManifestFileName), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.Enable("slow.test")
	}()

	listDone := make(chan struct{})
	go func() {
		_ = m.List()
		close(listDone)
	}()

	select {
	case <-listDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("List blocked while Enable waits on slow OnEnable")
	}

	close(block)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Enable did not finish")
	}
	if err := m.Disable("slow.test"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("OnDisable did not run")
	}
}
