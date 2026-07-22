// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"strings"

	"dogego/applog"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// InvalidateBlock disconnects the chain at the block before hash (Core invalidateblock).
func InvalidateBlock(j *store.HeaderJournal, aux *store.HeaderAuxJournal, policy *store.ChainPolicy, bs *BlockStoreCtx, displayHex string) error {
	if j == nil {
		return fmt.Errorf("header journal not available")
	}
	displayHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(displayHex), "0x"))
	height, err := j.HeightByDisplayHash(displayHex)
	if err != nil {
		return fmt.Errorf("block not found")
	}
	if height == 0 {
		return fmt.Errorf("cannot invalidate the genesis block")
	}
	tip, err := j.TipHeight()
	if err != nil {
		return err
	}
	for h := height; h <= tip; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return err
		}
		if policy != nil {
			if err := policy.AddInvalid(pow.BlockHashHex(h80)); err != nil {
				return err
			}
		}
	}
	keep := height - 1
	applog.Line("chain", fmt.Sprintf("invalidateblock: truncating to height %d (invalidated %s…)", keep, displayHex[:12]))
	if err := TruncateChainToHeight(j, aux, bs, keep); err != nil {
		return err
	}
	return nil
}

// ReconsiderBlock removes a block from the invalid set so it may be accepted again (Core reconsiderblock).
func ReconsiderBlock(policy *store.ChainPolicy, displayHex string) error {
	if policy == nil {
		return fmt.Errorf("chain policy not available")
	}
	displayHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(displayHex), "0x"))
	if len(displayHex) != 64 {
		return fmt.Errorf("block hash must be 64 hex characters")
	}
	if !policy.IsInvalid(displayHex) {
		return fmt.Errorf("block not found")
	}
	return policy.RemoveInvalid(displayHex)
}

// MarkPreciousBlock marks a block as preferred in equal-work reorgs (Core preciousblock).
func MarkPreciousBlock(j *store.HeaderJournal, policy *store.ChainPolicy, displayHex string) error {
	if j == nil {
		return fmt.Errorf("header journal not available")
	}
	displayHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(displayHex), "0x"))
	if _, err := j.HeightByDisplayHash(displayHex); err != nil {
		return fmt.Errorf("block not found")
	}
	if policy == nil {
		return fmt.Errorf("chain policy not available")
	}
	if err := policy.SetPrecious(displayHex); err != nil {
		return err
	}
	applog.Line("chain", fmt.Sprintf("preciousblock: marked %s…", displayHex[:12]))
	return nil
}

// HeadersBatchContainsHash reports whether any decoded header matches display hash.
func HeadersBatchContainsHash(decoded []wire.DecodedHeader, displayHex string) bool {
	displayHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(displayHex), "0x"))
	for _, d := range decoded {
		if strings.EqualFold(pow.BlockHashHex(d.Header80), displayHex) {
			return true
		}
	}
	return false
}
