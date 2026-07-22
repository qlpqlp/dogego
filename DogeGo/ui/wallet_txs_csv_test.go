package ui

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

func TestWalletTransactionsCSV(t *testing.T) {
	rows := []interface{}{
		map[string]interface{}{
			"time":           float64(1700000000),
			"txid":           "abc123",
			"amount":         10.5,
			"fee":            0.01,
			"confirmations":  float64(3),
			"category":       "receive",
			"tx_kind":        "received",
			"address":        "DAddr",
			"label":          "tip",
			"blockheight":    float64(100),
			"blockhash":      "blkhash",
			"trusted":        true,
			"iswatchonly":    false,
			"bip125-replaceable": "no",
		},
		map[string]interface{}{
			"time":        float64(1700000100),
			"txid":        "def456",
			"amount":      -2.0,
			"confirmations": float64(0),
			"category":    "send",
			"tx_kind":     "sent_pq",
			"pq_tag":      "pq-v1",
			"abandoned":   true,
		},
	}
	csvBytes := WalletTransactionsCSV(rows)
	r := csv.NewReader(bytes.NewReader(csvBytes))
	recs, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("rows: got %d want 3", len(recs))
	}
	if !strings.HasPrefix(recs[0][0], "time_unix") {
		t.Fatalf("header: %v", recs[0])
	}
	// Newest first (1700000100 before 1700000000)
	if recs[1][2] != "def456" || recs[2][2] != "abc123" {
		t.Fatalf("sort order: %v %v", recs[1][2], recs[2][2])
	}
	if recs[2][7] != "received" || recs[1][8] != "pq-v1" {
		t.Fatalf("kinds: %+v %+v", recs[1], recs[2])
	}
}

func TestWalletTransactionsCSVEmpty(t *testing.T) {
	csvBytes := WalletTransactionsCSV(nil)
	if !strings.Contains(string(csvBytes), "time_unix") {
		t.Fatal("expected header only")
	}
}
