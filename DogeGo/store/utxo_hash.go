// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"

	"dogego/pow"
	"dogego/wire"
)

func hashSerializedHex(h [32]byte) string {
	for i := 0; i < 16; i++ {
		h[i], h[31-i] = h[31-i], h[i]
	}
	return hex.EncodeToString(h[:])
}

// hashUTXOSetLE implements Dogecoin Core GetUTXOStats hash_serialized (blockchain.cpp).
func (u *UtxoCache) hashUTXOSetLE(bestLE [32]byte) [32]byte {
	if u == nil {
		return [32]byte{}
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	type agg struct {
		height int32
		maxV   int
		outs   map[int]struct {
			value  int64
			script []byte
		}
	}
	byTx := make(map[[32]byte]*agg)
	for k, e := range u.coins {
		var txid [32]byte
		copy(txid[:], k[:32])
		v := int(binary.LittleEndian.Uint32(k[32:36]))
		a := byTx[txid]
		if a == nil {
			a = &agg{height: int32(e.Height), outs: make(map[int]struct {
				value  int64
				script []byte
			})}
			byTx[txid] = a
		}
		if int32(e.Height) < a.height {
			a.height = int32(e.Height)
		}
		if v > a.maxV {
			a.maxV = v
		}
		a.outs[v] = struct {
			value  int64
			script []byte
		}{e.Value, e.PkScript}
	}
	txids := make([][32]byte, 0, len(byTx))
	for id := range byTx {
		txids = append(txids, id)
	}
	sort.Slice(txids, func(i, j int) bool {
		return bytes.Compare(txids[i][:], txids[j][:]) < 0
	})
	var buf bytes.Buffer
	_, _ = buf.Write(bestLE[:])
	for _, txid := range txids {
		a := byTx[txid]
		_, _ = buf.Write(txid[:])
		for i := 0; i <= a.maxV; i++ {
			o, ok := a.outs[i]
			if !ok {
				continue
			}
			_ = wire.WriteVarInt(&buf, uint64(i+1))
			_ = binary.Write(&buf, binary.LittleEndian, o.value)
			_ = wire.WriteVarInt(&buf, uint64(len(o.script)))
			_, _ = buf.Write(o.script)
		}
		_ = wire.WriteVarInt(&buf, 0)
	}
	first := sha256.Sum256(buf.Bytes())
	return sha256.Sum256(first[:])
}

// SerializedHashLE returns Core-compatible hash_serialized for best block hash (internal LE).
func (u *UtxoCache) SerializedHashLE(bestLE [32]byte) string {
	return hashSerializedHex(u.hashUTXOSetLE(bestLE))
}

// SerializedHashAtTip uses the header journal tip block hash as DB_BEST_BLOCK for hashing.
func (u *UtxoCache) SerializedHashAtTip(j *HeaderJournal) string {
	if u == nil {
		return hashSerializedHex([32]byte{})
	}
	if j == nil {
		return u.SerializedHashLE([32]byte{})
	}
	h80, err := j.ReadHeaderAt(u.TipHeight())
	if err != nil {
		return u.SerializedHashLE([32]byte{})
	}
	return u.SerializedHashLE(pow.BlockHashLE(h80))
}
