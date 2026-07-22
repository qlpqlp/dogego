// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"

	"dogego/analytics"
	"dogego/applog"
	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

const analyticsMaxBranch = 128

// recordHeaderReorgAnalytics snapshots displaced + incoming fork branches before prune.
// Best effort: never fails header connect.
func recordHeaderReorgAnalytics(j *store.HeaderJournal, aux *store.HeaderAuxJournal, bs *BlockStoreCtx, forkAt, tipH int64, decoded []wire.DecodedHeader, inc, cur *big.Int, precious bool) {
	if bs == nil || bs.Analytics == nil || forkAt >= tipH {
		return
	}
	ev := analytics.ReorgEvent{
		RecordedUnix:  time.Now().Unix(),
		Network:       bs.NetworkSlug,
		Kind:          "header_reorg",
		ForkAt:        forkAt,
		OldTipHeight:  tipH,
		Depth:         tipH - forkAt,
		IncomingCount: len(decoded),
		Precious:      precious,
	}
	if inc != nil {
		ev.IncomingWork = inc.String()
	}
	if cur != nil {
		ev.DisplacedWork = cur.String()
	}
	if inc != nil && cur != nil {
		ev.WorkDelta = new(big.Int).Sub(inc, cur).String()
	}

	addrVer := byte(30) // mainnet P2PKH; Params override when set
	if bs.Params.PubkeyHashAddrID != 0 {
		addrVer = bs.Params.PubkeyHashAddrID
	}

	displaced, dAux, dMiners, truncD := collectDisplacedReorgBlocks(j, aux, bs.Raw, forkAt+1, tipH, addrVer)
	incoming, iAux, iMiners, truncI := collectIncomingReorgBlocks(decoded, forkAt+1)
	ev.Displaced = displaced
	ev.Incoming = incoming
	ev.DisplacedAuxPowCount = dAux
	ev.IncomingAuxPowCount = iAux
	ev.DisplacedMinerCounts = dMiners
	ev.IncomingMinerCounts = iMiners
	ev.Truncated = truncD || truncI

	if err := analytics.RecordReorgEvent(bs.Analytics, ev); err != nil {
		applog.Line("indexer", "reorg analytics: "+err.Error())
		return
	}
	applog.Line("indexer", fmt.Sprintf(
		"reorg recorded: fork=%d depth=%d auxpow_d=%d auxpow_i=%d miners_d=%d miners_i=%d",
		ev.ForkAt, ev.Depth, ev.DisplacedAuxPowCount, ev.IncomingAuxPowCount,
		len(ev.DisplacedMinerCounts), len(ev.IncomingMinerCounts),
	))
}

func collectDisplacedReorgBlocks(j *store.HeaderJournal, aux *store.HeaderAuxJournal, raw *store.RawBlockStore, fromH, toH int64, addrVer byte) (blocks []analytics.ReorgBlockDetail, auxCount int, miners map[string]int, truncated bool) {
	miners = map[string]int{}
	if j == nil || fromH > toH {
		return nil, 0, nil, false
	}
	if toH-fromH+1 > analyticsMaxBranch {
		truncated = true
		fromH = toH - analyticsMaxBranch + 1
	}
	for h := fromH; h <= toH; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			continue
		}
		d := detailFromHeader80(h80, h)
		if aux != nil {
			if blob, err := aux.ReadAt(h); err == nil && len(blob) > 0 {
				d.AuxPow = true
				auxCount++
				if a, err := wire.ReadAuxPow(bytes.NewReader(blob)); err == nil && a != nil {
					d.ParentHash = pow.BlockHashHex(a.ParentHeader80[:])
				}
			} else if wire.HeaderHasAuxPowVersion(h80) {
				d.AuxPow = true
				auxCount++
			}
		} else if wire.HeaderHasAuxPowVersion(h80) {
			d.AuxPow = true
			auxCount++
		}
		fillMinerFromRaw(raw, pow.BlockHashLE(h80), addrVer, &d, miners)
		blocks = append(blocks, d)
	}
	if len(miners) == 0 {
		miners = nil
	}
	return blocks, auxCount, miners, truncated
}

func collectIncomingReorgBlocks(decoded []wire.DecodedHeader, startHeight int64) (blocks []analytics.ReorgBlockDetail, auxCount int, miners map[string]int, truncated bool) {
	if len(decoded) == 0 {
		return nil, 0, nil, false
	}
	limit := len(decoded)
	if limit > analyticsMaxBranch {
		truncated = true
		limit = analyticsMaxBranch
	}
	for i := 0; i < limit; i++ {
		h80 := decoded[i].Header80
		d := detailFromHeader80(h80, startHeight+int64(i))
		if decoded[i].Aux != nil {
			d.AuxPow = true
			auxCount++
			d.ParentHash = pow.BlockHashHex(decoded[i].Aux.ParentHeader80[:])
		} else if wire.HeaderHasAuxPowVersion(h80) {
			d.AuxPow = true
			auxCount++
		}
		d.MinerKind = "unknown"
		d.BodyAvailable = false
		blocks = append(blocks, d)
	}
	return blocks, auxCount, nil, truncated
}

func detailFromHeader80(h80 []byte, height int64) analytics.ReorgBlockDetail {
	d := analytics.ReorgBlockDetail{
		Height: height,
		Hash:   pow.BlockHashHex(h80),
	}
	if len(h80) >= 80 {
		d.TimeUnix = int64(binary.LittleEndian.Uint32(h80[68:72]))
		d.Bits = binary.LittleEndian.Uint32(h80[72:76])
	}
	return d
}

func fillMinerFromRaw(raw *store.RawBlockStore, id [32]byte, addrVer byte, d *analytics.ReorgBlockDetail, miners map[string]int) {
	if raw == nil || !raw.Has(id) {
		d.MinerKind = "unknown"
		d.BodyAvailable = false
		return
	}
	payload, err := raw.Get(id)
	if err != nil {
		d.MinerKind = "unknown"
		d.BodyAvailable = false
		return
	}
	d.BodyAvailable = true
	cb, _, err := wire.ReadTxAtIndex(payload, 0)
	if err != nil || cb == nil || !isCoinbaseTxWire(cb) {
		d.MinerKind = "unknown"
		return
	}
	var first string
	for _, o := range cb.Vout {
		a := chain.PayToPubKeyHashAddress(o.PkScript, addrVer)
		if a != "" {
			first = a
			break
		}
	}
	if first != "" {
		d.MinerAddress = first
		d.MinerKind = "p2pkh"
		miners[first]++
		return
	}
	d.MinerKind = "non_p2pkh"
	miners["(non-P2PKH or empty)"]++
}

func isCoinbaseTxWire(tx *wire.Tx) bool {
	if tx == nil || len(tx.Vin) != 1 {
		return false
	}
	if tx.Vin[0].PrevIdx != 0xffffffff {
		return false
	}
	var z [32]byte
	return tx.Vin[0].PrevHash == z
}
