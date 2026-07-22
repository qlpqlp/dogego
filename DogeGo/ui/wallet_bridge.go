// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "sync"

// WalletSendBridge is wired after the node has mempool + UTXO paths (may be set after the web UI starts).
type WalletSendBridge struct {
	mu sync.RWMutex
	fn func(dest string, amountDOGE float64, fundOpts map[string]interface{}) (txid string, errCode int, errMsg string)
	detailedFn func(dest string, amountDOGE float64, fundOpts map[string]interface{}) (WalletSendDetailed, int, string)
}

// WalletSendDetailed is the web send outcome including raw hex for rebroadcast.
type WalletSendDetailed struct {
	Txid           string
	Hex            string
	Status         string // broadcasting | mempool | confirmed
	BroadcastError string
}

// Set installs the send callback (safe to call from the node after P2P connect).
func (b *WalletSendBridge) Set(fn func(dest string, amountDOGE float64, fundOpts map[string]interface{}) (txid string, errCode int, errMsg string)) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.fn = fn
	b.mu.Unlock()
}

// SetDetailed installs the send callback that returns signed hex for flight tracking.
func (b *WalletSendBridge) SetDetailed(fn func(dest string, amountDOGE float64, fundOpts map[string]interface{}) (WalletSendDetailed, int, string)) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.detailedFn = fn
	b.mu.Unlock()
}

// Call runs sendtoaddress when wired.
func (b *WalletSendBridge) Call(dest string, amountDOGE float64, fundOpts map[string]interface{}) (txid string, errCode int, errMsg string) {
	if b == nil {
		return "", -1, "wallet send is not available yet"
	}
	b.mu.RLock()
	fn := b.fn
	b.mu.RUnlock()
	if fn == nil {
		return "", -1, "wallet send is not available yet (wait for node sync / RPC wiring)"
	}
	return fn(dest, amountDOGE, fundOpts)
}

// CallDetailed signs, broadcasts, and returns tx metadata for the web flight bar.
func (b *WalletSendBridge) CallDetailed(dest string, amountDOGE float64, fundOpts map[string]interface{}) (WalletSendDetailed, int, string) {
	if b == nil {
		return WalletSendDetailed{}, -1, "wallet send is not available yet"
	}
	b.mu.RLock()
	fn := b.detailedFn
	b.mu.RUnlock()
	if fn == nil {
		txid, code, msg := b.Call(dest, amountDOGE, fundOpts)
		if code != 0 {
			return WalletSendDetailed{}, code, msg
		}
		return WalletSendDetailed{Txid: txid, Status: "broadcasting"}, 0, ""
	}
	return fn(dest, amountDOGE, fundOpts)
}

// WalletTxsBridge supplies wallet transaction rows for GET /api/wallet/txs.
type WalletTxsBridge struct {
	mu     sync.RWMutex
	fn     func() []interface{}
	pageFn func(offset, limit int, q, kind string) WalletTxPageResult
}

// WalletTxPageResult is a paginated wallet history response for the web UI.
type WalletTxPageResult struct {
	Total  int
	Offset int
	Limit  int
	Items  []interface{}
}

// Set installs the list callback.
func (b *WalletTxsBridge) Set(fn func() []interface{}) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.fn = fn
	b.mu.Unlock()
}

// SetPage installs the paginated list callback.
func (b *WalletTxsBridge) SetPage(fn func(offset, limit int, q, kind string) WalletTxPageResult) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.pageFn = fn
	b.mu.Unlock()
}

// List returns wallet transactions when wired.
func (b *WalletTxsBridge) List() []interface{} {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	fn := b.fn
	b.mu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn()
}

// ListPage returns a filtered, paginated wallet history page when wired.
func (b *WalletTxsBridge) ListPage(offset, limit int, q, kind string) (WalletTxPageResult, bool) {
	if b == nil {
		return WalletTxPageResult{}, false
	}
	b.mu.RLock()
	fn := b.pageFn
	b.mu.RUnlock()
	if fn == nil {
		return WalletTxPageResult{}, false
	}
	return fn(offset, limit, q, kind), true
}
