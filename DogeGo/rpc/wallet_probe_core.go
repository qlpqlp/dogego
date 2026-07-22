// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/chain"
	"dogego/wallet/corewallet"
)

// execProbeWalletDat inspects a Core wallet.dat without importing (dogego_probewalletdat).
// When a built-in HD wallet is wired, the response also includes hd_keypool_core_index
// and pool_core_indices_stored from the live wallet.json (not from the probed file).
func execProbeWalletDat(chainName string, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var filename string
	if err := json.Unmarshal(params[0], &filename); err != nil {
		return nil, -8, "dogego_probewalletdat: filename must be a string"
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, -8, "dogego_probewalletdat: invalid filename"
	}
	filename, err := ValidateFilePath(dataPathRoots(paths), filename, false)
	if err != nil {
		return nil, -8, "dogego_probewalletdat: "+err.Error()
	}
	net, err := chain.ParseNetwork(chainName)
	if err != nil {
		return nil, -8, "dogego_probewalletdat: "+err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -1, err.Error()
	}
	probe, err := corewallet.ProbeWalletDat(filename, p.PrivKeyWIFVersion)
	if err != nil {
		return nil, -8, "dogego_probewalletdat: "+err.Error()
	}
	out, err := probeWalletDatResultMap(probe, paths)
	if err != nil {
		return nil, -1, err.Error()
	}
	return out, 0, ""
}

func probeWalletDatResultMap(probe *corewallet.ProbeResult, paths *DataPaths) (map[string]interface{}, error) {
	b, err := json.Marshal(probe)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	mergeWalletHDKeypoolCoreIndex(out, paths)
	return out, nil
}

func mergeWalletHDKeypoolCoreIndex(m map[string]interface{}, paths *DataPaths) {
	if m == nil || paths == nil || paths.WalletHDKeypoolCoreIndex == nil {
		return
	}
	entries := paths.WalletHDKeypoolCoreIndex()
	if len(entries) == 0 {
		return
	}
	rows := make([]map[string]interface{}, len(entries))
	for i, e := range entries {
		rows[i] = map[string]interface{}{
			"receive_index": e.ReceiveIndex,
			"core_index":    e.CoreIndex,
		}
	}
	m["hd_keypool_core_index"] = rows
	m["pool_core_indices_stored"] = len(entries)
}
