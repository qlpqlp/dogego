// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/config"
)

// LoadAddedNodesFromConfig seeds the in-memory addnode list from dogecoinconf.json.
func LoadAddedNodesFromConfig(store *AddedNodeStore, nodes []string, defaultPort int) {
	if store == nil {
		return
	}
	for _, n := range nodes {
		norm, err := NormalizeNodeAddr(n, defaultPort)
		if err != nil {
			continue
		}
		store.Add(norm)
	}
}

// PersistAddedNodes writes the current addnode list back to dogecoinconf.json.
func PersistAddedNodes(savePath string, eff *config.File, store *AddedNodeStore) error {
	if savePath == "" || eff == nil || store == nil {
		return nil
	}
	eff.AddedNodes = store.List()
	return config.Save(savePath, *eff)
}
