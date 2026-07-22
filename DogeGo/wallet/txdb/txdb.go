// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package txdb is the local wallet transaction index (addresses + tx history +
// scan cursor) - the Core wallet.dat analogue. Backed by Pebble (pure-Go LSM).
package txdb

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cockroachdb/pebble"
)

// noopPebbleLogger silences Pebble's default stdlib logging (WAL replay / compaction).
type noopPebbleLogger struct{}

func (noopPebbleLogger) Infof(string, ...interface{})  {}
func (noopPebbleLogger) Errorf(string, ...interface{}) {}
func (noopPebbleLogger) Fatalf(string, ...interface{}) {}

const schemaVersion = 1

// Key layout (all big-endian so lexical order == numeric order):
//
//	m/schema_version            -> ascii schema version
//	c                           -> scan cursor (int64, offset-encoded so -1 sorts fine)
//	t/<height:8><txid><0><vout:4><0><category><0><address> -> json(TxRow)
//
// Height is stored offset by (1<<63) so signed heights order correctly and
// range operations ("delete >= fromHeight", "newest first") are prefix scans.
var (
	keyMetaSchema = []byte("m/schema_version")
	keyCursor     = []byte("c")
	prefixTx      = []byte("t/")
)

const heightBias = uint64(1) << 63

func encodeHeight(h int64) uint64 { return uint64(h) + heightBias }

// txKey builds the row key for a tx_log entry. The 8-byte biased height prefix
// keeps entries height-ordered; the remaining fields make the key unique per
// unique per (txid, vout, category, address).
func txKey(r TxRow) []byte {
	var b bytes.Buffer
	b.Write(prefixTx)
	var hb [8]byte
	binary.BigEndian.PutUint64(hb[:], encodeHeight(r.BlockHeight))
	b.Write(hb[:])
	b.WriteString(r.TxID)
	b.WriteByte(0)
	var vb [4]byte
	binary.BigEndian.PutUint32(vb[:], r.Vout)
	b.Write(vb[:])
	b.WriteByte(0)
	b.WriteString(r.Category)
	b.WriteByte(0)
	b.WriteString(r.Address)
	return b.Bytes()
}

// txHeightBound returns the key at the start of the given height (inclusive
// lower bound for "block_height >= fromHeight" range operations).
func txHeightBound(fromHeight int64) []byte {
	var b bytes.Buffer
	b.Write(prefixTx)
	var hb [8]byte
	binary.BigEndian.PutUint64(hb[:], encodeHeight(fromHeight))
	b.Write(hb[:])
	return b.Bytes()
}

// prefixEnd returns the exclusive upper bound for a prefix scan.
func prefixEnd(p []byte) []byte {
	end := make([]byte, len(p))
	copy(end, p)
	for i := len(end) - 1; i >= 0; i-- {
		end[i]++
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil // prefix is all 0xff: no upper bound
}

// DB is the local wallet transaction index (addresses + tx history + scan cursor).
type DB struct {
	db   *pebble.DB
	path string
}

// DefaultPath returns the wallet index directory beside wallet.json.
// The directory name keeps the historical "wallet.db" identity.
func DefaultPath(walletJSONPath string) string {
	return filepath.Join(filepath.Dir(walletJSONPath), "wallet.db")
}

// Open opens or creates the wallet index at dbPath (a Pebble directory).
func Open(dbPath string) (*DB, error) {
	dbPath = filepath.Clean(dbPath)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, err
	}
	pdb, err := pebble.Open(dbPath, &pebble.Options{Logger: noopPebbleLogger{}})
	if err != nil {
		return nil, fmt.Errorf("wallet.db open: %w", err)
	}
	w := &DB{db: pdb, path: dbPath}
	if err := w.migrate(); err != nil {
		_ = pdb.Close()
		return nil, err
	}
	return w, nil
}

func (w *DB) migrate() error {
	_, closer, err := w.db.Get(keyMetaSchema)
	if err == pebble.ErrNotFound {
		return w.db.Set(keyMetaSchema, []byte(fmt.Sprintf("%d", schemaVersion)), pebble.Sync)
	}
	if err != nil {
		return fmt.Errorf("wallet.db migrate: %w", err)
	}
	_ = closer.Close()
	return nil
}

// Close closes the database handle.
func (w *DB) Close() error {
	if w == nil || w.db == nil {
		return nil
	}
	db := w.db
	w.db = nil
	return db.Close()
}

// TxRow is one wallet-affecting transaction output in the index.
type TxRow struct {
	TxID        string
	Category    string
	Address     string
	AmountKoinu int64
	FeeKoinu    int64
	Vout        uint32
	BlockHeight int64
}

type txRowValue struct {
	AmountKoinu int64 `json:"a"`
	FeeKoinu    int64 `json:"f,omitempty"`
}

func (r TxRow) encodeValue() []byte {
	b, _ := json.Marshal(txRowValue{AmountKoinu: r.AmountKoinu, FeeKoinu: r.FeeKoinu})
	return b
}

