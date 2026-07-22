// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/wire"
)

// MempoolRBFPool extends conflict detection with spender lookup and cluster removal for BIP125.
type MempoolRBFPool interface {
	MempoolConflictPool
	SpenderOfOutpoint(rpcPrevTxid string, vout uint32) string
	RemoveCluster(displayTxid string) ([]string, error)
	GetRawByTxID(rpcTxid string) ([]byte, error)
}

// MempoolRBFFeePackagePool extends RBF with conflict-set (descendant package) fee aggregation (Core PaysForRBF).
type MempoolRBFFeePackagePool interface {
	MempoolRBFPool
	ConflictPackageFeeSize(displayTxid string, feesKoinu map[string]int64, sizes map[string]int) (fee int64, size int, ok bool)
	RawMemPoolTxIDs() ([]string, error)
	BuildMempoolSizes() (map[string]int, error)
}

// MempoolRBFGraphPool adds descendant counts for BIP125 package limits.
type MempoolRBFGraphPool interface {
	MempoolRBFPool
	MempoolDescendantCount(displayTxid string) (int, error)
}

// ErrRBFInsufficientFee is returned when a replacement pays less than a conflicting mempool tx.
var ErrRBFInsufficientFee = fmt.Errorf("consensus: insufficient fee for BIP125 replacement")

// ErrRBFNotReplaceable is returned when a mempool conflict does not signal BIP125 replaceability.
var ErrRBFNotReplaceable = fmt.Errorf("consensus: conflicting transaction is not BIP125-replaceable")

// ErrRBFTxTooManyDescendants is returned when a replacement would exceed the mempool descendant limit.
var ErrRBFTxTooManyDescendants = fmt.Errorf("consensus: BIP125 replacement exceeds descendant limit")

// ErrRBFTooManyConflicts is returned when a replacement would evict more than the BIP125 rule-5 limit.
var ErrRBFTooManyConflicts = fmt.Errorf("consensus: BIP125 replacement conflicts with too many transactions")

// ErrRBFNewUnconfirmedInput is returned when a replacement adds an unconfirmed input not spent by a
// directly conflicting transaction (BIP125 rule 2).
var ErrRBFNewUnconfirmedInput = fmt.Errorf("consensus: BIP125 replacement adds a new unconfirmed input")

// MaxRBFReplacementCandidates matches Core MAX_REPLACEMENT_CANDIDATES (BIP125 rule 5).
const MaxRBFReplacementCandidates = 100

