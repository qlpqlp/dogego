// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"strings"
	"testing"

	"dogego/config"
)

func TestDashboardURLDefaults(t *testing.T) {
	got := DashboardURL(config.File{})
	if got != "http://localhost:2013/" {
		t.Fatalf("got %q", got)
	}
}

func TestDashboardURLTLS(t *testing.T) {
	got := DashboardURL(config.File{WebUI: "localhost:2013", WebUITLSCert: "/x.pem"})
	if got != "https://localhost:2013/" {
		t.Fatalf("got %q", got)
	}
}

func TestDashboardURLTLSLocal(t *testing.T) {
	got := DashboardURL(config.File{WebUI: "localhost:2013", WebUITLSLocal: true})
	if got != "https://localhost:2013/" {
		t.Fatalf("got %q", got)
	}
}

func TestDashboardURLAllowsIPLoopback(t *testing.T) {
	got := DashboardURL(config.File{WebUI: "127.0.0.1:2013", WebUITLSLocal: true})
	if got != "https://127.0.0.1:2013/" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveCustomSchemeNode(t *testing.T) {
	f := config.File{WebUI: "localhost:2013"}
	u, err := ResolveOpenURL("dogecoin://node", f)
	if err != nil || u != "http://localhost:2013/" {
		t.Fatalf("err=%v u=%q", err, u)
	}
	u, err = ResolveOpenURL("dogecoin://node/send", f)
	if err != nil || u != "http://localhost:2013#send" {
		t.Fatalf("err=%v u=%q", err, u)
	}
}

func TestResolveOpenURLHTTPPassthrough(t *testing.T) {
	u, err := ResolveOpenURL("http://localhost:2013/", config.File{})
	if err != nil || u != "http://localhost:2013/" {
		t.Fatalf("err=%v u=%q", err, u)
	}
}

func TestHandlerCommandQuotesSpaces(t *testing.T) {
	cmd := HandlerCommand(`C:\Program Files\dogego.exe`)
	if !strings.Contains(cmd, `"C:\Program Files\dogego.exe"`) {
		t.Fatalf("cmd %q", cmd)
	}
}
