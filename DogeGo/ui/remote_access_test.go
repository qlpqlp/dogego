// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/config"
)

func TestWebUIBindsBeyondLoopback(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:2013", false},
		{"localhost:2013", false},
		{"0.0.0.0:2013", true},
		{"192.168.1.10:2013", true},
		{"[::]:2013", true},
	}
	for _, c := range cases {
		if got := webUIBindsBeyondLoopback(c.addr); got != c.want {
			t.Fatalf("%s => %v want %v", c.addr, got, c.want)
		}
	}
}

func TestRemoteDashboardAuthRequired(t *testing.T) {
	if remoteDashboardAuthRequired(configFile(false), "0.0.0.0:2013") {
		t.Fatal("remote auth off")
	}
	if !remoteDashboardAuthRequired(configFile(true), "0.0.0.0:2013") {
		t.Fatal("remote auth on + LAN bind")
	}
	if remoteDashboardAuthRequired(configFile(true), "127.0.0.1:2013") {
		t.Fatal("loopback bind should not require remote auth gate")
	}
}

func configFile(remote bool) config.File {
	return config.File{WebUIRemoteAuth: remote}
}
