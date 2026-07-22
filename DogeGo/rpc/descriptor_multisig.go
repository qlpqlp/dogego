// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"dogego/secp256k1"

	"dogego/chain"
)

// parseImportDescriptor parses watch/import descriptors DogeGo supports.
func parseImportDescriptor(desc string) (parsedDescriptor, bool) {
	if p, ok := parseWatchDescriptor(desc); ok {
		return p, true
	}
	if p, ok := parseShCLTVPKHDescriptor(desc); ok {
		return p, true
	}
	if p, ok := parseShCSVPKHDescriptor(desc); ok {
		return p, true
	}
	if p, ok := parseShCLTVMultiDescriptor(desc); ok {
		return p, true
	}
	if p, ok := parseShCSVMultiDescriptor(desc); ok {
		return p, true
	}
	if p, ok := parseShMultiDescriptor(desc); ok {
		return p, true
	}
	return parseBareMultiDescriptor(desc)
}

func parseBareMultiDescriptor(desc string) (parsedDescriptor, bool) {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return parsedDescriptor{}, false
	}
	if i := strings.Index(desc, "#"); i >= 0 {
		desc = strings.TrimSpace(desc[:i])
	}
	lower := strings.ToLower(desc)
	if !strings.HasPrefix(lower, "multi(") || !strings.HasSuffix(lower, ")") {
		return parsedDescriptor{}, false
	}
	nReq, pubser, normKeys, err := parseMultiDescriptorArgs(desc)
	if err != nil {
		return parsedDescriptor{}, false
	}
	redeem, err := buildMultisigRedeemScript(nReq, pubser)
	if err != nil {
		return parsedDescriptor{}, false
	}
	var normParts []string
	normParts = append(normParts, strconv.Itoa(nReq))
	normParts = append(normParts, normKeys...)
	normalized := "multi(" + strings.Join(normParts, ",") + ")"
	return parsedDescriptor{
		normalized: normalized,
		scriptType: "bare-multi",
		redeem:     redeem,
		multiN:     nReq,
		multiKeys:  pubser,
	}, true
}

func parseShMultiDescriptor(desc string) (parsedDescriptor, bool) {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return parsedDescriptor{}, false
	}
	if i := strings.Index(desc, "#"); i >= 0 {
		desc = strings.TrimSpace(desc[:i])
	}
	lower := strings.ToLower(desc)
	if !strings.HasPrefix(lower, "sh(multi(") || !strings.HasSuffix(lower, "))") {
		return parsedDescriptor{}, false
	}
	body := strings.TrimSpace(desc[3 : len(desc)-1])
	nReq, pubser, normKeys, err := parseMultiDescriptorArgs(body)
	if err != nil {
		return parsedDescriptor{}, false
	}
	redeem, err := buildMultisigRedeemScript(nReq, pubser)
	if err != nil {
		return parsedDescriptor{}, false
	}
	var normParts []string
	normParts = append(normParts, strconv.Itoa(nReq))
	normParts = append(normParts, normKeys...)
	normalized := "sh(multi(" + strings.Join(normParts, ",") + "))"
	return parsedDescriptor{
		normalized: normalized,
		scriptType: "p2sh-multi",
		redeem:     redeem,
		p2shAddr:   "", // filled by importDescriptorOne via chain params
		multiN:     nReq,
		multiKeys:  pubser,
	}, true
}

func parseMultiDescriptorArgs(body string) (nRequired int, pubser [][]byte, normHex []string, err error) {
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(strings.ToLower(body), "multi(") || !strings.HasSuffix(body, ")") {
		return 0, nil, nil, fmt.Errorf("not multi()")
	}
	inner := strings.TrimSpace(body[6 : len(body)-1])
	parts := splitDescriptorCommaArgs(inner)
	if len(parts) < 2 {
		return 0, nil, nil, fmt.Errorf("multi: too few arguments")
	}
	nStr := strings.TrimSpace(parts[0])
	n64, err := strconv.ParseInt(nStr, 10, 64)
	if err != nil || n64 < 1 || n64 > 16 {
		return 0, nil, nil, fmt.Errorf("multi: bad n")
	}
	nRequired = int(n64)
	for i := 1; i < len(parts); i++ {
		ks := strings.TrimSpace(parts[i])
		if ks == "" {
			return 0, nil, nil, fmt.Errorf("multi: empty key")
		}
		if !isHexPubKeyString(ks) {
			return 0, nil, nil, fmt.Errorf("multi: invalid key")
		}
		raw, err := hex.DecodeString(strings.TrimPrefix(strings.ToLower(ks), "0x"))
		if err != nil {
			return 0, nil, nil, err
		}
		pk, err := secp256k1.ParsePubKey(raw)
		if err != nil {
			return 0, nil, nil, err
		}
		var ser []byte
		switch len(raw) {
		case 33:
			ser = pk.SerializeCompressed()
		case 65:
			ser = pk.SerializeUncompressed()
		default:
			return 0, nil, nil, fmt.Errorf("multi: invalid key length")
		}
		pubser = append(pubser, ser)
		normHex = append(normHex, hex.EncodeToString(ser))
	}
	return nRequired, pubser, normHex, nil
}

