// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"encoding/hex"
	"net/url"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/config"
)

func TestParsePaymentURIMainnet(t *testing.T) {
	addr := "DKznsfbYgqKSg6FW1wHhAJxwHF6VSDkHGS"
	link, ok := ParsePaymentURI("dogecoin:" + addr + "?amount=42&label=wow")
	if !ok {
		t.Fatal("expected payment URI")
	}
	if link.Address != addr || link.Amount != "42" || link.Label != "wow" || link.Network != "mainnet" {
		t.Fatalf("got %+v", link)
	}
}

func TestParsePaymentURITestnet(t *testing.T) {
	hash, err := hex.DecodeString("9131c29384f000c0d651660eefaf1717c8ca1855")
	if err != nil {
		t.Fatal(err)
	}
	addr := chain.Base58CheckEncode(0x41, hash)
	link, ok := ParsePaymentURI("dogecoin://" + addr)
	if !ok {
		t.Fatal("expected testnet payment URI")
	}
	if link.Network != "testnet" {
		t.Fatalf("network=%q", link.Network)
	}
}

func TestResolveOpenURLPaymentLink(t *testing.T) {
	f := config.File{WebUI: "127.0.0.1:2013", Network: "testnet"}
	addr := "DKznsfbYgqKSg6FW1wHhAJxwHF6VSDkHGS"
	got, err := ResolveOpenURL("dogecoin:"+addr+"?amount=1.5", f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "#send?") {
		t.Fatalf("got %q", got)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	frag := strings.TrimPrefix(u.Fragment, "send?")
	q, err := url.ParseQuery(frag)
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("to") != addr || q.Get("amount") != "1.5" {
		t.Fatalf("query=%v", q)
	}
	if q.Get("net_warn") != "mainnet" {
		t.Fatalf("expected net_warn mainnet, got %q", q.Get("net_warn"))
	}
}
