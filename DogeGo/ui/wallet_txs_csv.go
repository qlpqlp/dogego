// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"bytes"
	"encoding/csv"
	"sort"
	"strconv"
	"time"
)

// WalletTransactionsCSV exports wallet transaction rows as CSV (newest first).
func WalletTransactionsCSV(rows []interface{}) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"time_unix", "time_iso", "txid", "amount_doge", "fee_doge", "confirmations",
		"category", "tx_kind", "pq_tag", "address", "label", "blockheight", "blockhash",
		"abandoned", "bip125_replaceable", "trusted", "iswatchonly",
	})
	typed := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		m, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		typed = append(typed, m)
	}
	sort.Slice(typed, func(i, j int) bool {
		return walletTxCSVTimeUnix(typed[i]) > walletTxCSVTimeUnix(typed[j])
	})
	for _, m := range typed {
		_ = w.Write(walletTxCSVRecord(m))
	}
	w.Flush()
	return buf.Bytes()
}

func walletTxCSVRecord(m map[string]interface{}) []string {
	tUnix := walletTxCSVTimeUnix(m)
	iso := ""
	if tUnix > 0 {
		iso = time.Unix(tUnix, 0).UTC().Format(time.RFC3339)
	}
	return []string{
		strconv.FormatInt(tUnix, 10),
		iso,
		walletTxCSVString(m, "txid"),
		walletTxCSVFloat(m, "amount"),
		walletTxCSVFloat(m, "fee"),
		walletTxCSVInt(m, "confirmations"),
		walletTxCSVString(m, "category"),
		walletTxCSVString(m, "tx_kind"),
		walletTxCSVString(m, "pq_tag"),
		walletTxCSVString(m, "address"),
		walletTxCSVString(m, "label"),
		walletTxCSVInt(m, "blockheight"),
		walletTxCSVString(m, "blockhash"),
		walletTxCSVBool(m, "abandoned"),
		walletTxCSVString(m, "bip125-replaceable"),
		walletTxCSVBool(m, "trusted"),
		walletTxCSVBool(m, "iswatchonly"),
	}
}

func walletTxCSVTimeUnix(m map[string]interface{}) int64 {
	switch v := m["time"].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case jsonNumber:
		n, _ := v.Int64()
		return n
	default:
		return 0
	}
}

type jsonNumber interface {
	Int64() (int64, error)
}

func walletTxCSVString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}

func walletTxCSVFloat(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	default:
		return ""
	}
}

func walletTxCSVInt(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	default:
		return ""
	}
}

func walletTxCSVBool(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
