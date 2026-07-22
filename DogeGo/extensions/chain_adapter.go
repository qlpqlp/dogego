// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"encoding/hex"
	"fmt"
	"strings"

	"dogego/pow"
	"dogego/store"
)

// ChainAdapter exposes read-only chain data to extensions.
type ChainAdapter struct {
	NetworkName string
	Journal     *store.HeaderJournal
	Raw         *store.RawBlockStore
	TxIndex     *store.TxIndex
	UtxoTip     func() int64
}

func (c *ChainAdapter) Network() string {
	if c == nil {
		return ""
	}
	return c.NetworkName
}

func (c *ChainAdapter) TipHeight() (int64, error) {
	if c == nil || c.Journal == nil {
		return -1, nil
	}
	if c.UtxoTip != nil {
		if h := c.UtxoTip(); h >= 0 {
			return h, nil
		}
	}
	return c.Journal.TipHeight()
}

func (c *ChainAdapter) GetRawBlockByHeight(height int64) ([]byte, error) {
	if c == nil || c.Journal == nil || c.Raw == nil {
		return nil, nil
	}
	h80, err := c.Journal.ReadHeaderAt(height)
	if err != nil {
		return nil, err
	}
	id := pow.BlockHashLE(h80)
	return c.Raw.Get(id)
}

func (c *ChainAdapter) LookupTxHex(txid string) (txHex string, height int64, ok bool) {
	if c == nil || c.TxIndex == nil || c.Journal == nil {
		return "", 0, false
	}
	tx, err := store.LoadIndexedTx(c.TxIndex, c.Raw, txid)
	if err != nil || tx == nil {
		return "", 0, false
	}
	raw, err := tx.Serialize()
	if err != nil {
		return "", 0, false
	}
	blockHash, _, err := c.TxIndex.Lookup(txid)
	if err != nil {
		return "", 0, false
	}
	blockHex := pow.LEUint256DisplayHex(blockHash[:])
	height, err = c.Journal.HeightByDisplayHash(blockHex)
	if err != nil {
		return "", 0, false
	}
	return hex.EncodeToString(raw), height, true
}

func (c *ChainAdapter) BlockHashAtHeight(height int64) (string, error) {
	if c == nil || c.Journal == nil {
		return "", fmt.Errorf("journal unwired")
	}
	h80, err := c.Journal.ReadHeaderAt(height)
	if err != nil {
		return "", err
	}
	id := pow.BlockHashLE(h80)
	return pow.LEUint256DisplayHex(id[:]), nil
}

func (c *ChainAdapter) ConfirmedTxInBlock(blockHash, txid string) (uint32, bool) {
	if c == nil || c.TxIndex == nil {
		return 0, false
	}
	bh, _, err := c.TxIndex.Lookup(txid)
	if err != nil {
		return 0, false
	}
	if !strings.EqualFold(pow.LEUint256DisplayHex(bh[:]), strings.TrimSpace(blockHash)) {
		return 0, false
	}
	_, idx, err := c.TxIndex.Lookup(txid)
	if err != nil {
		return 0, false
	}
	return idx, true
}
