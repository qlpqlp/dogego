// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"strings"
	"testing"

	"dogego/wire"
)

func TestParseWitnessProgramV0Keyhash(t *testing.T) {
	script := make([]byte, 22)
	script[0] = 0x00
	script[1] = 0x14
	v, ok := ParseWitnessProgram(script)
	if !ok || v != 0 {
		t.Fatalf("v=%d ok=%v", v, ok)
	}
	if ClassifyScriptPubKey(script) != ScriptWitnessProgram {
		t.Fatalf("class %v", ClassifyScriptPubKey(script))
	}
	if IsStandardScript(script, DefaultStandardPolicy()) {
		t.Fatal("witness v0 should be non-standard on Dogecoin")
	}
}

func TestIsStandardTxRejectsWitnessOutput(t *testing.T) {
	script := make([]byte, 22)
	script[0] = 0x00
	script[1] = 0x14
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000, PkScript: script}},
	}
	err := IsStandardTx(tx, DefaultStandardPolicy(), 0)
	if err == nil || (!errors.Is(err, ErrNonStandardTx) && !strings.Contains(err.Error(), "scriptpubkey")) {
		t.Fatalf("want non-standard, got %v", err)
	}
	if got := MempoolRejectReason(err); got != "scriptpubkey" && got != "non-standard" {
		t.Fatalf("reject reason %q", got)
	}
}