// TryResolveMempoolRBFConflicts removes conflict clusters when the candidate pays a higher package fee rate.
// Unless fullRBF is true, every conflict must signal BIP125 replaceability via nSequence.
func TryResolveMempoolRBFConflicts(tx *wire.Tx, pool MempoolRBFPool, view PrevOutView, fullRBF bool) error {
	if tx == nil || pool == nil {
		return fmt.Errorf("consensus: nil tx or pool")
	}
	conflicts := make(map[string]struct{})
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if IsNullOutpoint(in) {
			continue
		}
		spender := pool.SpenderOfOutpoint(txidDisplayFromLE(in.PrevHash), in.PrevIdx)
		if spender != "" {
			conflicts[spender] = struct{}{}
		}
	}
	if len(conflicts) == 0 {
		return fmt.Errorf("consensus: no mempool spender for RBF")
	}
	if !fullRBF {
		for spender := range conflicts {
			raw, err := pool.GetRawByTxID(spender)
			if err != nil {
				return ErrRBFNotReplaceable
			}
			old, err := wire.DeserializeTx(raw)
			if err != nil {
				return ErrRBFNotReplaceable
			}
			if !wire.IsBIP125Replaceable(old) {
				return fmt.Errorf("%w (%s)", ErrRBFNotReplaceable, spender)
			}
		}
	}
	if graphPool, ok := pool.(MempoolRBFGraphPool); ok {
		if n, err := graphPool.MempoolDescendantCount(txidDisplayFromLE(tx.TxHash())); err == nil && n > DefaultMaxMempoolDescendants {
			return fmt.Errorf("%w (%d)", ErrRBFTxTooManyDescendants, n)
		}
		for spender := range conflicts {
			if n, err := graphPool.MempoolDescendantCount(spender); err == nil && n > DefaultMaxMempoolDescendants {
				return fmt.Errorf("%w (conflict %s has %d)", ErrRBFTxTooManyDescendants, spender, n)
			}
		}
		// BIP125 rule 5: total original transactions replaced (conflicts + their descendants) <= 100.
		if err := checkRBFConflictCount(conflicts, graphPool); err != nil {
			return err
		}
	}

	// BIP125 rule 2: a replacement may only spend an unconfirmed input that a directly
	// conflicting transaction also spends; adding a brand-new unconfirmed parent is rejected.
	if err := checkRBFNoNewUnconfirmedInputs(tx, conflicts, pool, view); err != nil {
		return err
	}
	newFee, err := TxFee(tx, view)
	if err != nil || newFee < 0 {
		return ErrRBFInsufficientFee
	}
	newSz := len(tx.SerializeForHash())
	if newSz <= 0 {
		return ErrRBFInsufficientFee
	}
	newPkgFee, newPkgSize := newFee, newSz
	var fees map[string]int64
	var sizes map[string]int
	if pkgPool, ok := pool.(MempoolRBFFeePackagePool); ok {
		fees, sizes = rbfFeeMaps(pkgPool, view)
		if f, s, ok := replacementPackageFeeSize(tx, pkgPool, fees, sizes); ok && s > 0 && f > 0 {
			newPkgFee, newPkgSize = f, s
		}
	}
	var totalConflictFees int64
	var totalConflictSize int
	for spender := range conflicts {
		raw, err := pool.GetRawByTxID(spender)
		if err != nil {
			return ErrRBFNotReplaceable
		}
		old, err := wire.DeserializeTx(raw)
		if err != nil {
			return ErrRBFNotReplaceable
		}
		if pkgPool, ok := pool.(MempoolRBFFeePackagePool); ok && fees != nil && sizes != nil {
			if f, s, ok := pkgPool.ConflictPackageFeeSize(spender, fees, sizes); ok && s > 0 {
				totalConflictFees += f
				totalConflictSize += s
				continue
			}
		}
		oldFee, err := TxFee(old, view)
		if err != nil || oldFee < 0 {
			return ErrRBFInsufficientFee
		}
		totalConflictFees += oldFee
		totalConflictSize += len(old.SerializeForHash())
	}
	if totalConflictSize <= 0 {
		totalConflictSize = newPkgSize
	}
	minNewFee := totalConflictFees + FeeForSize(IncrementalRelayFeePerKB(), newPkgSize)
	if newPkgFee < minNewFee {
		return fmt.Errorf("%w: need %d koinu, have %d", ErrRBFInsufficientFee, minNewFee, newPkgFee)
	}
	newRate := newPkgFee * 1000 / int64(newPkgSize)
	confRate := totalConflictFees * 1000 / int64(totalConflictSize)
	if newRate <= confRate {
		return fmt.Errorf("%w: package rate %d <= conflict %d", ErrRBFInsufficientFee, newRate, confRate)
	}
	for spender := range conflicts {
		if _, err := pool.RemoveCluster(spender); err != nil {
			return err
		}
	}
	return nil
}

// checkRBFConflictCount enforces BIP125 rule 5: the number of original transactions to be
// replaced (each conflict plus its in-mempool descendants) must not exceed 100.
func checkRBFConflictCount(conflicts map[string]struct{}, graphPool MempoolRBFGraphPool) error {
	total := 0
	for spender := range conflicts {
		total++ // the conflict itself
		if n, err := graphPool.MempoolDescendantCount(spender); err == nil && n > 0 {
			total += n
		}
		if total > MaxRBFReplacementCandidates {
			return fmt.Errorf("%w (%d > %d)", ErrRBFTooManyConflicts, total, MaxRBFReplacementCandidates)
		}
	}
	return nil
}

