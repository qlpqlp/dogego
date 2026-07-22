// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"

	"dogego/applog"
	"dogego/chain"
	"dogego/store"
)

// Mainnet header IBD often stalled at ~510000 on builds that rejected valid aux parent chain IDs.
const (
	postAuxEraStallTipMainnetMin int64 = 509_500
	postAuxEraStallTipMainnetMax int64 = 510_500
)

func isPostAuxEraStallTipMainnet(net chain.Network, tip int64) bool {
	return net == chain.MainnetDogecoin &&
		tip >= postAuxEraStallTipMainnetMin &&
		tip <= postAuxEraStallTipMainnetMax
}

func postAuxEraStallRecoveryHint() string {
	return "Header tip stuck near height 510000 (~8% on mainnet). If logs mention aux parent chain id must be zero, rebuild and restart with the current dogego.exe (no header wipe required). Otherwise wait for background header sync or run dogego_recoverheaders."
}

// maybeNotePostAuxEraHeaderStall sets an operator hint when the journal tip stops advancing in the
// known post-aux era stall band (obsolete binary or peer rotation needed).
func maybeNotePostAuxEraHeaderStall(net chain.Network, tip int64) {
	if !isPostAuxEraStallTipMainnet(net, tip) {
		return
	}
	if headerSyncRecoveryHintStr() != "" {
		return
	}
	setHeaderSyncRecoveryHint(postAuxEraStallRecoveryHint())
}

// maybeKickPostAuxEraHeaderRecovery on startup (or after rewind) when the journal tip is in the
// known post-aux stall band but the network is still far ahead - common after upgrading from builds
// that rejected valid aux parent chain IDs at ~510k.
func maybeKickPostAuxEraHeaderRecovery(net chain.Network, j *store.HeaderJournal, kick func(force bool) bool) {
	if j == nil || kick == nil {
		return
	}
	tip, err := j.TipHeight()
	if err != nil || !isPostAuxEraStallTipMainnet(net, tip) {
		return
	}
	if !shouldContinueHeaderCatchUpDuringIBD(j, 0) {
		return
	}
	maybeNotePostAuxEraHeaderStall(net, tip)
	if kick(false) {
		applog.Line("headers", fmt.Sprintf("post-aux era tip at height %d - header catch-up prioritized", tip))
	}
}

// maybeClearPostAuxEraStallHint removes the generic post-aux stall hint once the journal passes the band.
func maybeClearPostAuxEraStallHint(net chain.Network, tip int64) {
	if net != chain.MainnetDogecoin || tip <= postAuxEraStallTipMainnetMax {
		return
	}
	if headerSyncRecoveryHintStr() == postAuxEraStallRecoveryHint() {
		clearHeaderSyncRecoveryHint()
	}
}

