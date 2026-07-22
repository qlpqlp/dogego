// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
)

// defaultNetworksInfo is Core-shaped getnetworkinfo.networks when P2P stats are not wired.
func defaultNetworksInfo() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name": "ipv4", "limited": false, "reachable": true,
			"proxy": "", "proxy_randomize_credentials": false,
		},
		{
			"name": "ipv6", "limited": false, "reachable": true,
			"proxy": "", "proxy_randomize_credentials": false,
		},
		{
			"name": "onion", "limited": true, "reachable": false,
			"proxy": "", "proxy_randomize_credentials": false,
		},
	}
}

// execListBanned implements listbanned (Core net.cpp).
func execListBanned(paths *DataPaths) ([]interface{}, int, string) {
	if paths == nil || paths.BanManager == nil {
		return nil, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	list := paths.BanManager.ListBanned()
	out := make([]interface{}, len(list))
	for i := range list {
		out[i] = list[i]
	}
	return out, 0, ""
}

// execClearBanned implements clearbanned (Core net.cpp).
func execClearBanned(paths *DataPaths) (interface{}, int, string) {
	if paths == nil || paths.BanManager == nil {
		return nil, CodeRPCP2PDisabled, ErrP2PDisabled
	}
	paths.BanManager.ClearBanned()
	return nil, 0, ""
}

// execGetAddedNodeInfo implements getaddednodeinfo (Core net.cpp).
// Optional filter string must match an entry from DataPaths.AddedNodes when that hook is wired; otherwise Core-style error.
func execGetAddedNodeInfo(paths *DataPaths, params []json.RawMessage) ([]interface{}, int, string) {
	if len(params) > 1 {
		return nil, -8, "getaddednodeinfo: too many arguments"
	}
	nodes := []string{}
	if paths != nil && paths.AddedNodes != nil {
		nodes = paths.AddedNodes()
	}
	if len(params) == 1 {
		var raw interface{}
		if err := json.Unmarshal(params[0], &raw); err != nil {
			return nil, -8, "getaddednodeinfo: bad argument"
		}
		var filter string
		switch v := raw.(type) {
		case string:
			filter = strings.TrimSpace(v)
		case bool:
			// Some legacy CLIs passed a dummy boolean; treat as "all nodes".
			filter = ""
		default:
			return nil, -8, "getaddednodeinfo: invalid argument"
		}
		if filter != "" {
			for _, n := range nodes {
				if n == filter {
					return addedNodeInfoSlice([]string{n}, paths), 0, ""
				}
			}
			return nil, CodeRPCNodeNotAdded, ErrNodeNotAdded
		}
	}
	return addedNodeInfoSlice(nodes, paths), 0, ""
}

func addedNodeInfoSlice(nodes []string, paths *DataPaths) []interface{} {
	out := make([]interface{}, 0, len(nodes))
	for _, n := range nodes {
		connected := false
		var addresses []interface{}
		if paths != nil && paths.IsPeerConnected != nil {
			connected = paths.IsPeerConnected(n)
		}
		if paths != nil && paths.PeerAddresses != nil {
			addresses = paths.PeerAddresses(n)
		}
		if addresses == nil {
			addresses = []interface{}{}
		}
		out = append(out, map[string]interface{}{
			"addednode": n,
			"connected": connected,
			"addresses": addresses,
		})
	}
	return out
}
