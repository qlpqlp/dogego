// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"testing"

	"dogego/wire"
)

func TestEffectiveDustLimitFeeBased(t *testing.T) {
	pkScript := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	pkScript = append(pkScript, 0x88, 0xac)
	out := wire.TxOut{Value: HardDustLimitKoinu, PkScript: pkScript}
	pol := DefaultStandardPolicy()
	// At default relay fee, fee-based threshold is below hard limit for P2PKH.
	if EffectiveDustLimit(out, pol, DefaultMinRelayTxFeePerKB) != HardDustLimitKoinu {
		t.Fatalf("expected hard dust limit for typical P2PKH")
	}
	highRelay := uint64(10_000_000) // 0.1 DOGE/kB
	soft := DustThresholdForOutput(out, highRelay)
	if soft <= HardDustLimitKoinu {
		t.Fatalf("high relay should raise fee dust threshold: %d", soft)
	}
	if EffectiveDustLimit(out, pol, highRelay) != soft {
		t.Fatalf("effective limit should use fee threshold")
	}
}

func TestIsUnspendableScript(t *testing.T) {
	if !IsUnspendableScript([]byte{0x6a, 0x00}) {
		t.Fatal("OP_RETURN is unspendable")
	}
	if !IsUnspendableScript(make([]byte, MaxBlockBaseSize+1)) {
		t.Fatal("oversized script should be unspendable")
	}
}
