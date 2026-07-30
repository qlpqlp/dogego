// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"net/http"
	"testing"

	"dogego/config"
)

func TestAlignWebUIWithListen(t *testing.T) {
	got := alignWebUIWithListen("10.69.0.25:2013", "localhost:2013")
	if got != "10.69.0.25:2013" {
		t.Fatalf("loopback form default should follow pup listen, got %q", got)
	}
	got = alignWebUIWithListen("10.69.0.25:2013", "127.0.0.1:2013")
	if got != "10.69.0.25:2013" {
		t.Fatalf("127.0.0.1 form default should follow pup listen, got %q", got)
	}
	got = alignWebUIWithListen("10.69.0.25:2013", "10.69.0.25:2013")
	if got != "10.69.0.25:2013" {
		t.Fatalf("already aligned: got %q", got)
	}
	got = alignWebUIWithListen("127.0.0.1:2013", "localhost:2013")
	if got != "localhost:2013" {
		t.Fatalf("loopback listen should keep form webui, got %q", got)
	}
	got = alignWebUIWithListen("0.0.0.0:2013", "localhost:2013")
	if got != "0.0.0.0:2013" {
		t.Fatalf("all-interfaces listen should replace loopback webui, got %q", got)
	}
}

func TestSetupDashboardURLForBrowserDogeboxProxy(t *testing.T) {
	f := config.File{WebUI: "10.69.0.28:2013"}
	req, err := http.NewRequest(http.MethodPost, "http://10.69.0.28:2013/api/setup", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://dogebox:10002")
	got := setupDashboardURLForBrowser(req, f)
	if got != "http://dogebox:10002/" {
		t.Fatalf("DogeBox proxy origin should win over pup bind, got %q", got)
	}
}

func TestSetupDashboardURLForBrowserSameHostKeepsPort(t *testing.T) {
	f := config.File{WebUI: "localhost:9999"}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:2013/api/setup", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://localhost:2013")
	got := setupDashboardURLForBrowser(req, f)
	if got != "http://localhost:9999/" {
		t.Fatalf("same hostname should keep configured webui port, got %q", got)
	}
}

func TestSetupDashboardURLForBrowserRefererFallback(t *testing.T) {
	f := config.File{WebUI: "10.69.0.28:2013"}
	req, err := http.NewRequest(http.MethodPost, "http://10.69.0.28:2013/api/setup", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Referer", "http://dogebox:10002/")
	got := setupDashboardURLForBrowser(req, f)
	if got != "http://dogebox:10002/" {
		t.Fatalf("Referer host should win when Origin missing, got %q", got)
	}
}
