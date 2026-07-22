// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"
	"strings"
	"sync"

)

// AssumeValidTipWindow is how many blocks below the header tip still get full script checks
// when at or below the assume-valid height (Core uses ~2 weeks of work-equivalent time).
const AssumeValidTipWindow int64 = 20160

// DefaultAssumeValidHex returns Core chainparams defaultAssumeValid for the network ("" = verify all).
func DefaultAssumeValidHex(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "mainnet", "":
		// Dogecoin Core chainparams.cpp - height 5,050,000
		return "e7d4577405223918491477db725a393bcfc349d8ee63b0a4fde23cbfbfd81dea"
	case "testnet":
		return ""
	default:
		return ""
	}
}

// AssumeValid tracks Core -assumevalid: skip ECDSA/script verification on buried blocks in the best chain.
type AssumeValid struct {
	mu         sync.RWMutex
	hashHex    string
	height     int64 // resolved on best chain; -1 = unknown or verify all
	forceAll   bool  // user set 0 / empty disable
	headerTip  int64 // updated by node for tip-window checks
}

// NewAssumeValid builds policy from user hex (empty = network default; "0" = verify all scripts).
func NewAssumeValid(network, userHex string) *AssumeValid {
	a := &AssumeValid{height: -1, headerTip: -1}
	userHex = strings.TrimSpace(strings.ToLower(userHex))
	if userHex == "0" {
		a.forceAll = true
		return a
	}
	if userHex == "" {
		userHex = strings.TrimSpace(strings.ToLower(DefaultAssumeValidHex(network)))
	}
	if userHex == "" || userHex == "0" {
		a.forceAll = true
		return a
	}
	a.hashHex = userHex
	return a
}

// HashHex returns the configured assume-valid block hash (display / RPC order).
func (a *AssumeValid) HashHex() string {
	if a == nil {
		return ""
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.hashHex
}

// Height returns the resolved height on the header chain (-1 if unset).
func (a *AssumeValid) Height() int64 {
	if a == nil {
		return -1
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.height
}

// Resolved reports whether the assume-valid hash was found on the local header chain.
func (a *AssumeValid) Resolved() bool {
	return a != nil && a.Height() >= 0
}

// SetHeaderTip updates the journal tip for Core-style tip-window script checks.
func (a *AssumeValid) SetHeaderTip(tip int64) {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.headerTip = tip
	a.mu.Unlock()
}

// PinResolvedHeight records assume-valid height without a journal lookup (used when the node already resolved the hash).
func (a *AssumeValid) PinResolvedHeight(height int64) {
	if a == nil || a.forceAll || height < 0 {
		return
	}
	a.mu.Lock()
	a.height = height
	a.mu.Unlock()
}

// ClearResolution drops a cached assume-valid height (e.g. after header chain rewind).
func (a *AssumeValid) ClearResolution() {
	if a == nil || a.forceAll {
		return
	}
	a.mu.Lock()
	a.height = -1
	a.mu.Unlock()
}

// TryResolve resolves assume-valid on the header journal when not yet found; reports newly resolved.
func (a *AssumeValid) TryResolve(journal HeaderChain) (justResolved bool) {
	if a == nil || journal == nil || a.forceAll || a.hashHex == "" || a.Resolved() {
		return false
	}
	if err := a.Resolve(journal); err != nil {
		return false
	}
	return a.Resolved()
}

// Resolve finds assume-valid height on the header journal (best chain only).
func (a *AssumeValid) Resolve(journal HeaderChain) error {
	if a == nil || journal == nil || a.forceAll || a.hashHex == "" {
		return nil
	}
	h, err := journal.HeightByDisplayHash(a.hashHex)
	if err != nil {
		a.mu.Lock()
		a.height = -1
		a.mu.Unlock()
		return fmt.Errorf("assumevalid block %s… not in header chain: %w", a.hashHex[:12], err)
	}
	a.mu.Lock()
	a.height = h
	a.mu.Unlock()
	return nil
}

// ScriptChecksEnabled reports whether ConnectBlock should run ECDSA/script verification at blockHeight.
func (a *AssumeValid) ScriptChecksEnabled(blockHeight int64) bool {
	if a == nil || a.forceAll || a.hashHex == "" {
		return true
	}
	a.mu.RLock()
	avH := a.height
	tip := a.headerTip
	a.mu.RUnlock()
	if avH < 0 {
		return true
	}
	if blockHeight > avH {
		return true
	}
	if tip >= 0 && tip-blockHeight < AssumeValidTipWindow {
		return true
	}
	return false
}

// globalAssumeValid is set by the node during Run (nil = verify all).
var globalAssumeValid *AssumeValid

// SetGlobalAssumeValid installs active assume-valid policy for ConnectBlock.
func SetGlobalAssumeValid(a *AssumeValid) {
	globalAssumeValid = a
}

// ScriptChecksEnabledAtHeight uses the global assume-valid policy.
func ScriptChecksEnabledAtHeight(blockHeight int64) bool {
	if globalAssumeValid == nil {
		return true
	}
	return globalAssumeValid.ScriptChecksEnabled(blockHeight)
}

// WithFullScriptVerification runs fn with all script checks enabled (verifychain / reindex).
func WithFullScriptVerification(fn func() error) error {
	prev := globalAssumeValid
	globalAssumeValid = NewAssumeValid("mainnet", "0")
	defer func() { globalAssumeValid = prev }()
	return fn()
}

// GlobalAssumeValidSummary returns Core-shaped RPC fields for getblockchaininfo.
func GlobalAssumeValidSummary() map[string]interface{} {
	if globalAssumeValid == nil {
		return nil
	}
	out := map[string]interface{}{
		"dogego_assumevalid": globalAssumeValid.HashHex(),
	}
	if h := globalAssumeValid.Height(); h >= 0 {
		out["dogego_assumevalid_height"] = h
	}
	return out
}

