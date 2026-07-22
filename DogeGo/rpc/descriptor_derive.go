// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/chain"
)

// splitDescriptorChecksum splits "desc#checksum" when checksum is 8 hex chars (Core extractdescriptor).
func splitDescriptorChecksum(desc string) (base, checksum string) {
	desc = strings.TrimSpace(desc)
	if i := strings.LastIndex(desc, "#"); i >= 0 {
		cs := strings.TrimSpace(desc[i+1:])
		if len(cs) == 8 && isHexScriptArg(cs) {
			return strings.TrimSpace(desc[:i]), cs
		}
	}
	return desc, ""
}

// deriveAddressesFromDescriptor returns display addresses for supported output descriptors.
func deriveAddressesFromDescriptor(chainName string, desc string) ([]string, int, string) {
	base, cs := splitDescriptorChecksum(desc)
	if cs != "" {
		if descriptorChecksum(base) != cs {
			return nil, -5, "Checksum mismatch"
		}
	}
	parsed, ok := parseImportDescriptor(base)
	if !ok {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(base)), "multi(") {
			return nil, -5, "Bare multisig descriptors require permitbaremultisig"
		}
		return nil, -5, "Unsupported descriptor"
	}
	if parsed.scriptType == "bare-multi" {
		return nil, -5, "Descriptor type has no address"
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}
	_, addr, ok := pkScriptAndAddressFromParsedDescriptor(p, parsed)
	if !ok || addr == "" {
		return nil, -5, "Cannot derive address from descriptor"
	}
	return []string{addr}, 0, ""
}

// execDeriveAddresses implements Core deriveaddresses for the DogeGo descriptor subset (non-range only).
func execDeriveAddresses(chainName string, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var desc string
	if err := json.Unmarshal(params[0], &desc); err != nil {
		return nil, -8, "deriveaddresses: descriptor must be a string"
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var rangeArg interface{}
		if err := json.Unmarshal(params[1], &rangeArg); err != nil {
			return nil, -8, "deriveaddresses: range must be [begin,end]"
		}
		switch v := rangeArg.(type) {
		case []interface{}:
			if len(v) != 2 {
				return nil, -8, "deriveaddresses: range must be [begin,end]"
			}
			begin, ok0 := jsonNumberInt(v[0])
			end, ok1 := jsonNumberInt(v[1])
			if !ok0 || !ok1 || begin != 0 || end != 0 {
				return nil, -5, "Range is not needed for this descriptor"
			}
		default:
			return nil, -8, "deriveaddresses: range must be [begin,end]"
		}
	}
	addrs, code, msg := deriveAddressesFromDescriptor(chainName, desc)
	if code != 0 {
		return nil, code, "deriveaddresses: "+msg
	}
	return addrs, 0, ""
}

// execExtractDescriptor splits a descriptor and optional checksum (Core-shaped).
func execExtractDescriptor(params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var desc string
	if err := json.Unmarshal(params[0], &desc); err != nil {
		return nil, -8, "extractdescriptor: descriptor must be a string"
	}
	base, cs := splitDescriptorChecksum(desc)
	parsed, ok := parseImportDescriptor(base)
	if !ok {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(base)), "multi(") {
			return nil, -5, "extractdescriptor: bare multisig descriptors require permitbaremultisig"
		}
		return nil, -5, "extractdescriptor: unsupported descriptor"
	}
	if cs != "" && descriptorChecksum(parsed.normalized) != cs {
		return nil, -5, "extractdescriptor: checksum mismatch"
	}
	out := map[string]interface{}{
		"descriptor": parsed.normalized,
		"checksum":   descriptorChecksum(parsed.normalized),
		"isrange":    false,
	}
	if cs != "" {
		out["checksum"] = cs
	}
	return out, 0, ""
}

func jsonNumberInt(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}
