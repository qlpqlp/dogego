// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// LoadFromJSONLFile replaces the in-memory UTXO set from a dumptxoutset JSON-lines file.
// tipHeight is stored as the cache tip (must match chain tip when used for gettxoutsetinfo).
func (u *UtxoCache) LoadFromJSONLFile(path string, tipHeight int64) (coinsLoaded int, err error) {
	if u == nil {
		return 0, fmt.Errorf("utxo load: nil cache")
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	u.mu.Lock()
	defer u.mu.Unlock()
	u.coins = make(map[[36]byte]UtxoEntry)
	u.tipHeight = tipHeight
	sc := bufio.NewScanner(f)
	const maxLine = 4 << 20
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, maxLine)
	for sc.Scan() {
		var row struct {
			Txid         string `json:"txid"`
			Vout         uint32 `json:"vout"`
			Value        int64  `json:"value"`
			Height       int64  `json:"height"`
			ScriptPubKey string `json:"scriptPubKey"`
		}
		if err := json.Unmarshal(sc.Bytes(), &row); err != nil {
			return coinsLoaded, fmt.Errorf("utxo load: parse line: %w", err)
		}
		var prev [32]byte
		if err := decodeDisplayTxid(row.Txid, &prev); err != nil {
			return coinsLoaded, fmt.Errorf("utxo load: txid %q: %w", row.Txid, err)
		}
		pk, err := hex.DecodeString(row.ScriptPubKey)
		if err != nil {
			return coinsLoaded, fmt.Errorf("utxo load: script %q: %w", row.Txid, err)
		}
		u.coins[outpointKey(prev, row.Vout)] = UtxoEntry{
			Value:    row.Value,
			PkScript: pk,
			Height:   row.Height,
		}
		coinsLoaded++
	}
	if err := sc.Err(); err != nil {
		return coinsLoaded, err
	}
	return coinsLoaded, nil
}
