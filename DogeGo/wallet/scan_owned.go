// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"dogego/consensus"
)

func seedOwnedFromPriorReceives(
	owned map[[36]byte]walletCoin,
	prior []ScannedTx,
	scriptSet map[string][]byte,
	addrByScript map[string]string,
) {
	if len(prior) == 0 {
		return
	}
	addrToScriptKey := make(map[string]string, len(addrByScript))
	for scriptKey, addr := range addrByScript {
		if addr != "" {
			addrToScriptKey[addr] = scriptKey
		}
	}
	for _, r := range prior {
		if r.Category != "receive" || r.BlockHeight < 0 || r.TxID == "" {
			continue
		}
		scriptKey, ok := addrToScriptKey[r.Address]
		if !ok {
			continue
		}
		pk := scriptSet[scriptKey]
		if len(pk) == 0 {
			continue
		}
		var prevHash [32]byte
		if err := consensus.DecodeDisplayTxid(r.TxID, &prevHash); err != nil {
			continue
		}
		key := walletOutpointKey(prevHash, r.Vout)
		if _, exists := owned[key]; exists {
			continue
		}
		owned[key] = walletCoin{
			value:  r.AmountKoinu,
			script: pk,
			addr:   r.Address,
		}
	}
}
