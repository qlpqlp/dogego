// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// execSubmitBlock stores a block when its header is in the journal, or extends the journal by one
// header when the block builds on the current tip (no P2P relay).
func execSubmitBlock(j HeaderJournal, aux *store.HeaderAuxJournal, raw *store.RawBlockStore, paths *DataPaths, chainName string, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var hexstr string
	if err := json.Unmarshal(params[0], &hexstr); err != nil {
		return nil, -8, "submitblock: hexdata must be a string"
	}
	hexstr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexstr), "0x"))
	if len(hexstr)%2 != 0 {
		return nil, -8, "submitblock: Block decode failed"
	}
	payload, err := hex.DecodeString(hexstr)
	if err != nil {
		return nil, -8, "submitblock: Block decode failed"
	}
	if len(params) == 2 && strings.TrimSpace(string(params[1])) != "null" {
		var dummy interface{}
		if err := json.Unmarshal(params[1], &dummy); err != nil {
			return nil, -8, "submitblock: invalid optional parameters"
		}
	}
	if j == nil || raw == nil {
		return "rejected: submitblock requires full node with header journal and raw block store", 0, ""
	}
	if len(payload) < 81 {
		return "rejected: block too short", 0, ""
	}
	want := pow.BlockHashLE(payload[:80])
	if raw.Has(want) {
		return nil, 0, ""
	}
	if err := wire.ValidateBlockPayload(payload, want); err != nil {
		return "rejected: " + err.Error(), 0, ""
	}
	display := pow.BlockHashHex(payload[:80])
	height, err := j.HeightByDisplayHash(display)
	if err != nil {
		hj, ok := j.(*store.HeaderJournal)
		if !ok {
			return "rejected: block header not found in local chain (headers-first sync required)", 0, ""
		}
		net, _ := chain.ParseNetwork(chainName)
		p, perr := chain.ParamsFor(net)
		if perr != nil {
			return "rejected: " + perr.Error(), 0, ""
		}
		parentH, _, _ := activeChainFromJournal(j, raw, paths)
		height, err = consensus.ExtendHeadersFromPayload(hj, aux, p, payload, parentH, rpcNetworkNowUnix(paths))
		if err != nil {
			return "rejected: " + err.Error(), 0, ""
		}
	} else {
		h80, err := j.ReadHeaderAt(height)
		if err != nil || len(h80) != 80 {
			return "rejected: header journal read failed", 0, ""
		}
		if string(h80) != string(payload[:80]) {
			return "rejected: block header does not match header chain at height", 0, ""
		}
	}
	net, _ := chain.ParseNetwork(chainName)
	if err := consensus.CheckBlockPayload(payload, want, height, net); err != nil {
		return "rejected: " + err.Error(), 0, ""
	}
	if paths != nil && paths.ConnectSubmittedBlock != nil {
		if err := paths.ConnectSubmittedBlock(payload, height); err != nil {
			return "rejected: " + err.Error(), 0, ""
		}
	}
	if err := raw.Put(want, payload); err != nil {
		return "rejected: " + err.Error(), 0, ""
	}
	if paths != nil && paths.RelayBlock != nil {
		if err := paths.RelayBlock(payload); err != nil {
			return "rejected: relay failed: " + err.Error(), 0, ""
		}
	}
	return nil, 0, ""
}
