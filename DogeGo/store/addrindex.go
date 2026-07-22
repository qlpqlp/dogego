// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"dogego/chain"
	"dogego/wire"
)

const (
	addrRecvRecordLen  = 60
	addrSpendRecordLen = 96
	outSpendRecordLen  = 48
)

// AddrIndex maps hash160 to receive/spend history and outpoint spends for O(1) explorer lookups.
// Built alongside the tx index from raw blocks; rebuild with reindextx (clear=true recommended).
type AddrIndex struct {
	recvRoot  string
	spendRoot string
	outRoot   string
	mu        sync.Mutex

	txIx *TxIndex
	raw  *RawBlockStore
}

// AddrReceiveHit is one indexed output paying hash160.
type AddrReceiveHit struct {
	Height  int64
	TxIndex uint32
	Vout    uint32
	Value   int64
	TxID    string
}

// AddrSpendHit is one indexed input spending a prevout that paid hash160.
type AddrSpendHit struct {
	Height   int64
	TxIndex  uint32
	Vin      uint32
	Value    int64
	TxID     string
	PrevTxID string
	PrevVout uint32
}

// OutpointSpendHit locates the spend of a prevout.
type OutpointSpendHit struct {
	Height  int64
	TxIndex uint32
	Vin     uint32
	TxID    string
}

// OpenAddrIndex creates indexes/addr/{recv,spend,outspend} under chain data dir.
func OpenAddrIndex(chainDataDir string) (*AddrIndex, error) {
	base := filepath.Join(chainDataDir, "indexes", "addr")
	for _, sub := range []string{"recv", "spend", "outspend"} {
		if err := os.MkdirAll(filepath.Join(base, sub), 0o700); err != nil {
			return nil, err
		}
	}
	return &AddrIndex{
		recvRoot:  filepath.Join(base, "recv"),
		spendRoot: filepath.Join(base, "spend"),
		outRoot:   filepath.Join(base, "outspend"),
	}, nil
}

// SetResolver wires tx index + raw blocks for cross-block prevout resolution during indexing.
func (a *AddrIndex) SetResolver(txIx *TxIndex, raw *RawBlockStore) {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.txIx = txIx
	a.raw = raw
	a.mu.Unlock()
}

// RootDir returns indexes/addr.
func (a *AddrIndex) RootDir() string {
	if a == nil {
		return ""
	}
	return filepath.Dir(a.recvRoot)
}

// HasAny reports whether any address index files exist.
func (a *AddrIndex) HasAny() bool {
	if a == nil {
		return false
	}
	for _, root := range []string{a.recvRoot, a.spendRoot, a.outRoot} {
		entries, err := os.ReadDir(root)
		if err == nil && len(entries) > 0 {
			return true
		}
	}
	return false
}

