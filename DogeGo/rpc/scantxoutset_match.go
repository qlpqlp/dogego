// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"strings"

	"dogego/chain"
)

type scanTxOutMatcher struct {
	Desc    string
	Script  []byte
	Match   func(pkScript []byte) bool
}

func matcherScripts(m scanTxOutMatcher) [][]byte {
	if len(m.Script) == 0 {
		return nil
	}
	return [][]byte{m.Script}
}

func buildScanTxOutMatchers(chainName string, scanObjects []scanObjectDesc) ([]scanTxOutMatcher, int, string) {
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -1, "scantxoutset: " + err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -1, "scantxoutset: " + err.Error()
	}
	var out []scanTxOutMatcher
	for i, obj := range scanObjects {
		if obj.hasRange {
			return nil, -8, "scantxoutset: HD range descriptors are not supported in DogeGo"
		}
		desc := strings.TrimSpace(obj.desc)
		if desc == "" {
			return nil, -8, "scantxoutset: empty descriptor at index " + strconv.Itoa(i)
		}
		if rawSpk, ok := parseRawDescriptor(desc); ok {
			want := append([]byte(nil), rawSpk...)
			norm := "raw(" + hex.EncodeToString(want) + ")"
			out = append(out, scanTxOutMatcher{
				Desc:   norm,
				Script: want,
				Match: func(pkScript []byte) bool {
					return bytes.Equal(pkScript, want)
				},
			})
			continue
		}
		parsed, ok := parseImportDescriptor(desc)
		if !ok {
			return nil, -5, "scantxoutset: unsupported descriptor at index " + strconv.Itoa(i)
		}
		spk, _, ok := pkScriptAndAddressFromParsedDescriptor(p, parsed)
		if !ok || len(spk) == 0 {
			return nil, -5, "scantxoutset: cannot derive script for descriptor at index " + strconv.Itoa(i)
		}
		want := append([]byte(nil), spk...)
		norm := parsed.normalized
		out = append(out, scanTxOutMatcher{
			Desc:   norm,
			Script: want,
			Match: func(pkScript []byte) bool {
				return bytes.Equal(pkScript, want)
			},
		})
	}
	return out, 0, ""
}

type scanObjectDesc struct {
	desc     string
	hasRange bool
}

func parseRawDescriptor(desc string) ([]byte, bool) {
	desc = strings.TrimSpace(desc)
	if i := strings.Index(desc, "#"); i >= 0 {
		desc = strings.TrimSpace(desc[:i])
	}
	lower := strings.ToLower(desc)
	if !strings.HasPrefix(lower, "raw(") || !strings.HasSuffix(lower, ")") {
		return nil, false
	}
	inner := strings.TrimSpace(desc[4 : len(desc)-1])
	inner = strings.TrimPrefix(strings.ToLower(inner), "0x")
	raw, err := hex.DecodeString(inner)
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	return raw, true
}
