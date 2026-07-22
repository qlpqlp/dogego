// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"strconv"
	"strings"

	"dogego/chain"
	"dogego/consensus"
)

func parseShCLTVMultiDescriptor(desc string) (parsedDescriptor, bool) {
	return parseShTimelockMultiDescriptor(desc, "cltv", consensus.BuildCLTVMultisigRedeemScript, "p2sh-cltv-multi")
}

func parseShCSVMultiDescriptor(desc string) (parsedDescriptor, bool) {
	return parseShTimelockMultiDescriptor(desc, "csv", consensus.BuildCSVMultisigRedeemScript, "p2sh-csv-multi")
}

func parseShTimelockMultiDescriptor(
	desc string,
	tag string,
	wrap func(lockTime int64, multisig []byte) []byte,
	scriptType string,
) (parsedDescriptor, bool) {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return parsedDescriptor{}, false
	}
	if i := strings.Index(desc, "#"); i >= 0 {
		desc = strings.TrimSpace(desc[:i])
	}
	lower := strings.ToLower(desc)
	prefix := "sh(" + tag + "("
	if !strings.HasPrefix(lower, prefix) || !strings.HasSuffix(lower, ")") {
		return parsedDescriptor{}, false
	}
	body := strings.TrimSpace(desc[3 : len(desc)-1])
	tagLower := strings.ToLower(tag)
	if !strings.HasPrefix(strings.ToLower(body), tagLower+"(") {
		return parsedDescriptor{}, false
	}
	rest := body[len(tag):]
	if len(rest) < 2 || rest[0] != '(' {
		return parsedDescriptor{}, false
	}
	rest = rest[1:]
	end := strings.Index(rest, ")")
	if end < 0 {
		return parsedDescriptor{}, false
	}
	lockStr := strings.TrimSpace(rest[:end])
	lockTime, err := strconv.ParseInt(lockStr, 10, 64)
	if err != nil || lockTime < 0 {
		return parsedDescriptor{}, false
	}
	multiPart := strings.TrimSpace(rest[end+1:])
	nReq, pubser, normKeys, err := parseMultiDescriptorArgs(multiPart)
	if err != nil {
		return parsedDescriptor{}, false
	}
	ms, err := buildMultisigRedeemScript(nReq, pubser)
	if err != nil {
		return parsedDescriptor{}, false
	}
	redeem := wrap(lockTime, ms)
	var normParts []string
	normParts = append(normParts, strconv.Itoa(nReq))
	normParts = append(normParts, normKeys...)
	multiNorm := "multi(" + strings.Join(normParts, ",") + ")"
	normalized := "sh(" + tag + "(" + lockStr + ")" + multiNorm + ")"
	return parsedDescriptor{
		normalized: normalized,
		scriptType: scriptType,
		redeem:     redeem,
		multiN:     nReq,
		multiKeys:  pubser,
	}, true
}

func parseShCLTVPKHDescriptor(desc string) (parsedDescriptor, bool) {
	return parseShTimelockPKHDescriptor(desc, "cltv", consensus.BuildCLTVP2PKHRedeemScript, "p2sh-cltv-pkh")
}

func parseShCSVPKHDescriptor(desc string) (parsedDescriptor, bool) {
	return parseShTimelockPKHDescriptor(desc, "csv", consensus.BuildCSVP2PKHRedeemScript, "p2sh-csv-pkh")
}

func parseShTimelockPKHDescriptor(
	desc string,
	tag string,
	wrap func(relativeLock int64, pubKeyHash [20]byte) []byte,
	scriptType string,
) (parsedDescriptor, bool) {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return parsedDescriptor{}, false
	}
	if i := strings.Index(desc, "#"); i >= 0 {
		desc = strings.TrimSpace(desc[:i])
	}
	lower := strings.ToLower(desc)
	prefix := "sh(" + tag + "("
	if !strings.HasPrefix(lower, prefix) || !strings.HasSuffix(lower, "))") {
		return parsedDescriptor{}, false
	}
	body := strings.TrimSpace(desc[3 : len(desc)-1])
	tagLower := strings.ToLower(tag)
	if !strings.HasPrefix(strings.ToLower(body), tagLower+"(") {
		return parsedDescriptor{}, false
	}
	rest := body[len(tag):]
	if len(rest) < 2 || rest[0] != '(' {
		return parsedDescriptor{}, false
	}
	rest = rest[1:]
	end := strings.Index(rest, ")")
	if end < 0 {
		return parsedDescriptor{}, false
	}
	lockStr := strings.TrimSpace(rest[:end])
	lockTime, err := strconv.ParseInt(lockStr, 10, 64)
	if err != nil || lockTime < 0 {
		return parsedDescriptor{}, false
	}
	pkhPart := strings.TrimSpace(rest[end+1:])
	pkhLower := strings.ToLower(pkhPart)
	if !strings.HasPrefix(pkhLower, "pkh(") || !strings.HasSuffix(pkhLower, ")") {
		return parsedDescriptor{}, false
	}
	addr := strings.TrimSpace(pkhPart[4 : len(pkhPart)-1])
	if !chainLooksLikeBase58Address(addr) {
		return parsedDescriptor{}, false
	}
	_, h160, err := chain.Base58CheckDecode(addr)
	if err != nil {
		return parsedDescriptor{}, false
	}
	redeem := wrap(lockTime, h160)
	normalized := "sh(" + tag + "(" + lockStr + ")pkh(" + addr + "))"
	return parsedDescriptor{
		normalized: normalized,
		addr:       addr,
		scriptType: scriptType,
		redeem:     redeem,
	}, true
}

func descriptorScriptTypeIsP2SHMulti(scriptType string) bool {
	switch scriptType {
	case "p2sh-multi", "p2sh-cltv-multi", "p2sh-csv-multi":
		return true
	default:
		return false
	}
}

func descriptorScriptTypeUsesStoredRedeem(scriptType string) bool {
	return descriptorScriptTypeIsP2SHMulti(scriptType) || scriptType == "p2sh-cltv-pkh" || scriptType == "p2sh-csv-pkh"
}

func descriptorScriptTypeIsP2SHPKHWithKeys(scriptType string) bool {
	switch scriptType {
	case "pkh", "p2sh-pkh", "p2sh-cltv-pkh", "p2sh-csv-pkh":
		return true
	default:
		return false
	}
}
