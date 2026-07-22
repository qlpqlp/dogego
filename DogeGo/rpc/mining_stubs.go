// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/mempool"
	"dogego/store"
)

func parseOptionalMaxTriesAuxPow(params []json.RawMessage, maxIdx, auxIdx int, method string) (int, string) {
	if len(params) > maxIdx && strings.TrimSpace(string(params[maxIdx])) != "null" {
		var maxTries float64
		if err := json.Unmarshal(params[maxIdx], &maxTries); err != nil {
			return -8, method + ": invalid maxtries"
		}
		if maxTries < 0 || maxTries != float64(int64(maxTries)) {
			return -8, method + ": invalid maxtries"
		}
	}
	if len(params) > auxIdx && strings.TrimSpace(string(params[auxIdx])) != "null" {
		var aux float64
		if err := json.Unmarshal(params[auxIdx], &aux); err != nil {
			return -8, method + ": invalid auxpow"
		}
		if aux != float64(int64(aux)) {
			return -8, method + ": invalid auxpow"
		}
	}
	return 0, ""
}

// execGenerate mines to MiningAddress (Core wallet coinbase); delegates to generatetoaddress.
func execGenerate(
	j HeaderJournal,
	aux *store.HeaderAuxJournal,
	paths *DataPaths,
	raw *store.RawBlockStore,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	chainName string,
	params []json.RawMessage,
) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	var nblocks float64
	if err := json.Unmarshal(params[0], &nblocks); err != nil {
		return nil, -8, "generate: invalid nblocks"
	}
	if nblocks < 1 || nblocks != float64(int64(nblocks)) {
		return nil, -8, "generate: nblocks must be a positive integer"
	}
	if code, msg := parseOptionalMaxTriesAuxPow(params, 1, 2, "generate"); code != 0 {
		return nil, code, msg
	}
	addr := ""
	if paths != nil {
		addr = strings.TrimSpace(paths.MiningAddress)
	}
	if addr == "" {
		return nil, -1, "generate: no mining address configured (enable testnet wallet or set miningaddress in dogecoinconf.json)"
	}
	addrJSON, _ := json.Marshal(addr)
	genParams := []json.RawMessage{params[0], json.RawMessage(addrJSON)}
	if len(params) > 1 {
		genParams = append(genParams, params[1])
	}
	if len(params) > 2 {
		genParams = append(genParams, params[2])
	}
	return execGenerateToAddress(j, aux, paths, raw, pool, txIndex, chainName, genParams)
}
