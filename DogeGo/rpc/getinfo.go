// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/store"
)

// execGetInfo implements the deprecated Core aggregate RPC (sync fields mirror getblockchaininfo; limited wallet hints).
func execGetInfo(chainName string, j HeaderJournal, raw *store.RawBlockStore, paths *DataPaths) (map[string]interface{}, int, string) {
	sync := computeChainIBDState(j, chainName, raw, paths)
	blocks := sync.blocks
	diff, err := headerDifficultyAt(j, blocks)
	if err != nil {
		return nil, -1, err.Error()
	}
	proto := chain.ProtocolVersion
	if paths != nil && paths.LocalP2P != nil {
		p, _, _ := paths.LocalP2P()
		proto = p
	}
	cn := strings.ToLower(strings.TrimSpace(chainName))
	testnet := cn == "test" || cn == "testnet"
	relay := 0.0
	if paths != nil && paths.FeeFilter != nil {
		relay = float64(paths.FeeFilter()) / 1e8
	}
	conns := 1
	if paths != nil && paths.NetworkActive != nil && !paths.NetworkActive() {
		conns = 0
	}
	errStr := ""
	net := networkFromChainName(chainName)
	if warns := consensus.ChainWarnings(j, net); len(warns) > 0 {
		errStr = strings.Join(warns, "; ")
	}
	out := map[string]interface{}{
		"version":              1140900,
		"protocolversion":      proto,
		"blocks":               blocks,
		"headers":              sync.headers,
		"timeoffset":           medianPeerTimeOffset(paths),
		"connections":          conns,
		"proxy":                "",
		"difficulty":           diff,
		"testnet":              testnet,
		"relayfee":             relay,
		"initialblockdownload": sync.ibd,
		"verificationprogress": sync.verProg,
		"errors":               errStr,
		"dogego_wallet_active": WalletActive(paths),
		"dogego_note":          "deprecated Core aggregate; sync fields mirror getblockchaininfo; use wallet RPC when dogego_wallet_active",
	}
	if paths != nil && paths.WalletDefaultAddress != nil {
		if a := strings.TrimSpace(paths.WalletDefaultAddress()); a != "" {
			out["dogego_wallet_address"] = a
		}
	}
	return out, 0, ""
}
