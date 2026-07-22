// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"strings"

	"dogego/consensus"
	"dogego/wire"
)

// isMisbehaviorBlockError reports whether a StoreValidatedBlock failure indicates a bad peer block.
func isMisbehaviorBlockError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	if strings.Contains(s, "defer connect") {
		return false
	}
	if strings.Contains(s, "missing store") {
		return false
	}
	if strings.Contains(s, "connect height") && strings.Contains(s, "pending") {
		return false
	}
	// Valid peer block; local binary still has the pre-Core legacy subsidy RNG.
	if consensus.IsLegacySubsidyBug(err) {
		return false
	}
	return true
}

func trimRejectReason(err error) string {
	if err == nil {
		return "invalid"
	}
	s := err.Error()
	if len(s) > 80 {
		return s[:80]
	}
	return s
}

// RejectInvalidBlock sends BIP61 reject for an invalid block (Core net_processing).
func RejectInvalidBlock(mw *MsgWriter, hash [32]byte, reason string) error {
	if mw == nil {
		return nil
	}
	h := hash
	pl, err := wire.EncodeReject("block", wire.RejectInvalid, reason, &h)
	if err != nil {
		return err
	}
	return mw.Write("reject", pl)
}

// RejectInvalidTx sends BIP61 reject for an invalid transaction (Core net_processing).
func RejectInvalidTx(mw *MsgWriter, hash [32]byte, reason string) error {
	return RejectTx(mw, hash, wire.RejectInvalid, reason)
}

// RejectTx sends BIP61 reject for a transaction with the given code.
func RejectTx(mw *MsgWriter, hash [32]byte, code byte, reason string) error {
	if mw == nil {
		return nil
	}
	h := hash
	pl, err := wire.EncodeReject("tx", code, reason, &h)
	if err != nil {
		return err
	}
	return mw.Write("reject", pl)
}