func splitDescriptorCommaArgs(s string) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '(':
			depth++
			cur.WriteRune(r)
		case ')':
			depth--
			cur.WriteRune(r)
		case ',':
			if depth == 0 {
				out = append(out, cur.String())
				cur.Reset()
				continue
			}
			cur.WriteRune(r)
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func descriptorWalletMultisigSolvable(chainName string, paths *DataPaths, nRequired int, pubkeys [][]byte) bool {
	if paths == nil || nRequired < 1 || len(pubkeys) < nRequired {
		return false
	}
	want := make(map[string]struct{})
	for _, pk := range pubkeys {
		want[hex.EncodeToString(pk)] = struct{}{}
	}
	matched := 0
	seenWIF := make(map[string]struct{})
	addWIF := func(wif string) {
		wif = strings.TrimSpace(wif)
		if wif == "" {
			return
		}
		if _, ok := seenWIF[wif]; ok {
			return
		}
		seenWIF[wif] = struct{}{}
		pubHex, err := hexPubKeyFromWIF(chainName, wif)
		if err != nil {
			return
		}
		if _, ok := want[pubHex]; ok {
			matched++
		}
	}
	if paths.WalletKnownAddresses != nil && paths.WalletWIFForAddress != nil {
		for _, addr := range paths.WalletKnownAddresses() {
			wif, err := paths.WalletWIFForAddress(addr)
			if err != nil {
				continue
			}
			addWIF(wif)
			if matched >= nRequired {
				return true
			}
		}
	}
	for _, wif := range rpcWalletWIFs(paths) {
		addWIF(wif)
		if matched >= nRequired {
			return true
		}
	}
	return matched >= nRequired
}

func multisigPubkeySetHex(pubkeys [][]byte) map[string]struct{} {
	out := make(map[string]struct{}, len(pubkeys))
	for _, pk := range pubkeys {
		out[hex.EncodeToString(pk)] = struct{}{}
	}
	return out
}

func validateDescriptorImportKeys(chainName string, parsed parsedDescriptor, keys []string) (int, string) {
	for _, wif := range keys {
		wif = strings.TrimSpace(wif)
		if wif == "" {
			continue
		}
		switch parsed.scriptType {
		case "pkh":
			addr, err := addressFromWIF(chainName, wif)
			if err != nil {
				return -8, "invalid key in keys array"
			}
			if addr != parsed.addr {
				return -8, "address in descriptor does not match private key"
			}
		case "p2sh-pkh", "p2sh-cltv-pkh", "p2sh-csv-pkh":
			addr, err := addressFromWIF(chainName, wif)
			if err != nil {
				return -8, "invalid key in keys array"
			}
			if addr != parsed.addr {
				return -8, "address in descriptor does not match private key"
			}
		case "p2sh-multi", "p2sh-cltv-multi", "p2sh-csv-multi", "bare-multi":
			pubHex, err := hexPubKeyFromWIF(chainName, wif)
			if err != nil {
				return -8, "invalid key in keys array"
			}
			if _, ok := multisigPubkeySetHex(parsed.multiKeys)[pubHex]; !ok {
				return -8, "private key is not included in the descriptor"
			}
		}
	}
	return 0, ""
}

func hexPubKeyFromWIF(chainName, wif string) (string, error) {
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return "", err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", err
	}
	secret, compressed, err := chain.DecodeWIF(strings.TrimSpace(wif), p.PrivKeyWIFVersion)
	if err != nil {
		return "", err
	}
	priv, _ := secp256k1.PrivKeyFromBytes(secret)
	pub := priv.PubKey().SerializeCompressed()
	if !compressed {
		pub = priv.PubKey().SerializeUncompressed()
	}
	return hex.EncodeToString(pub), nil
}
