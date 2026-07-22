package rpc

import (
	"encoding/hex"
	"testing"

	"dogego/consensus"
	"dogego/mempool"
	"dogego/wire"
)

func TestWalletPQTagFromTxHex(t *testing.T) {
	script := make([]byte, 38)
	script[0] = 0x6a
	script[1] = 0x24
	copy(script[2:6], []byte(consensus.PQTagFalcon))
	for i := 6; i < 38; i++ {
		script[i] = byte(i)
	}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{
			{Value: 1, PkScript: []byte{0x51}},
			{Value: 0, PkScript: script},
		},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if tag := walletPQTagFromTxHex(hex.EncodeToString(raw)); tag != consensus.PQTagFalcon {
		t.Fatalf("tag=%q want FLC1", tag)
	}
}

func TestWalletEnrichTxKindSentPQ(t *testing.T) {
	script := make([]byte, 38)
	script[0] = 0x6a
	script[1] = 0x24
	copy(script[2:6], []byte(consensus.PQTagFalcon))
	for i := 6; i < 38; i++ {
		script[i] = byte(i)
	}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{
			{Value: 1, PkScript: []byte{0x51}},
			{Value: 0, PkScript: script},
		},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	pool := mempool.New(10)
	_ = pool.Add(raw)
	txid := txidToRPC(tx.TxHash())
	kind, tag := walletEnrichTxKind("testnet", nil, nil, nil, pool, nil, walletTxRow{
		category: "send", txid: txid, blockHeight: -1,
	})
	if kind != "sent_pq" || tag != consensus.PQTagFalcon {
		t.Fatalf("got %q %q", kind, tag)
	}
	rows := []walletTxRow{{category: "send", txid: txid, blockHeight: -1}}
	filtered := filterWalletTxRows(rows, "", "quantum", 30, "testnet", nil, nil, nil, pool, nil)
	if len(filtered) != 1 {
		t.Fatalf("quantum filter len=%d", len(filtered))
	}
}

func TestWalletEnrichTxKindSent(t *testing.T) {
	kind, tag := walletEnrichTxKind("test", nil, nil, nil, nil, nil, walletTxRow{category: "send"})
	if kind != "sent" || tag != "" {
		t.Fatalf("got %q %q", kind, tag)
	}
}

func TestWalletEnrichTxKindReceived(t *testing.T) {
	kind, _ := walletEnrichTxKind("test", nil, nil, nil, nil, nil, walletTxRow{category: "receive"})
	if kind != "received" {
		t.Fatalf("got %q", kind)
	}
}