// ClearAddrIndex removes all address index files.
func ClearAddrIndex(a *AddrIndex) error {
	if a == nil {
		return fmt.Errorf("nil addr index")
	}
	for _, root := range []string{a.recvRoot, a.spendRoot, a.outRoot} {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if err := os.Remove(filepath.Join(root, e.Name())); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func hash160FileName(h [20]byte) string {
	return strings.ToLower(hex.EncodeToString(h[:]))
}

func pkScriptHash160(pkScript []byte) ([20]byte, bool) {
	var h [20]byte
	if len(pkScript) == 25 && pkScript[0] == 0x76 && pkScript[1] == 0xa9 && pkScript[2] == 0x14 && pkScript[23] == 0x88 && pkScript[24] == 0xac {
		copy(h[:], pkScript[3:23])
		return h, true
	}
	if len(pkScript) == 23 && pkScript[0] == 0xa9 && pkScript[1] == 0x14 && pkScript[22] == 0x87 {
		copy(h[:], pkScript[2:22])
		return h, true
	}
	return h, false
}

func blockHeightFromRaw(raw []byte) int64 {
	tx, _, err := wire.ReadTxAtIndex(raw, 0)
	if err != nil || tx == nil || len(tx.Vin) == 0 {
		return -1
	}
	if h, ok := coinbaseHeightFromScript(tx.Vin[0].Script); ok {
		return h
	}
	return -1
}

func coinbaseHeightFromScript(script []byte) (int64, bool) {
	if len(script) < 1 {
		return 0, false
	}
	n := int(script[0])
	if n >= 1 && n <= 4 && len(script) >= 1+n {
		var h int64
		for i := 0; i < n; i++ {
			h |= int64(script[1+i]) << (8 * i)
		}
		return h, true
	}
	if len(script) >= 5 && script[0] == 4 {
		return int64(binary.LittleEndian.Uint32(script[1:5])), true
	}
	return 0, false
}

func encodeRecvRecord(height int64, txIndex, vout uint32, value int64, txHash [32]byte) []byte {
	b := make([]byte, addrRecvRecordLen)
	binary.LittleEndian.PutUint64(b[0:8], uint64(height))
	binary.LittleEndian.PutUint32(b[8:12], txIndex)
	binary.LittleEndian.PutUint32(b[12:16], vout)
	binary.LittleEndian.PutUint64(b[16:24], uint64(value))
	copy(b[24:56], txHash[:])
	return b
}

func decodeRecvRecord(b []byte) (AddrReceiveHit, bool) {
	if len(b) < addrRecvRecordLen {
		return AddrReceiveHit{}, false
	}
	var txHash [32]byte
	copy(txHash[:], b[24:56])
	return AddrReceiveHit{
		Height:  int64(binary.LittleEndian.Uint64(b[0:8])),
		TxIndex: binary.LittleEndian.Uint32(b[8:12]),
		Vout:    binary.LittleEndian.Uint32(b[12:16]),
		Value:   int64(binary.LittleEndian.Uint64(b[16:24])),
		TxID:    txidRPCFileName(txHash),
	}, true
}

func encodeSpendRecord(height int64, txIndex, vin uint32, value int64, txHash, prevHash [32]byte, prevVout uint32) []byte {
	b := make([]byte, addrSpendRecordLen)
	binary.LittleEndian.PutUint64(b[0:8], uint64(height))
	binary.LittleEndian.PutUint32(b[8:12], txIndex)
	binary.LittleEndian.PutUint32(b[12:16], vin)
	binary.LittleEndian.PutUint64(b[16:24], uint64(value))
	copy(b[24:56], txHash[:])
	copy(b[56:88], prevHash[:])
	binary.LittleEndian.PutUint32(b[88:92], prevVout)
	return b
}

func decodeSpendRecord(b []byte) (AddrSpendHit, bool) {
	if len(b) < addrSpendRecordLen {
		return AddrSpendHit{}, false
	}
	var txHash, prevHash [32]byte
	copy(txHash[:], b[24:56])
	copy(prevHash[:], b[56:88])
	return AddrSpendHit{
		Height:   int64(binary.LittleEndian.Uint64(b[0:8])),
		TxIndex:  binary.LittleEndian.Uint32(b[8:12]),
		Vin:      binary.LittleEndian.Uint32(b[12:16]),
		Value:    int64(binary.LittleEndian.Uint64(b[16:24])),
		TxID:     txidRPCFileName(txHash),
		PrevTxID: txidRPCFileName(prevHash),
		PrevVout: binary.LittleEndian.Uint32(b[88:92]),
	}, true
}

func encodeOutSpendRecord(height int64, txIndex, vin uint32, txHash [32]byte) []byte {
	b := make([]byte, outSpendRecordLen)
	binary.LittleEndian.PutUint64(b[0:8], uint64(height))
	binary.LittleEndian.PutUint32(b[8:12], txIndex)
	binary.LittleEndian.PutUint32(b[12:16], vin)
	copy(b[16:48], txHash[:])
	return b
}

func decodeOutSpendRecord(b []byte) (OutpointSpendHit, bool) {
	if len(b) < outSpendRecordLen {
		return OutpointSpendHit{}, false
	}
	var txHash [32]byte
	copy(txHash[:], b[16:48])
	return OutpointSpendHit{
		Height:  int64(binary.LittleEndian.Uint64(b[0:8])),
		TxIndex: binary.LittleEndian.Uint32(b[8:12]),
		Vin:     binary.LittleEndian.Uint32(b[12:16]),
		TxID:    txidRPCFileName(txHash),
	}, true
}

func (a *AddrIndex) appendRecord(root, name string, rec []byte) error {
	path := filepath.Join(root, name)
	a.mu.Lock()
	defer a.mu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(rec)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func wireHashFromDisplay(txid string) ([32]byte, bool) {
	txid = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(txid, "0x")))
	if len(txid) != 64 {
		return [32]byte{}, false
	}
	b, err := hex.DecodeString(txid)
	if err != nil || len(b) != 32 {
		return [32]byte{}, false
	}
	var h [32]byte
	for i := 0; i < 32; i++ {
		h[i] = b[31-i]
	}
	return h, true
}

func (a *AddrIndex) resolvePrevout(prevHash [32]byte, prevVout uint32, blockTx map[[32]byte]*wire.Tx) (value int64, pkScript []byte, ok bool) {
	if tx, inBlock := blockTx[prevHash]; inBlock && int(prevVout) < len(tx.Vout) {
		o := tx.Vout[prevVout]
		return o.Value, append([]byte(nil), o.PkScript...), true
	}
	prevID := txidRPCFileName(prevHash)
	a.mu.Lock()
	txIx := a.txIx
	raw := a.raw
	a.mu.Unlock()
	if txIx != nil && raw != nil {
		if val, spk, ok := LoadIndexedTxVout(txIx, raw, prevID, prevVout); ok {
			return val, spk, true
		}
		if tx, err := LoadIndexedTx(txIx, raw, prevID); err == nil && int(prevVout) < len(tx.Vout) {
			o := tx.Vout[prevVout]
			return o.Value, append([]byte(nil), o.PkScript...), true
		}
	}
	return 0, nil, false
}

// IndexBlock appends receive/spend/outspend records for one stored block payload.
func (a *AddrIndex) IndexBlock(_ [32]byte, raw []byte) error {
	if a == nil {
		return nil
	}
	height := blockHeightFromRaw(raw)
	var txs []*wire.Tx
	if err := wire.ForEachBlockTx(raw, func(_ uint32, tx *wire.Tx) error {
		txs = append(txs, tx)
		return nil
	}); err != nil {
		return err
	}
	blockTx := make(map[[32]byte]*wire.Tx, len(txs))
	for _, tx := range txs {
		blockTx[tx.TxHash()] = tx
	}
	for ti, tx := range txs {
		txHash := tx.TxHash()
		for vi, o := range tx.Vout {
			h160, ok := pkScriptHash160(o.PkScript)
			if !ok {
				continue
			}
			if err := a.appendRecord(a.recvRoot, hash160FileName(h160), encodeRecvRecord(height, uint32(ti), uint32(vi), o.Value, txHash)); err != nil {
				return fmt.Errorf("addr recv index: %w", err)
			}
		}
		for vi, in := range tx.Vin {
			var z [32]byte
			if in.PrevHash == z && in.PrevIdx == 0xffffffff {
				continue
			}
			outName := txidRPCFileName(in.PrevHash) + fmt.Sprintf("_%d", in.PrevIdx)
			if err := a.appendRecord(a.outRoot, outName, encodeOutSpendRecord(height, uint32(ti), uint32(vi), txHash)); err != nil {
				return fmt.Errorf("outspend index: %w", err)
			}
			val, spk, ok := a.resolvePrevout(in.PrevHash, in.PrevIdx, blockTx)
			if !ok {
				continue
			}
			h160, ok := pkScriptHash160(spk)
			if !ok {
				continue
			}
			if err := a.appendRecord(a.spendRoot, hash160FileName(h160), encodeSpendRecord(height, uint32(ti), uint32(vi), val, txHash, in.PrevHash, in.PrevIdx)); err != nil {
				return fmt.Errorf("addr spend index: %w", err)
			}
		}
	}
	return nil
}

func readAllRecords(root, name string, recLen int, decode func([]byte) (any, bool)) ([]any, error) {
	path := filepath.Join(root, name)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(b)%recLen != 0 {
		b = b[:len(b)-len(b)%recLen]
	}
	out := make([]any, 0, len(b)/recLen)
	for off := 0; off+recLen <= len(b); off += recLen {
		if v, ok := decode(b[off : off+recLen]); ok {
			out = append(out, v)
		}
	}
	return out, nil
}

// readAddrRecordsNewestFirst pages fixed-size append-only addr index files without loading the full history.
// Records are stored in connect order (oldest first); paging returns newest-first slices.
func readAddrRecordsNewestFirst(root, name string, recLen, offset, limit int, decode func([]byte) (any, bool)) ([]any, int, error) {
	path := filepath.Join(root, name)
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	size := st.Size()
	if size <= 0 {
		return nil, 0, nil
	}
	rem := size % int64(recLen)
	if rem != 0 {
		size -= rem
	}
	total := int(size / int64(recLen))
	if total == 0 {
		return nil, 0, nil
	}
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	want := limit
	if want <= 0 {
		want = total - offset
	}
	if want < 0 {
		want = 0
	}
	endIdx := total - 1 - offset
	startIdx := endIdx - want + 1
	if startIdx < 0 {
		startIdx = 0
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	out := make([]any, 0, endIdx-startIdx+1)
	buf := make([]byte, recLen)
	for rec := endIdx; rec >= startIdx; rec-- {
		off := int64(rec) * int64(recLen)
		if _, err := f.ReadAt(buf, off); err != nil {
			return nil, total, err
		}
		if v, ok := decode(buf); ok {
			out = append(out, v)
		}
	}
	return out, total, nil
}

// LookupReceives returns receive hits for hash160 newest-first with offset/limit paging.
func (a *AddrIndex) LookupReceives(h160 [20]byte, offset, limit int) (hits []AddrReceiveHit, total int, err error) {
	if a == nil {
		return nil, 0, fmt.Errorf("addr index disabled")
	}
	raw, total, err := readAddrRecordsNewestFirst(a.recvRoot, hash160FileName(h160), addrRecvRecordLen, offset, limit, func(b []byte) (any, bool) {
		h, ok := decodeRecvRecord(b)
		return h, ok
	})
	if err != nil {
		return nil, 0, err
	}
	hits = make([]AddrReceiveHit, 0, len(raw))
	for _, v := range raw {
		hits = append(hits, v.(AddrReceiveHit))
	}
	return hits, total, nil
}

// LookupSpends returns spend hits for hash160 newest-first with offset/limit paging.
func (a *AddrIndex) LookupSpends(h160 [20]byte, offset, limit int) (hits []AddrSpendHit, total int, err error) {
	if a == nil {
		return nil, 0, fmt.Errorf("addr index disabled")
	}
	raw, total, err := readAddrRecordsNewestFirst(a.spendRoot, hash160FileName(h160), addrSpendRecordLen, offset, limit, func(b []byte) (any, bool) {
		h, ok := decodeSpendRecord(b)
		return h, ok
	})
	if err != nil {
		return nil, 0, err
	}
	hits = make([]AddrSpendHit, 0, len(raw))
	for _, v := range raw {
		hits = append(hits, v.(AddrSpendHit))
	}
	return hits, total, nil
}

// LookupOutpointSpend finds the spend of prevTxid:prevVout from the outspend index.
func (a *AddrIndex) LookupOutpointSpend(prevTxid string, prevVout int) (OutpointSpendHit, bool) {
	if a == nil || prevVout < 0 {
		return OutpointSpendHit{}, false
	}
	prevTxid = strings.ToLower(strings.TrimSpace(prevTxid))
	name := prevTxid + fmt.Sprintf("_%d", prevVout)
	b, err := os.ReadFile(filepath.Join(a.outRoot, name))
	if err != nil || len(b) < outSpendRecordLen {
		return OutpointSpendHit{}, false
	}
	hit, ok := decodeOutSpendRecord(b[:outSpendRecordLen])
	return hit, ok
}

// Hash160FromAddress decodes a base58 address to hash160 (any network version).
func Hash160FromAddress(address string) ([20]byte, bool) {
	address = strings.TrimSpace(address)
	if address == "" {
		return [20]byte{}, false
	}
	_, h, err := chain.Base58CheckDecode(address)
	if err != nil {
		return [20]byte{}, false
	}
	return h, true
}
