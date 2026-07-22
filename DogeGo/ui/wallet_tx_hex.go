// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/hex"
	"strings"

	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// walletTxHexCached returns hex from wallet.db only (no tx-index / block walk).
func walletTxHexCached(cfg StartConfig, txid string) string {
	txid = strings.ToLower(strings.TrimSpace(txid))
	if txid == "" || cfg.Wallet == nil {
		return ""
	}
	if hx, ok := cfg.Wallet.LookupTxHex(txid); ok && hx != "" {
		return hx
	}
	return ""
}

// walletTxHexForUI returns serialized tx hex for wallet history rows (wallet.db, mempool, tx index, or block at height).
// When hex is loaded from chain data it is persisted into wallet.db for later gettransaction fast paths.
func walletTxHexForUI(cfg StartConfig, txid string, blockHeight int64) string {
	txid = strings.ToLower(strings.TrimSpace(txid))
	if txid == "" {
		return ""
	}
	if cfg.Wallet != nil {
		if hx, ok := cfg.Wallet.LookupTxHex(txid); ok && hx != "" {
			return hx
		}
	}
	if cfg.Pool != nil {
		if b, err := cfg.Pool.GetRawByTxID(txid); err == nil && len(b) > 0 {
			hx := hex.EncodeToString(b)
			walletRememberTxHex(cfg, txid, hx)
			return hx
		}
	}
	if cfg.TxIndex != nil && cfg.RawBlocks != nil {
		if tx, err := store.LoadIndexedTx(cfg.TxIndex, cfg.RawBlocks, txid); err == nil {
			if ser, err := tx.Serialize(); err == nil {
				hx := hex.EncodeToString(ser)
				walletRememberTxHex(cfg, txid, hx)
				return hx
			}
		}
	}
	if cfg.Journal != nil && cfg.RawBlocks != nil && blockHeight >= 0 {
		if hx := walletTxHexFromBlockHeight(cfg.Journal, cfg.RawBlocks, txid, blockHeight); hx != "" {
			walletRememberTxHex(cfg, txid, hx)
			return hx
		}
	}
	return ""
}

func walletRememberTxHex(cfg StartConfig, txid, hx string) {
	if cfg.Wallet == nil || hx == "" {
		return
	}
	_ = cfg.Wallet.RememberTxHex(txid, hx)
}

func walletTxHexFromBlockHeight(j *store.HeaderJournal, raw *store.RawBlockStore, txid string, height int64) string {
	h80, err := j.ReadHeaderAt(height)
	if err != nil {
		return ""
	}
	id := pow.BlockHashLE(h80)
	payload, err := raw.Get(id)
	if err != nil {
		return ""
	}
	want := strings.ToLower(txid)
	var hexOut string
	_ = wire.ForEachBlockTx(payload, func(_ uint32, tx *wire.Tx) error {
		if hexOut != "" {
			return nil
		}
		if strings.EqualFold(mempool.TxIDDisplayHex(tx.TxHash()), want) {
			if ser, err := tx.Serialize(); err == nil {
				hexOut = hex.EncodeToString(ser)
			}
		}
		return nil
	})
	return hexOut
}
