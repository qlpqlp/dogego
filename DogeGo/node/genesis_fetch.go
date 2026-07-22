// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"strings"
)

// ErrGenesisPeerNotFound means the connected peer returned notfound for the genesis block getdata.
var ErrGenesisPeerNotFound = errors.New("genesis block notfound from peer (likely pruned; need archival NODE_NETWORK peer)")

func isGenesisNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrGenesisPeerNotFound) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "notfound") && strings.Contains(s, "genesis")
}

// shouldRedialPrimaryForAncientFetch reports whether the primary session should rotate after a fetch
// failure on very low heights (genesis / early IBD) where pruned peers cannot serve blocks, or after
// undersized block stubs at any height.
func shouldRedialPrimaryForAncientFetch(err error, wantHeight int64) bool {
	if err == nil || wantHeight < 0 {
		return false
	}
	if shouldRotatePeerForStubBlock(err) {
		return true
	}
	if wantHeight > 512 {
		return false
	}
	if isGenesisNotFoundErr(err) {
		return true
	}
	if !sessionFailureHardFromFetchErr(err) {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "notfound") || strings.Contains(s, "batch incomplete")
}
