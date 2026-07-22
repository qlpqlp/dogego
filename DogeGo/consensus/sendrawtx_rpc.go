// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "errors"

// IsMissingInputsErr reports whether err is a missing-prevout admission failure (Core sendrawtransaction -25).
func IsMissingInputsErr(err error) bool {
	return errors.Is(err, ErrMissingPrevout)
}

// SendRawTransactionRPCError maps mempool admission failure to Core sendrawtransaction RPC codes.
// Missing prevouts use RPC_TRANSACTION_ERROR (-25); policy failures use RPC_TRANSACTION_REJECTED (-26).
func SendRawTransactionRPCError(err error) (code int, message string) {
	if err == nil {
		return -26, "unknown"
	}
	if errors.Is(err, ErrMissingPrevout) {
		return -25, "Missing inputs"
	}
	return -26, MempoolRejectReason(err)
}
