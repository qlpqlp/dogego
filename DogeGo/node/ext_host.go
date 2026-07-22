// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package node

import (
	"fmt"
	"strings"

	"dogego/applog"
	"dogego/chain"
	"dogego/extensions"
	"dogego/rpc"
	"dogego/store"
)

func extensionNetworkSlug(cfg Config) string {
	if cfg.Network == chain.MainnetDogecoin {
		return "mainnet"
	}
	return "testnet"
}

func chainRPCPathsNeeded(cfg Config) bool {
	return cfg.NodeMode == "full" ||
		strings.TrimSpace(cfg.RPCAddr) != "" ||
		strings.TrimSpace(cfg.WebUIAddr) != ""
}

func ensureExtensionManager(
	cfg Config,
	extMgr **extensions.Manager,
	chainDataDir string,
	j *store.HeaderJournal,
	rb *store.RawBlockStore,
	txIx *store.TxIndex,
	utxo *store.UtxoCache,
) *extensions.Manager {
	if extMgr == nil || j == nil {
		return nil
	}
	if *extMgr != nil {
		return *extMgr
	}
	extChain := &extensions.ChainAdapter{
		NetworkName: extensionNetworkSlug(cfg),
		Journal:     j,
		Raw:         rb,
		TxIndex:     txIx,
		UtxoTip: func() int64 {
			if utxo != nil {
				return utxo.TipHeight()
			}
			return -1
		},
	}
	m := extensions.NewManager(chainDataDir, extensionNetworkSlug(cfg), extChain)
	if err := m.Load(); err != nil {
		applog.Line("extensions", "load: "+err.Error())
	}
	rpc.SetExtensionRPCCatalog(func() []string {
		if m == nil {
			return nil
		}
		return m.CatalogRPCMethods()
	})
	*extMgr = m
	rpc.SetExtensionsHost(m)
	applog.Line("extensions", fmt.Sprintf("extension host ready (%s)", extensionNetworkSlug(cfg)))
	return m
}
