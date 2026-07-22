// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEverySupportedMethodHasHelp(t *testing.T) {
	for _, m := range SupportedMethods() {
		if _, ok := rpcMethodHelp[m]; !ok {
			t.Fatalf("missing rpcMethodHelp for %q", m)
		}
	}
}

func TestExecHelpList(t *testing.T) {
	v, code, msg := execHelp(nil)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		t.Fatalf("result %#v", v)
	}
}

func TestExecHelpKnownCommand(t *testing.T) {
	p := []json.RawMessage{json.RawMessage(`"getblockcount"`)}
	v, code, msg := execHelp(p)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		t.Fatalf("result %#v", v)
	}
}

func TestExecHelpUnknownCommand(t *testing.T) {
	p := []json.RawMessage{json.RawMessage(`"not_a_real_rpc"`)}
	v, code, msg := execHelp(p)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	s, ok := v.(string)
	if !ok || s != "help: unknown command: not_a_real_rpc" {
		t.Fatalf("got %q", s)
	}
}

func TestExecHelpTooManyParams(t *testing.T) {
	p := []json.RawMessage{json.RawMessage(`"a"`), json.RawMessage(`"b"`)}
	_, code, msg := execHelp(p)
	if code != -32602 || msg == "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecHelpInvalidCommandType(t *testing.T) {
	p := []json.RawMessage{json.RawMessage(`123`)}
	_, code, msg := execHelp(p)
	if code != -32602 {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
}

func TestExecHelpDogegoWalletImportMethods(t *testing.T) {
	for _, m := range []string{"dogego_importmnemonic", "dogego_importbip38", "dogego_listwalletaddresses"} {
		m := m
		t.Run(m, func(t *testing.T) {
			p := []json.RawMessage{json.RawMessage(`"` + m + `"`)}
			v, code, msg := execHelp(p)
			if code != 0 || msg != "" {
				t.Fatalf("code=%d msg=%q", code, msg)
			}
			s, ok := v.(string)
			if !ok || s == "" || s == "help: unknown command: "+m {
				t.Fatalf("help for %q: %q", m, s)
			}
		})
	}
}

func TestExecHelpGetAddressesByLabelMentionsCoreShape(t *testing.T) {
	p := []json.RawMessage{json.RawMessage(`"getaddressesbylabel"`)}
	v, code, msg := execHelp(p)
	if code != 0 || msg != "" {
		t.Fatalf("code=%d msg=%q", code, msg)
	}
	s, ok := v.(string)
	if !ok || !strings.Contains(s, "Core-shaped object") {
		t.Fatalf("help=%#v", v)
	}
}
