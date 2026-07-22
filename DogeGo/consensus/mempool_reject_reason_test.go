// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"fmt"
	"testing"
)

func TestMempoolRejectReason(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{ErrMempoolCoinbase, "coinbase"},
		{ErrMissingPrevout, "Missing inputs"},
		{ErrMinRelayFee, "mempool min fee not met"},
		{fmt.Errorf("%w: dust", ErrNonStandardTx), "dust"},
		{fmt.Errorf("bad-txns-vin-empty"), "bad-txns-vin-empty"},
		{fmt.Errorf("%w (input 0)", ErrSpendInMempool), "txn-mempool-conflict"},
		{fmt.Errorf("%w (input 0)", ErrSpendOnChain), "bad-txns-inputs-spent"},
		{ErrRBFInsufficientFee, "insufficient fee"},
		{fmt.Errorf("%w (101 > 100)", ErrRBFTooManyConflicts), "too many potential replacements"},
		{fmt.Errorf("%w (abc:0)", ErrRBFNewUnconfirmedInput), "replacement-adds-unconfirmed"},
		{fmt.Errorf("too-long-mempool-chain: 26 ancestors > 25"), "too-long-mempool-chain"},
		{fmt.Errorf("mempool full (1)"), "mempool full"},
		{fmt.Errorf("script-verify: signature failed"), "mandatory-script-verify-flag-failed (script-verify: signature failed)"},
		{fmt.Errorf("script-verify: DISCOURAGE_UPGRADABLE_NOPS"), "mandatory-script-verify-flag-failed (OP_SUCCESSx reserved)"},
		{fmt.Errorf("script-verify: SIG_NULLDUMMY"), "mandatory-script-verify-flag-failed (NULLDUMMY verification failure)"},
	}
	for _, tc := range cases {
		if got := MempoolRejectReason(tc.err); got != tc.want {
			t.Errorf("%v: got %q want %q", tc.err, got, tc.want)
		}
	}
}

func TestMempoolRejectReasonWraps(t *testing.T) {
	inner := fmt.Errorf("input 0: %w", ErrMinRelayFee)
	if got := MempoolRejectReason(inner); got != "mempool min fee not met" {
		t.Fatalf("got %q", got)
	}
	if !errors.Is(inner, ErrMinRelayFee) {
		t.Fatal("wrap broken")
	}
}
