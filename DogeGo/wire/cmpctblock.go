// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// PrefilledTransaction is one explicit transaction inside a BIP152 HeaderAndShortIDs.
type PrefilledTransaction struct {
	Index uint64
	Tx    []byte // wire-encoded tx (same as "tx" message body)
}

// HeaderAndShortIDs is the cmpctblock message payload (BIP152 v1).
type HeaderAndShortIDs struct {
	Header80  [80]byte
	Nonce     uint64
	ShortIDs  []uint64
	Prefilled []PrefilledTransaction
}

// BlockTransactionsRequest is the getblocktxn message payload.
type BlockTransactionsRequest struct {
	BlockHash [32]byte
	Indexes   []uint64
}

// BlockTransactions is the blocktxn message payload.
type BlockTransactions struct {
	BlockHash    [32]byte
	Transactions [][]byte
}

// EncodeHeaderAndShortIDs serializes a cmpctblock body.
func EncodeHeaderAndShortIDs(h *HeaderAndShortIDs) ([]byte, error) {
	if h == nil {
		return nil, fmt.Errorf("nil HeaderAndShortIDs")
	}
	var b bytes.Buffer
	if _, err := b.Write(h.Header80[:]); err != nil {
		return nil, err
	}
	if err := binary.Write(&b, binary.LittleEndian, h.Nonce); err != nil {
		return nil, err
	}
	if err := WriteCompactSize(&b, uint64(len(h.ShortIDs))); err != nil {
		return nil, err
	}
	if _, err := b.Write(EncodeCmpctShortIDs(h.ShortIDs)); err != nil {
		return nil, err
	}
	if err := WriteCompactSize(&b, uint64(len(h.Prefilled))); err != nil {
		return nil, err
	}
	var prev int64 = -1
	for _, pf := range h.Prefilled {
		if err := WriteCompactSizeDifferential(&b, pf.Index, prev); err != nil {
			return nil, err
		}
		prev = int64(pf.Index)
		if err := encodePrefilledTx(&b, pf.Tx); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}

// DecodeHeaderAndShortIDs parses a cmpctblock body.
func DecodeHeaderAndShortIDs(payload []byte) (*HeaderAndShortIDs, error) {
	r := bytes.NewReader(payload)
	var out HeaderAndShortIDs
	if _, err := io.ReadFull(r, out.Header80[:]); err != nil {
		return nil, fmt.Errorf("cmpct header: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &out.Nonce); err != nil {
		return nil, fmt.Errorf("cmpct nonce: %w", err)
	}
	nShort, err := ReadCompactSize(r)
	if err != nil {
		return nil, fmt.Errorf("cmpct shortids_length: %w", err)
	}
	if nShort > 100000 {
		return nil, fmt.Errorf("cmpct shortids_length too large %d", nShort)
	}
	shortRaw := make([]byte, nShort*cmpctShortTxIDBytes)
	if _, err := io.ReadFull(r, shortRaw); err != nil {
		return nil, fmt.Errorf("cmpct shortids: %w", err)
	}
	out.ShortIDs, err = DecodeCmpctShortIDs(shortRaw)
	if err != nil {
		return nil, err
	}
	nPref, err := ReadCompactSize(r)
	if err != nil {
		return nil, fmt.Errorf("cmpct prefilled_length: %w", err)
	}
	if nPref > 100000 {
		return nil, fmt.Errorf("cmpct prefilled_length too large %d", nPref)
	}
	out.Prefilled = make([]PrefilledTransaction, 0, nPref)
	var prev int64 = -1
	for i := uint64(0); i < nPref; i++ {
		idx, err := ReadCompactSizeDifferential(r, prev)
		if err != nil {
			return nil, fmt.Errorf("cmpct prefilled[%d] index: %w", i, err)
		}
		prev = int64(idx)
		txRaw, err := readPrefilledTxFromPayload(r, payload)
		if err != nil {
			return nil, fmt.Errorf("cmpct prefilled[%d] tx: %w", i, err)
		}
		out.Prefilled = append(out.Prefilled, PrefilledTransaction{Index: idx, Tx: txRaw})
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("cmpct trailing %d bytes", r.Len())
	}
	return &out, nil
}

// EncodeBlockTransactionsRequest serializes getblocktxn.
func EncodeBlockTransactionsRequest(req *BlockTransactionsRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("nil BlockTransactionsRequest")
	}
	var b bytes.Buffer
	if _, err := b.Write(req.BlockHash[:]); err != nil {
		return nil, err
	}
	if err := WriteCompactSize(&b, uint64(len(req.Indexes))); err != nil {
		return nil, err
	}
	var prev int64 = -1
	for _, idx := range req.Indexes {
		if err := WriteCompactSizeDifferential(&b, idx, prev); err != nil {
			return nil, err
		}
		prev = int64(idx)
	}
	return b.Bytes(), nil
}

// DecodeBlockTransactionsRequest parses getblocktxn.
func DecodeBlockTransactionsRequest(payload []byte) (*BlockTransactionsRequest, error) {
	r := bytes.NewReader(payload)
	var out BlockTransactionsRequest
	if _, err := io.ReadFull(r, out.BlockHash[:]); err != nil {
		return nil, fmt.Errorf("getblocktxn blockhash: %w", err)
	}
	n, err := ReadCompactSize(r)
	if err != nil {
		return nil, fmt.Errorf("getblocktxn indexes_length: %w", err)
	}
	if n > 100000 {
		return nil, fmt.Errorf("getblocktxn indexes_length too large %d", n)
	}
	out.Indexes = make([]uint64, 0, n)
	var prev int64 = -1
	for i := uint64(0); i < n; i++ {
		idx, err := ReadCompactSizeDifferential(r, prev)
		if err != nil {
			return nil, fmt.Errorf("getblocktxn index[%d]: %w", i, err)
		}
		prev = int64(idx)
		out.Indexes = append(out.Indexes, idx)
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("getblocktxn trailing %d bytes", r.Len())
	}
	return &out, nil
}

// EncodeBlockTransactions serializes blocktxn.
func EncodeBlockTransactions(bt *BlockTransactions) ([]byte, error) {
	if bt == nil {
		return nil, fmt.Errorf("nil BlockTransactions")
	}
	var b bytes.Buffer
	if _, err := b.Write(bt.BlockHash[:]); err != nil {
		return nil, err
	}
	if err := WriteCompactSize(&b, uint64(len(bt.Transactions))); err != nil {
		return nil, err
	}
	for i, tx := range bt.Transactions {
		if err := encodePrefilledTx(&b, tx); err != nil {
			return nil, fmt.Errorf("blocktxn tx[%d]: %w", i, err)
		}
	}
	return b.Bytes(), nil
}

// DecodeBlockTransactions parses blocktxn.
func DecodeBlockTransactions(payload []byte) (*BlockTransactions, error) {
	r := bytes.NewReader(payload)
	var out BlockTransactions
	if _, err := io.ReadFull(r, out.BlockHash[:]); err != nil {
		return nil, fmt.Errorf("blocktxn blockhash: %w", err)
	}
	n, err := ReadCompactSize(r)
	if err != nil {
		return nil, fmt.Errorf("blocktxn transactions_length: %w", err)
	}
	if n > 100000 {
		return nil, fmt.Errorf("blocktxn transactions_length too large %d", n)
	}
	out.Transactions = make([][]byte, 0, n)
	for i := uint64(0); i < n; i++ {
		txRaw, err := readPrefilledTxFromPayload(r, payload)
		if err != nil {
			return nil, fmt.Errorf("blocktxn tx[%d]: %w", i, err)
		}
		out.Transactions = append(out.Transactions, txRaw)
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("blocktxn trailing %d bytes", r.Len())
	}
	return &out, nil
}

// CmpctBlockTxCount returns the implied transaction count in a HeaderAndShortIDs.
func CmpctBlockTxCount(h *HeaderAndShortIDs) int {
	if h == nil {
		return 0
	}
	return len(h.ShortIDs) + len(h.Prefilled)
}

func encodePrefilledTx(w io.Writer, tx []byte) error {
	if len(tx) == 0 {
		return fmt.Errorf("empty tx")
	}
	if _, err := w.Write(tx); err != nil {
		return err
	}
	return nil
}

func readPrefilledTxFromPayload(r *bytes.Reader, payload []byte) ([]byte, error) {
	start := len(payload) - r.Len()
	if _, err := ReadTx(r); err != nil {
		return nil, err
	}
	end := len(payload) - r.Len()
	if start < 0 || end < start || end > len(payload) {
		return nil, fmt.Errorf("prefilled tx bounds")
	}
	return append([]byte(nil), payload[start:end]...), nil
}
