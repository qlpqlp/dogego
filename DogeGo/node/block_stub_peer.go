// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import "strings"

// shouldRotatePeerForStubBlock reports whether the block-fetch peer should disconnect after
// delivering undersized payloads that fail the coarse size floor (<140 B on ancient mainnet)
// or pruned-peer stubs that fail consensus after download.
func shouldRotatePeerForStubBlock(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "too short") || strings.Contains(s, "undersized stub")
}
