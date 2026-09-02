// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "strings"

// penalizeBlockPeer records a block/header fetch failure on the block peer scorer and addrbook (Core disconnect + addrman cooldown).
func penalizeBlockPeer(scorer *BlockPeerScorer, book *AddrBook, addr string, hard bool) {
	if addr == "" {
		return
	}
	if scorer != nil {
		scorer.NoteSessionFailure(addr, hard)
	}
	if book != nil && hard {
		book.NoteFailure(addr)
	}
}

// penalizeWrongNetworkPeer applies a long quarantine after bad P2P magic / wire desync.
func penalizeWrongNetworkPeer(scorer *BlockPeerScorer, book *AddrBook, addr string, err error) {
	if addr == "" || err == nil || !strings.Contains(err.Error(), "bad magic") {
		penalizeBlockPeer(scorer, book, addr, true)
		return
	}
	if scorer != nil {
		scorer.NoteWrongNetworkMagic(addr)
	}
	if book != nil {
		book.NoteFailure(addr)
	}
}

// penalizeStubBlockPeer applies a long cooldown when a peer returns undersized block stubs.
func penalizeStubBlockPeer(scorer *BlockPeerScorer, book *AddrBook, addr string) {
	if addr == "" {
		return
	}
	if scorer != nil {
		scorer.NoteStubBlock(addr)
	}
	if book != nil {
		book.NoteFailure(addr)
	}
}

// addrBookFromPeerMgr returns the learned address book when multi-peer P2P is active.
func addrBookFromPeerMgr(pm *PeerMgr) *AddrBook {
	if pm == nil {
		return nil
	}
	pm.mu.Lock()
	book := pm.addrs
	pm.mu.Unlock()
	return book
}

// activeAddrBook returns the peer manager book when wired, else the startup bootstrap book.
func activeAddrBook(pm *PeerMgr, bootstrap *AddrBook) *AddrBook {
	if book := addrBookFromPeerMgr(pm); book != nil {
		return book
	}
	return bootstrap
}