// decodeTxRow reconstructs a TxRow from its key + value.
func decodeTxRow(key, val []byte) (TxRow, bool) {
	if !bytes.HasPrefix(key, prefixTx) {
		return TxRow{}, false
	}
	body := key[len(prefixTx):]
	if len(body) < 8 {
		return TxRow{}, false
	}
	h := int64(binary.BigEndian.Uint64(body[:8]) - heightBias)
	rest := body[8:]
	// txid <0> vout(4) <0> category <0> address
	i := bytes.IndexByte(rest, 0)
	if i < 0 || len(rest) < i+1+4+1 {
		return TxRow{}, false
	}
	txid := string(rest[:i])
	rest = rest[i+1:]
	vout := binary.BigEndian.Uint32(rest[:4])
	rest = rest[4:]
	if len(rest) < 1 || rest[0] != 0 {
		return TxRow{}, false
	}
	rest = rest[1:]
	j := bytes.IndexByte(rest, 0)
	if j < 0 {
		return TxRow{}, false
	}
	category := string(rest[:j])
	address := string(rest[j+1:])
	var v txRowValue
	_ = json.Unmarshal(val, &v)
	return TxRow{
		TxID: txid, Category: category, Address: address,
		AmountKoinu: v.AmountKoinu, FeeKoinu: v.FeeKoinu, Vout: vout, BlockHeight: h,
	}, true
}

// MaxScannedHeight returns the highest indexed block height (-1 when empty).
func (w *DB) MaxScannedHeight() (int64, error) {
	it, err := w.db.NewIter(&pebble.IterOptions{
		LowerBound: prefixTx,
		UpperBound: prefixEnd(prefixTx),
	})
	if err != nil {
		return -1, err
	}
	defer it.Close()
	if !it.Last() {
		return -1, nil
	}
	key := it.Key()
	body := key[len(prefixTx):]
	if len(body) < 8 {
		return -1, nil
	}
	return int64(binary.BigEndian.Uint64(body[:8]) - heightBias), nil
}

// ScanCursor returns the persisted wallet scan cursor (-1 when unset).
func (w *DB) ScanCursor() (int64, error) {
	val, closer, err := w.db.Get(keyCursor)
	if err == pebble.ErrNotFound {
		return -1, nil
	}
	if err != nil {
		return -1, err
	}
	defer closer.Close()
	if len(val) != 8 {
		return -1, nil
	}
	return int64(binary.BigEndian.Uint64(val) - heightBias), nil
}

func (w *DB) setScanCursor(b *pebble.Batch, h int64) error {
	var v [8]byte
	binary.BigEndian.PutUint64(v[:], encodeHeight(h))
	if b != nil {
		return b.Set(keyCursor, v[:], nil)
	}
	return w.db.Set(keyCursor, v[:], pebble.Sync)
}

// ListTx returns all rows in the index (newest block height first).
func (w *DB) ListTx() ([]TxRow, error) {
	it, err := w.db.NewIter(&pebble.IterOptions{
		LowerBound: prefixTx,
		UpperBound: prefixEnd(prefixTx),
	})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var out []TxRow
	// Reverse iteration yields highest biased-height (newest) first.
	for ok := it.Last(); ok; ok = it.Prev() {
		if r, valid := decodeTxRow(it.Key(), it.Value()); valid {
			out = append(out, r)
		}
	}
	return out, it.Error()
}

func maxHeight(rows []TxRow) int64 {
	maxH := int64(-1)
	for _, r := range rows {
		if r.BlockHeight > maxH {
			maxH = r.BlockHeight
		}
	}
	return maxH
}

// ReplaceFromHeight drops rows at block_height >= fromHeight and inserts rows.
func (w *DB) ReplaceFromHeight(fromHeight int64, rows []TxRow) error {
	b := w.db.NewBatch()
	defer b.Close()
	lo := txHeightBound(fromHeight)
	hi := prefixEnd(prefixTx)
	if err := b.DeleteRange(lo, hi, nil); err != nil {
		return err
	}
	for _, r := range rows {
		if err := b.Set(txKey(r), r.encodeValue(), nil); err != nil {
			return err
		}
	}
	if maxH := maxHeight(rows); maxH >= fromHeight {
		if err := w.setScanCursor(b, maxH); err != nil {
			return err
		}
	}
	return w.db.Apply(b, pebble.Sync)
}

// AppendBlock merges rows for a single connected block (idempotent per key).
func (w *DB) AppendBlock(height int64, rows []TxRow) error {
	if len(rows) == 0 {
		return w.setScanCursor(nil, height)
	}
	b := w.db.NewBatch()
	defer b.Close()
	for _, r := range rows {
		if err := b.Set(txKey(r), r.encodeValue(), nil); err != nil {
			return err
		}
	}
	if err := w.setScanCursor(b, height); err != nil {
		return err
	}
	return w.db.Apply(b, pebble.Sync)
}

// ImportLegacy migrates wallet.json scanned_txs into the index when it is empty.
func (w *DB) ImportLegacy(rows []TxRow) error {
	n, err := w.countTx()
	if err != nil {
		return err
	}
	if n > 0 || len(rows) == 0 {
		return nil
	}
	b := w.db.NewBatch()
	defer b.Close()
	for _, r := range rows {
		if err := b.Set(txKey(r), r.encodeValue(), nil); err != nil {
			return err
		}
	}
	if maxH := maxHeight(rows); maxH >= 0 {
		if err := w.setScanCursor(b, maxH); err != nil {
			return err
		}
	}
	return w.db.Apply(b, pebble.Sync)
}

func (w *DB) countTx() (int, error) {
	it, err := w.db.NewIter(&pebble.IterOptions{
		LowerBound: prefixTx,
		UpperBound: prefixEnd(prefixTx),
	})
	if err != nil {
		return 0, err
	}
	defer it.Close()
	n := 0
	for ok := it.First(); ok; ok = it.Next() {
		n++
	}
	return n, it.Error()
}
