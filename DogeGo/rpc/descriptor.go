// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"dogego/chain"
)

type parsedDescriptor struct {
	normalized string
	addr       string
	scriptType string // pkh | p2sh-pkh | p2sh-multi | bare-multi
	redeem     []byte
	p2shAddr   string
	multiN     int
	multiKeys  [][]byte
}

// parseWatchDescriptor parses Core-style watch descriptors DogeGo supports (pkh / sh(pkh)).
func parseWatchDescriptor(desc string) (parsedDescriptor, bool) {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return parsedDescriptor{}, false
	}
	if i := strings.Index(desc, "#"); i >= 0 {
		desc = strings.TrimSpace(desc[:i])
	}
	lower := strings.ToLower(desc)
	switch {
	case strings.HasPrefix(lower, "addr(") && strings.HasSuffix(lower, ")"):
		inner := strings.TrimSpace(desc[5 : len(desc)-1])
		if chainLooksLikeBase58Address(inner) {
			return parsedDescriptor{
				normalized: "addr(" + inner + ")",
				addr:       inner,
				scriptType: "pkh",
			}, true
		}
	case strings.HasPrefix(lower, "pkh(") && strings.HasSuffix(lower, ")"):
		inner := strings.TrimSpace(desc[4 : len(desc)-1])
		if chainLooksLikeBase58Address(inner) {
			return parsedDescriptor{
				normalized: "pkh(" + inner + ")",
				addr:       inner,
				scriptType: "pkh",
			}, true
		}
	case strings.HasPrefix(lower, "sh(pkh(") && strings.HasSuffix(lower, "))"):
		inner := strings.TrimSpace(desc[7 : len(desc)-2])
		if chainLooksLikeBase58Address(inner) {
			return parsedDescriptor{
				normalized: "sh(pkh(" + inner + "))",
				addr:       inner,
				scriptType: "p2sh-pkh",
			}, true
		}
	}
	return parsedDescriptor{}, false
}

func chainLooksLikeBase58Address(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 26 || len(s) > 64 {
		return false
	}
	_, _, err := chain.Base58CheckDecode(s)
	return err == nil
}

func descriptorChecksum(desc string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(desc)))
	return hex.EncodeToString(h[:4])
}

// execGetDescriptorInfo returns Core-shaped metadata for supported output descriptors.
func execGetDescriptorInfo(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var desc string
	if err := json.Unmarshal(params[0], &desc); err != nil {
		return nil, -8, "getdescriptorinfo: descriptor must be a string"
	}
	parsed, ok := parseImportDescriptor(desc)
	if !ok {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(desc)), "multi(") {
			return nil, -5, "getdescriptorinfo: bare multisig descriptors require permitbaremultisig on import"
		}
		return nil, -5, "getdescriptorinfo: unsupported descriptor (pkh, sh(pkh), sh(multi), sh(cltv/csv multi/pkh), multi only)"
	}
	var hasKey bool
	switch parsed.scriptType {
	case "pkh", "p2sh-pkh", "p2sh-cltv-pkh", "p2sh-csv-pkh":
		checkAddr := parsed.addr
		hasKey = descriptorWalletHasSpendKey(paths, checkAddr)
	case "p2sh-multi", "p2sh-cltv-multi", "p2sh-csv-multi", "bare-multi":
		hasKey = descriptorWalletMultisigSolvable(chainName, paths, parsed.multiN, parsed.multiKeys)
	default:
		return nil, -5, "getdescriptorinfo: unsupported descriptor"
	}
	return map[string]interface{}{
		"descriptor":     parsed.normalized,
		"checksum":       descriptorChecksum(parsed.normalized),
		"isrange":        false,
		"issolvable":     hasKey,
		"hasprivatekeys": hasKey,
		"watchonly":      !hasKey,
	}, 0, ""
}

func descriptorWalletHasSpendKey(paths *DataPaths, addr string) bool {
	if paths == nil || addr == "" {
		return false
	}
	if paths.WalletContainsAddress != nil && paths.WalletContainsAddress(addr) {
		return true
	}
	if paths.WalletWIFForAddress != nil {
		if _, err := paths.WalletWIFForAddress(addr); err == nil {
			return true
		}
	}
	return false
}
