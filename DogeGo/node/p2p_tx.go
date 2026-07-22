// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"fmt"
	"strings"

	"dogego/applog"
	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

// ErrWitnessTxRejected is returned when an inbound tx carries witness data (DogeGo rejects at P2P).
var ErrWitnessTxRejected = errors.New("witness transaction rejected")

// RejectWitnessTxIfPresent applies Core-style early rejection for segwit payloads (BIP61 + misbehavior).
func RejectWitnessTxIfPresent(mw *MsgWriter, peerAddr string, mb *MisbehaviorTracker, tx *wire.Tx) bool {
	if tx == nil || !tx.HasWitness() {
		return false
	}
	if mb != nil && peerAddr != "" {
		mb.Note(peerAddr, misbehaviorWitnessTx, "witness transaction")
	}
	h := tx.TxHash()
	_ = RejectInvalidTx(mw, h, "no witness")
	applog.Line("mempool", fmt.Sprintf("P2P witness tx rejected from %s", peerAddr))
	return true
}

// AdmitInboundTx decodes and admits a legacy tx from P2P (witness rejected before mempool policy).
func AdmitInboundTx(pl []byte, peerAddr string, mw *MsgWriter, mb *MisbehaviorTracker, pool *mempool.Pool, orphans *mempool.OrphanPool, txIx *store.TxIndex, raw *store.RawBlockStore, j consensus.HeaderChain, net chain.Network, peerFeeFilter uint64, fullRBF bool, standard consensus.StandardPolicy, limits consensus.MempoolRelayLimits) error {
	tx, err := wire.DeserializeTx(pl)
	if err != nil {
		return err
	}
	if RejectWitnessTxIfPresent(mw, peerAddr, mb, tx) {
		return ErrWitnessTxRejected
	}
	adm := consensus.NewMempoolAdmission(pool, pool, txIx, raw, j, net)
	adm.MinRelayFeePerKB = consensus.EffectiveMinRelayFeePerKB(peerFeeFilter, pool.MinRelayFeePerKB())
	adm.FullRBF = fullRBF
	adm.Standard = standard
	limits.Apply(&adm)
	return consensus.AcceptMempoolTxWithOrphans(pl, tx, pool, orphans, adm, peerAddr)
}

// HandleInboundTxAdmissionFailure applies Core-style BIP61 reject for failed P2P tx admission.
// Orphans and duplicate spends are not rejected (orphan pool / silent drop).
func HandleInboundTxAdmissionFailure(pl []byte, peerAddr string, mw *MsgWriter, mb *MisbehaviorTracker, err error) {
	if err == nil || errors.Is(err, ErrWitnessTxRejected) || errors.Is(err, consensus.ErrOrphanTx) {
		return
	}
	if errors.Is(err, consensus.ErrSpendInMempool) || errors.Is(err, consensus.ErrSpendOnChain) {
		return
	}
	var hash [32]byte
	code := byte(wire.RejectInvalid)
	if tx, derr := wire.DeserializeTx(pl); derr == nil && tx != nil {
		hash = tx.TxHash()
	} else if len(pl) > 0 {
		code = wire.RejectMalformed
	}
	reason := trimRejectReason(errors.New(consensus.MempoolRejectReason(err)))
	switch {
	case errors.Is(err, consensus.ErrMinRelayFee):
		code = wire.RejectInsufficientFee
	case strings.Contains(err.Error(), "non-standard") || strings.Contains(err.Error(), "nonstandard"):
		code = wire.RejectNonstandard
	}
	if mb != nil && peerAddr != "" && code != wire.RejectInsufficientFee {
		mb.Note(peerAddr, misbehaviorInvalidTx, reason)
	}
	if err := RejectTx(mw, hash, code, reason); err != nil {
		applog.Line("mempool", "P2P tx reject send: "+err.Error())
	}
}
