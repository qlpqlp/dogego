// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"bytes"
	"fmt"

	"dogego/applog"
	"dogego/consensus"
	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
	"dogego/wire"
)

const (
	maxGetCFiltersRange = wire.MaxGetCFiltersRange
	maxGetCFHeadersRange = wire.MaxGetCFHeadersRange
)

type filterServeEnv struct {
	mw      *MsgWriter
	j       *store.HeaderJournal
	raw     *store.RawBlockStore
	txIx    *store.TxIndex
	filters *store.BlockFilterIndex
}

func (e filterServeEnv) ok() bool {
	return e.mw != nil && e.j != nil && e.filters != nil && e.raw != nil && e.txIx != nil
}

func resolveFilterRange(j *store.HeaderJournal, req wire.FilterRangeRequest, maxSpan int64) (startH, stopH int64, err error) {
	if req.FilterType != wire.FilterTypeBasic {
		return 0, 0, fmt.Errorf("unsupported filter type %d", req.FilterType)
	}
	stopHex := pow.LEUint256DisplayHex(req.StopHashLE[:])
	stopH, err = j.HeightByDisplayHash(stopHex)
	if err != nil {
		return 0, 0, fmt.Errorf("unknown stop hash")
	}
	startH = int64(req.StartHeight)
	if startH < 0 || startH > stopH {
		return 0, 0, fmt.Errorf("invalid filter range")
	}
	if stopH-startH+1 > maxSpan {
		return 0, 0, fmt.Errorf("filter range too large")
	}
	return startH, stopH, nil
}

func (e filterServeEnv) blockHashAt(h int64) ([32]byte, error) {
	var zero [32]byte
	h80, err := e.j.ReadHeaderAt(h)
	if err != nil {
		return zero, err
	}
	return pow.BlockHashLE(h80), nil
}

func (e filterServeEnv) loadOrBuildFilter(id [32]byte) ([]byte, [32]byte, error) {
	var zeroHdr [32]byte
	if e.filters.Has(id) {
		enc, hdr, err := e.filters.Get(id)
		if err != nil {
			return nil, zeroHdr, err
		}
		return enc, copy32(hdr), nil
	}
	payload, err := e.raw.Get(id)
	if err != nil {
		return nil, zeroHdr, err
	}
	if err := rpc.IndexBasicBlockFilter(e.filters, id, payload, e.j, e.raw, e.txIx); err != nil {
		return nil, zeroHdr, err
	}
	enc, hdr, err := e.filters.Get(id)
	if err != nil {
		return nil, zeroHdr, err
	}
	return enc, copy32(hdr), nil
}

func copy32(b []byte) [32]byte {
	var out [32]byte
	if len(b) >= 32 {
		copy(out[:], b[:32])
	}
	return out
}

// HandleInboundGetCFilters serves BIP157 cfilter messages for stored basic filters.
func HandleInboundGetCFilters(mw *MsgWriter, j *store.HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, filters *store.BlockFilterIndex, pl []byte) error {
	env := filterServeEnv{mw: mw, j: j, raw: raw, txIx: txIx, filters: filters}
	if !env.ok() {
		return nil
	}
	req, err := wire.DecodeFilterRangeRequest(pl)
	if err != nil {
		return err
	}
	startH, stopH, err := resolveFilterRange(j, req, maxGetCFiltersRange)
	if err != nil {
		return err
	}
	sent := 0
	for h := startH; h <= stopH; h++ {
		id, err := env.blockHashAt(h)
		if err != nil {
			continue
		}
		encoded, _, err := env.loadOrBuildFilter(id)
		if err != nil || len(encoded) == 0 {
			continue
		}
		nEl := filterElementCount(encoded)
		body, err := wire.EncodeCFilterPayload(wire.CFilterPayload{
			BlockHashLE: id,
			FilterType:  wire.FilterTypeBasic,
			Filter:      encoded,
			NumElements: nEl,
		})
		if err != nil {
			return err
		}
		if err := mw.Write("cfilter", body); err != nil {
			return err
		}
		sent++
	}
	if sent > 0 {
		applog.Line("net", fmt.Sprintf("served %d cfilter(s) heights %d..%d", sent, startH, stopH))
	}
	return nil
}

