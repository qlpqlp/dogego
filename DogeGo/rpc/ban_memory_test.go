// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMemoryBanManagerAddListRemove(t *testing.T) {
	b := NewMemoryBanManager()
	if err := b.SetBan("10.0.0.1", "add", 0, false); err != nil {
		t.Fatal(err)
	}
	list := b.ListBanned()
	if len(list) != 1 {
		t.Fatalf("len %d", len(list))
	}
	if list[0]["address"] != "10.0.0.1" {
		t.Fatalf("%v", list[0]["address"])
	}
	if err := b.SetBan("10.0.0.1", "add", 0, false); err == nil {
		t.Fatal("expected duplicate ban error")
	}
	if err := b.SetBan("10.0.0.1", "remove", 0, false); err != nil {
		t.Fatal(err)
	}
	if len(b.ListBanned()) != 0 {
		t.Fatal("expected empty after remove")
	}
}

func TestMemoryBanManagerCIDR(t *testing.T) {
	b := NewMemoryBanManager()
	if err := b.SetBan("192.168.0.0/24", "add", 3600, false); err != nil {
		t.Fatal(err)
	}
	if len(b.ListBanned()) != 1 {
		t.Fatal(len(b.ListBanned()))
	}
}

func TestMemoryBanManagerIsBanned(t *testing.T) {
	b := NewMemoryBanManager()
	ip := net.ParseIP("203.0.113.7")
	if b.IsBanned(ip) {
		t.Fatal("expected not banned")
	}
	if err := b.SetBan("203.0.113.7", "add", 3600, false); err != nil {
		t.Fatal(err)
	}
	if !b.IsBanned(ip) {
		t.Fatal("expected banned")
	}
	if err := b.SetBan("10.20.0.0/16", "add", 3600, false); err != nil {
		t.Fatal(err)
	}
	if !b.IsBanned(net.ParseIP("10.20.5.9")) {
		t.Fatal("expected CIDR ban")
	}
	if b.IsBanned(net.ParseIP("10.21.0.1")) {
		t.Fatal("outside CIDR should not be banned")
	}
}

func TestMemoryBanManagerPermanentBan(t *testing.T) {
	b := NewMemoryBanManager()
	b.mu.Lock()
	b.entries["10.0.0.99"] = memoryBanEntry{
		address: "10.0.0.99", bannedUntil: 0, banCreated: time.Now().Unix(), banReason: "perm",
	}
	b.mu.Unlock()
	ip := net.ParseIP("10.0.0.99")
	if !b.IsBanned(ip) {
		t.Fatal("permanent ban")
	}
	if n := b.PurgeExpired(); n != 0 {
		t.Fatalf("purge %d", n)
	}
}

func TestMemoryBanManagerPrunesExpired(t *testing.T) {
	b := NewMemoryBanManager()
	past := time.Now().Unix() - 10
	b.mu.Lock()
	b.entries["10.0.0.1"] = memoryBanEntry{
		address:     "10.0.0.1",
		bannedUntil: past,
		banCreated:  past - 3600,
		banReason:   "test",
	}
	b.entries["10.0.0.2"] = memoryBanEntry{
		address:     "10.0.0.2",
		bannedUntil: time.Now().Unix() + 3600,
		banCreated:  time.Now().Unix(),
		banReason:   "active",
	}
	b.mu.Unlock()
	if n := b.PurgeExpired(); n != 1 {
		t.Fatalf("purge %d", n)
	}
	list := b.ListBanned()
	if len(list) != 1 {
		t.Fatalf("len %d", len(list))
	}
	if list[0]["address"] != "10.0.0.2" {
		t.Fatalf("%v", list[0]["address"])
	}
}

func TestHandlerSetbanListbannedWithBanManager(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	bans := NewMemoryBanManager()
	paths := &DataPaths{BanManager: bans}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"setban","params":["203.0.113.7","add"]}`)
	res, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		res.Body.Close()
		t.Fatal(err)
	}
	res.Body.Close()
	if out["error"] != nil {
		t.Fatalf("%+v", out["error"])
	}
	body2 := []byte(`{"jsonrpc":"1.0","id":2,"method":"listbanned","params":[]}`)
	res2, _ := http.Post(srv.URL, "application/json", bytes.NewReader(body2))
	var out2 map[string]interface{}
	_ = json.NewDecoder(res2.Body).Decode(&out2)
	res2.Body.Close()
	arr, ok := out2["result"].([]interface{})
	if !ok || len(arr) != 1 {
		t.Fatalf("result %#v", out2["result"])
	}
}

func TestHandlerAddnodeNotWired(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	srv := httptest.NewServer(Handler("test", j, nil, nil, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"addnode","params":["1.2.3.4:22556","add"]}`)
	res, _ := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	var out map[string]interface{}
	_ = json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	if out["error"] == nil {
		t.Fatal("expected error")
	}
}

func TestHandlerAddnodeWired(t *testing.T) {
	j := &memJournal{tip: 0, best: "a", gen: "b", count: 1, hdrs: [][]byte{make([]byte, 80)}}
	paths := &DataPaths{
		AddNode: func(node, command string) error {
			if node != "peer:22556" || command != "onetry" {
				return fmt.Errorf("unexpected")
			}
			return nil
		},
	}
	srv := httptest.NewServer(Handler("test", j, nil, paths, nil, nil, nil, true, nil))
	defer srv.Close()
	body := []byte(`{"jsonrpc":"1.0","id":1,"method":"addnode","params":["peer:22556","onetry"]}`)
	res, _ := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	var out map[string]interface{}
	_ = json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	if out["error"] != nil {
		t.Fatalf("%+v", out["error"])
	}
}
