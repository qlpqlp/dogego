// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "dogego/node/dgr"

type relayAddrBookView struct {
	book *AddrBook
}

func (v relayAddrBookView) NoteTry(addr string) {
	if v.book != nil {
		v.book.NoteRelayTry(addr)
	}
}

func (v relayAddrBookView) NoteSuccess(addr string) {
	if v.book != nil {
		v.book.NoteRelaySuccess(addr)
	}
}

func (v relayAddrBookView) NoteFailure(addr string) {
	if v.book != nil {
		v.book.NoteRelayFailure(addr)
	}
}

func (v relayAddrBookView) RelayDialScore(addr string) int {
	if v.book == nil {
		return -1 << 30
	}
	return v.book.RelayDialScore(addr)
}

func peerMgrRelayBook(peerMgr **PeerMgr) dgr.RelayAddrBook {
	if peerMgr == nil || *peerMgr == nil {
		return nil
	}
	book := (*peerMgr).RelayAddrBook()
	if book == nil {
		return nil
	}
	return relayAddrBookView{book: book}
}