// checkRBFNoNewUnconfirmedInputs enforces BIP125 rule 2: every unconfirmed input the replacement
// spends must also have been spent by one of the directly conflicting transactions. An input is
// treated as unconfirmed when its prevout transaction is itself present in the mempool (raw lookup
// succeeds) rather than confirmed on-chain.
func checkRBFNoNewUnconfirmedInputs(tx *wire.Tx, conflicts map[string]struct{}, pool MempoolRBFPool, view PrevOutView) error {
	conflictInputs := conflictSpentOutpoints(conflicts, pool)
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if IsNullOutpoint(in) {
			continue
		}
		parentID := txidDisplayFromLE(in.PrevHash)
		if raw, err := pool.GetRawByTxID(parentID); err != nil || len(raw) == 0 {
			continue // confirmed input (or unknown) - not covered by rule 2
		}
		key := rpcOutpointKey(parentID, in.PrevIdx)
		if _, ok := conflictInputs[key]; !ok {
			return fmt.Errorf("%w (%s)", ErrRBFNewUnconfirmedInput, key)
		}
	}
	return nil
}

// conflictSpentOutpoints collects every outpoint spent by the directly conflicting transactions.
func conflictSpentOutpoints(conflicts map[string]struct{}, pool MempoolRBFPool) map[string]struct{} {
	out := make(map[string]struct{})
	for spender := range conflicts {
		raw, err := pool.GetRawByTxID(spender)
		if err != nil || len(raw) == 0 {
			continue
		}
		old, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		for i := range old.Vin {
			in := &old.Vin[i]
			if IsNullOutpoint(in) {
				continue
			}
			out[rpcOutpointKey(txidDisplayFromLE(in.PrevHash), in.PrevIdx)] = struct{}{}
		}
	}
	return out
}

func rbfFeeMaps(pool MempoolRBFFeePackagePool, view PrevOutView) (map[string]int64, map[string]int) {
	fees := make(map[string]int64)
	if view == nil {
		return fees, nil
	}
	ids, err := pool.RawMemPoolTxIDs()
	if err != nil {
		return fees, nil
	}
	for _, id := range ids {
		raw, err := pool.GetRawByTxID(id)
		if err != nil {
			continue
		}
		old, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		fee, err := TxFee(old, view)
		if err != nil || fee < 0 {
			continue
		}
		fees[id] = fee
	}
	sizes, _ := pool.BuildMempoolSizes()
	return fees, sizes
}

// replacementPackageFeeSize sums fees/sizes for tx plus in-mempool parents (replacement package).
func replacementPackageFeeSize(tx *wire.Tx, pool MempoolRBFFeePackagePool, fees map[string]int64, sizes map[string]int) (int64, int, bool) {
	if tx == nil {
		return 0, 0, false
	}
	seed := txidDisplayFromLE(tx.TxHash())
	pkgFees := fees[seed]
	pkgSize := sizes[seed]
	if pkgSize == 0 {
		pkgSize = len(tx.SerializeForHash())
	}
	seen := map[string]struct{}{seed: {}}
	stack := make([]string, 0, 4)
	for _, in := range tx.Vin {
		if IsNullOutpoint(&in) {
			continue
		}
		pid := txidDisplayFromLE(in.PrevHash)
		if _, ok := fees[pid]; !ok {
			continue
		}
		if _, dup := seen[pid]; dup {
			continue
		}
		stack = append(stack, pid)
	}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := seen[cur]; ok {
			continue
		}
		seen[cur] = struct{}{}
		pkgFees += fees[cur]
		if sz, ok := sizes[cur]; ok {
			pkgSize += sz
		}
		raw, err := pool.GetRawByTxID(cur)
		if err != nil {
			continue
		}
		parent, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		for _, in := range parent.Vin {
			if IsNullOutpoint(&in) {
				continue
			}
			pid := txidDisplayFromLE(in.PrevHash)
			if _, ok := fees[pid]; !ok {
				continue
			}
			if _, dup := seen[pid]; !dup {
				stack = append(stack, pid)
			}
		}
	}
	if pkgSize <= 0 {
		return 0, 0, false
	}
	return pkgFees, pkgSize, true
}
