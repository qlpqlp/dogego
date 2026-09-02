// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"sync/atomic"

	"dogego/store"
	"dogego/wallet"
)

// LateRefs lets the node bind chain stores after the dashboard is already listening
// (conf check → open browser → then load headers/rawblocks/indexes/wallet).
type LateRefs struct {
	Journal   atomic.Pointer[store.HeaderJournal]
	RawBlocks atomic.Pointer[store.RawBlockStore]
	TxIndex   atomic.Pointer[store.TxIndex]
	AddrIndex atomic.Pointer[store.AddrIndex]
	Wallet    atomic.Pointer[wallet.Disk]
}

// ActiveJournal returns the live header journal (late-bound or StartConfig.Journal).
func (cfg StartConfig) ActiveJournal() *store.HeaderJournal {
	if cfg.Late != nil {
		if j := cfg.Late.Journal.Load(); j != nil {
			return j
		}
	}
	return cfg.Journal
}

// ActiveRawBlocks returns the live raw block store (late-bound or StartConfig.RawBlocks).
func (cfg StartConfig) ActiveRawBlocks() *store.RawBlockStore {
	if cfg.Late != nil {
		if r := cfg.Late.RawBlocks.Load(); r != nil {
			return r
		}
	}
	return cfg.RawBlocks
}

// ActiveTxIndex returns the live tx index (late-bound or StartConfig.TxIndex).
func (cfg StartConfig) ActiveTxIndex() *store.TxIndex {
	if cfg.Late != nil {
		if ix := cfg.Late.TxIndex.Load(); ix != nil {
			return ix
		}
	}
	return cfg.TxIndex
}

// ActiveAddrIndex returns the live address index (late-bound or StartConfig.AddrIndex).
func (cfg StartConfig) ActiveAddrIndex() *store.AddrIndex {
	if cfg.Late != nil {
		if ix := cfg.Late.AddrIndex.Load(); ix != nil {
			return ix
		}
	}
	return cfg.AddrIndex
}

// ActiveWallet returns the live wallet disk (late-bound or StartConfig.Wallet).
func (cfg StartConfig) ActiveWallet() *wallet.Disk {
	if cfg.Late != nil {
		if w := cfg.Late.Wallet.Load(); w != nil {
			return w
		}
	}
	return cfg.Wallet
}
