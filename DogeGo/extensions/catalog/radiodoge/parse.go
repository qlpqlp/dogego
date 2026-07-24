// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package radiodoge

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	// Matches wallet-style confirmation lines from Heltec V3 display logs.
	reDOGEConfirm = regexp.MustCompile(`DOGECOIN_RESPONSE:\s*\{[^}]*"result"\s*:\s*"([^"]*)"[^}]*"error"\s*:\s*([^,}]+)`)
	// Hex payloads that look like serialized Dogecoin txs (version + vin marker).
	reTxHex = regexp.MustCompile(`(?i)(?:message|data|tx|transaction)[=:\s]+([0-9a-f]{100,})`)
	reLooseHex = regexp.MustCompile(`(?i)\b((?:01000000|02000000)[0-9a-f]{80,})\b`)
)

// Confirmation is a gateway response seen in device logs.
type Confirmation struct {
	Result string
	Error  string
	OK     bool
}

// ParseConfirmations extracts DOGECOIN_RESPONSE entries from /api/logs JSON or text.
func ParseConfirmations(logsBody string) []Confirmation {
	var out []Confirmation
	for _, line := range expandLogLines(logsBody) {
		m := reDOGEConfirm.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		c := Confirmation{
			Result: strings.TrimSpace(m[1]),
			Error:  strings.TrimSpace(m[2]),
		}
		c.OK = c.Result != "" && (c.Error == "" || c.Error == "null")
		out = append(out, c)
	}
	return out
}

// ExtractTxHexCandidates finds likely signed-tx hex blobs in logs (inbound mesh RX).
func ExtractTxHexCandidates(logsBody string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(h string) {
		h = strings.ToLower(strings.TrimSpace(h))
		if len(h)%2 != 0 || len(h) < 100 {
			return
		}
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	for _, line := range expandLogLines(logsBody) {
		for _, m := range reTxHex.FindAllStringSubmatch(line, -1) {
			if len(m) > 1 {
				add(m[1])
			}
		}
		for _, m := range reLooseHex.FindAllStringSubmatch(line, -1) {
			if len(m) > 1 {
				add(m[1])
			}
		}
	}
	return out
}

func expandLogLines(body string) []string {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}
	var wrapped struct {
		Logs []string `json:"logs"`
	}
	if json.Unmarshal([]byte(body), &wrapped) == nil && len(wrapped.Logs) > 0 {
		return wrapped.Logs
	}
	// Plain text or JSON array of strings.
	var arr []string
	if json.Unmarshal([]byte(body), &arr) == nil {
		return arr
	}
	return strings.Split(body, "\n")
}

// MatchConfirmation returns true if logs contain a successful confirmation for txid.
func MatchConfirmation(logsBody, txid string) bool {
	txid = strings.ToLower(strings.TrimSpace(txid))
	if txid == "" {
		return false
	}
	for _, c := range ParseConfirmations(logsBody) {
		if !c.OK {
			continue
		}
		if strings.EqualFold(c.Result, txid) {
			return true
		}
	}
	// Firmware sometimes echoes "transaction already in block chain".
	low := strings.ToLower(logsBody)
	return strings.Contains(low, strings.ToLower(txid)) && strings.Contains(low, "already in block chain")
}
