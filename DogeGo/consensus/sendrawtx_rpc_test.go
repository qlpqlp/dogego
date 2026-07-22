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

func TestSendRawTransactionRPCError(t *testing.T) {
	code, msg := SendRawTransactionRPCError(ErrMissingPrevout)
	if code != -25 || msg != "Missing inputs" {
		t.Fatalf("got %d %q", code, msg)
	}
	code, msg = SendRawTransactionRPCError(ErrMempoolCoinbase)
	if code != -26 || msg != "coinbase" {
		t.Fatalf("got %d %q", code, msg)
	}
	code, msg = SendRawTransactionRPCError(fmt.Errorf("input 0: %w", ErrMissingPrevout))
	if code != -25 || msg != "Missing inputs" {
		t.Fatalf("wrapped: got %d %q", code, msg)
	}
	if !errors.Is(fmt.Errorf("x: %w", ErrMissingPrevout), ErrMissingPrevout) {
		t.Fatal("wrap check")
	}
}
