// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"strings"
	"testing"

	"dogego/wire"
)

func TestMainnetFieldMultiTxBlock15504Committed(t *testing.T) {
	entry, err := mainnetFieldMultiTxBlock15504Entry()
	if err != nil {
		t.Fatalf("committed block 15504: %v", err)
	}
	if entry.Height != mainnetFieldMultiTxBlockHeight {
		t.Fatalf("height=%d want %d", entry.Height, mainnetFieldMultiTxBlockHeight)
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(entry.Hex))
	if err != nil {
		t.Fatal(err)
	}
	pb, err := wire.ParseBlock(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(pb.Txs) < 2 {
		t.Fatalf("tx count=%d want multi-tx block", len(pb.Txs))
	}
}