// HandleInboundGetCFHeaders serves BIP157 cfheaders (filter hashes + prev header).
func HandleInboundGetCFHeaders(mw *MsgWriter, j *store.HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, filters *store.BlockFilterIndex, pl []byte) error {
	env := filterServeEnv{mw: mw, j: j, raw: raw, txIx: txIx, filters: filters}
	if !env.ok() {
		return nil
	}
	req, err := wire.DecodeFilterRangeRequest(pl)
	if err != nil {
		return err
	}
	startH, stopH, err := resolveFilterRange(j, req, maxGetCFHeadersRange)
	if err != nil {
		return err
	}
	var prevHeader [32]byte
	if startH > 0 {
		id, err := env.blockHashAt(startH - 1)
		if err == nil {
			if _, hdr, err := env.loadOrBuildFilter(id); err == nil {
				prevHeader = hdr
			}
		}
	}
	var hashes [][32]byte
	for h := startH; h <= stopH; h++ {
		id, err := env.blockHashAt(h)
		if err != nil {
			continue
		}
		enc, _, err := env.loadOrBuildFilter(id)
		if err != nil || len(enc) == 0 {
			continue
		}
		hashes = append(hashes, consensus.BlockFilterHash(enc))
	}
	if len(hashes) == 0 {
		return nil
	}
	body, err := wire.EncodeCFHeadersPayload(wire.CFHeadersPayload{
		FilterType:           wire.FilterTypeBasic,
		StopHashLE:           req.StopHashLE,
		PreviousFilterHeader: prevHeader,
		FilterHashes:         hashes,
	})
	if err != nil {
		return err
	}
	if err := mw.Write("cfheaders", body); err != nil {
		return err
	}
	applog.Line("net", fmt.Sprintf("served cfheaders heights %d..%d (%d hashes)", startH, stopH, len(hashes)))
	return nil
}

// HandleInboundGetCFCheckpt serves BIP157 cfcheckpt (filter headers every 1000 blocks).
func HandleInboundGetCFCheckpt(mw *MsgWriter, j *store.HeaderJournal, raw *store.RawBlockStore, txIx *store.TxIndex, filters *store.BlockFilterIndex, pl []byte) error {
	env := filterServeEnv{mw: mw, j: j, raw: raw, txIx: txIx, filters: filters}
	if !env.ok() {
		return nil
	}
	req, err := wire.DecodeGetCFCheckptPayload(pl)
	if err != nil {
		return err
	}
	if req.FilterType != wire.FilterTypeBasic {
		return fmt.Errorf("unsupported filter type %d", req.FilterType)
	}
	stopHex := pow.LEUint256DisplayHex(req.StopHashLE[:])
	stopH, err := j.HeightByDisplayHash(stopHex)
	if err != nil {
		return fmt.Errorf("unknown stop hash")
	}
	var headers [][32]byte
	for h := int64(wire.CFCheckptInterval); h <= stopH; h += wire.CFCheckptInterval {
		id, err := env.blockHashAt(h)
		if err != nil {
			continue
		}
		_, hdr, err := env.loadOrBuildFilter(id)
		if err != nil {
			continue
		}
		headers = append(headers, hdr)
	}
	body, err := wire.EncodeCFCheckptPayload(wire.CFCheckptPayload{
		FilterType:    wire.FilterTypeBasic,
		StopHashLE:    req.StopHashLE,
		FilterHeaders: headers,
	})
	if err != nil {
		return err
	}
	if err := mw.Write("cfcheckpt", body); err != nil {
		return err
	}
	applog.Line("net", fmt.Sprintf("served cfcheckpt through height %d (%d headers)", stopH, len(headers)))
	return nil
}

func filterElementCount(encoded []byte) uint32 {
	if n, err := wire.ReadCompactSize(bytes.NewReader(encoded)); err == nil {
		if n > 0xffffffff {
			return 0xffffffff
		}
		return uint32(n)
	}
	return 0
}
