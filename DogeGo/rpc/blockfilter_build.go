// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"fmt"

	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func blockInputPrevoutScriptsRaw(blockRaw []byte, ix *store.TxIndex, raw *store.RawBlockStore) ([][]byte, error) {
	if ix == nil || raw == nil {
		return nil, fmt.Errorf("tx index required for block filter prevouts")
	}
	var scripts [][]byte
	err := wire.ForEachBlockTx(blockRaw, func(_ uint32, tx *wire.Tx) error {
		if tx == nil {
			return nil
		}
		for _, in := range tx.Vin {
			if consensus.IsNullOutpoint(&in) {
				continue
			}
			id := mempool.TxIDDisplayHex(in.PrevHash)
			_, spk, ok := store.LoadIndexedTxVout(ix, raw, id, in.PrevIdx)
			if !ok {
				return fmt.Errorf("missing prevout for %s:%d", id, in.PrevIdx)
			}
			if len(spk) == 0 {
				continue
			}
			scripts = append(scripts, append([]byte(nil), spk...))
		}
		return nil
	})
	return scripts, err
}

// BuildBasicBlockFilter constructs encoded filter + header for a stored block.
func BuildBasicBlockFilter(hashLE [32]byte, blockRaw []byte, j HeaderJournal, raw *store.RawBlockStore, ix *store.TxIndex, prevFilterHeader [32]byte) (encoded []byte, header [32]byte, err error) {
	inScripts, err := blockInputPrevoutScriptsRaw(blockRaw, ix, raw)
	if err != nil {
		return nil, [32]byte{}, err
	}
	outScripts, err := consensus.CollectBasicFilterOutputScriptsRaw(blockRaw)
	if err != nil {
		return nil, [32]byte{}, err
	}
	encoded = consensus.BuildBasicGCSFilter(hashLE, outScripts, inScripts)
	filterHash := consensus.BlockFilterHash(encoded)
	header = consensus.BlockFilterHeader(filterHash, prevFilterHeader)
	return encoded, header, nil
}

// IndexBasicBlockFilter builds and persists the basic filter for a block (idempotent overwrite).
func IndexBasicBlockFilter(f *store.BlockFilterIndex, hashLE [32]byte, blockRaw []byte, j HeaderJournal, raw *store.RawBlockStore, ix *store.TxIndex) error {
	if f == nil || ix == nil || raw == nil {
		return nil
	}
	var prevHeader [32]byte
	if j != nil {
		display := pow.LEUint256DisplayHex(hashLE[:])
		if height, err := j.HeightByDisplayHash(display); err == nil && height > 0 {
			h80, err := j.ReadHeaderAt(height - 1)
			if err == nil {
				prevHash := pow.BlockHashLE(h80)
				if _, hdr, err := f.Get(prevHash); err == nil {
					copy(prevHeader[:], hdr)
				}
			}
		}
	}
	encoded, header, err := BuildBasicBlockFilter(hashLE, blockRaw, j, raw, ix, prevHeader)
	if err != nil {
		return err
	}
	return f.Put(hashLE, encoded, header[:])
}
