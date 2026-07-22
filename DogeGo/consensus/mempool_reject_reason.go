// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"strings"
)

// MempoolRejectReason maps a mempool admission error to a Core-style reject string
// (testmempoolaccept reject-reason, BIP61 reject text).
func MempoolRejectReason(err error) string {
	if err == nil {
		return "unknown"
	}
	switch {
	case errors.Is(err, ErrMempoolCoinbase):
		return "coinbase"
	case errors.Is(err, ErrMinRelayFee):
		return "mempool min fee not met"
	case errors.Is(err, ErrAbsurdlyHighFee):
		return "absurdly-high-fee"
	case errors.Is(err, ErrNonFinalTx):
		return "non-final"
	case errors.Is(err, ErrSequenceLock):
		return "non-BIP68-final"
	case errors.Is(err, ErrCoinbaseImmature):
		return "bad-txns-premature-spend-of-coinbase"
	case errors.Is(err, ErrWitnessNotSupported):
		return "no-witness-yet"
	case errors.Is(err, ErrMissingPrevout):
		return "Missing inputs"
	case errors.Is(err, ErrTxSigops):
		return "bad-txns-too-many-sigops"
	case errors.Is(err, ErrRBFInsufficientFee):
		return "insufficient fee"
	case errors.Is(err, ErrRBFNotReplaceable):
		return "txn-mempool-conflict"
	case errors.Is(err, ErrRBFTxTooManyDescendants):
		return "too many potential replacements"
	case errors.Is(err, ErrRBFTooManyConflicts):
		return "too many potential replacements"
	case errors.Is(err, ErrRBFNewUnconfirmedInput):
		return "replacement-adds-unconfirmed"
	case errors.Is(err, ErrSpendInMempool):
		return "txn-mempool-conflict"
	case errors.Is(err, ErrSpendOnChain):
		return "bad-txns-inputs-spent"
	}
	msg := err.Error()
	if strings.Contains(msg, "mempool full") {
		return "mempool full"
	}
	if r := coreShapedRejectFromMessage(msg); r != "" {
		return r
	}
	if errors.Is(err, ErrNonStandardTx) || strings.Contains(msg, "non-standard") {
		return nonStandardRejectReason(msg)
	}
	return "mandatory-script-verify-flag-failed (DogeGo): " + strings.TrimPrefix(msg, "consensus: ")
}

func coreShapedRejectFromMessage(msg string) string {
	msg = strings.TrimPrefix(msg, "consensus: ")
	if i := strings.Index(msg, " ("); i > 0 {
		head := msg[:i]
		if strings.HasPrefix(head, "bad-txns") || strings.HasPrefix(head, "bad-cb") ||
			strings.HasPrefix(head, "too-long-mempool-chain") || strings.HasPrefix(head, "too-many-descendants") {
			return head
		}
	}
	if strings.HasPrefix(msg, "bad-txns") || strings.HasPrefix(msg, "bad-cb") {
		return msg
	}
	if strings.HasPrefix(msg, "too-long-mempool-chain") || strings.HasPrefix(msg, "too-many-descendants") {
		if k := strings.Index(msg, ": "); k > 0 {
			return msg[:k]
		}
		return msg
	}
	if strings.Contains(msg, "DISCOURAGE_UPGRADABLE_NOPS") {
		return "mandatory-script-verify-flag-failed (OP_SUCCESSx reserved)"
	}
	if strings.Contains(msg, "SIG_NULLDUMMY") {
		return "mandatory-script-verify-flag-failed (NULLDUMMY verification failure)"
	}
	if strings.Contains(msg, "script-verify") {
		detail := msg
		if j := strings.Index(detail, "script-verify"); j >= 0 {
			detail = strings.TrimSpace(detail[j:])
		}
		return "mandatory-script-verify-flag-failed (" + detail + ")"
	}
	return ""
}

func nonStandardRejectReason(msg string) string {
	const prefix = "non-standard transaction: "
	if i := strings.Index(msg, prefix); i >= 0 {
		tag := strings.TrimSpace(msg[i+len(prefix):])
		if j := strings.Index(tag, " "); j > 0 {
			tag = tag[:j]
		}
		if tag != "" {
			return tag
		}
	}
	return "non-standard"
}
