// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/cockroachdb/pebble"
)

// AddressBalance is one tick balance for a Dogecoin address.
type AddressBalance struct {
	Tick                string         `json:"tick"`
	Balance             string         `json:"balance"`
	TransferableBalance string         `json:"transferable_balance,omitempty"`
	TransfersCount      int            `json:"transfers_count,omitempty"`
	Transfers           []TransferUTXO `json:"transfers,omitempty"`
}

// TransferUTXO is a pending transferable DRC-20 inscription.
type TransferUTXO struct {
	Outpoint string `json:"outpoint"`
	Value    string `json:"value"`
	Tick     string `json:"tick,omitempty"`
}

// AddressHistoryRow is one indexed event for an address.
type AddressHistoryRow struct {
	ID        string `json:"id,omitempty"`
	Tick      string `json:"tick"`
	Height    int64  `json:"height"`
	Type      string `json:"type"`
	Amt       string `json:"amt,omitempty"`
	Address   string `json:"address,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	TxID      string `json:"txid,omitempty"`
	Vout      uint32 `json:"vout,omitempty"`
	Created   int64  `json:"created,omitempty"`
}

type pendingTransfer struct {
	Tick    string `json:"tick"`
	Amount  string `json:"amount"`
	Address string `json:"address"`
	ID      string `json:"id"`
}

func keyBal(addr, tick string) []byte {
	return []byte("bal/" + normAddr(addr) + "/" + strings.ToUpper(strings.TrimSpace(tick)))
}

func keyAddrHist(addr, id string) []byte {
	return []byte("ah/" + normAddr(addr) + "/" + strings.ToLower(id))
}

func keyXfer(outpoint string) []byte {
	return []byte("xf/" + strings.ToLower(strings.TrimSpace(outpoint)))
}

func keyOwn(outpoint string) []byte {
	return []byte("own/" + strings.ToLower(strings.TrimSpace(outpoint)))
}

func keyHeightIns(h int64, id string) []byte {
	return []byte(fmt.Sprintf("ih/%012d/%s", h, strings.ToLower(id)))
}

func normAddr(a string) string { return strings.TrimSpace(a) }

func opTitle(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "deploy":
		return "Deploy"
	case "mint":
		return "Mint"
	case "transfer":
		return "Transfer"
	default:
		if op == "" {
			return "Event"
		}
		return strings.ToUpper(op[:1]) + strings.ToLower(op[1:])
	}
}

func parseAmt(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0)
	}
	z, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return big.NewInt(0)
	}
	return z
}

func fmtAmt(z *big.Int) string {
	if z == nil {
		return "0"
	}
	return z.String()
}

func addAmt(cur, delta string) string {
	c := parseAmt(cur)
	c.Add(c, parseAmt(delta))
	return fmtAmt(c)
}

func subAmt(cur, delta string) string {
	c := parseAmt(cur)
	d := parseAmt(delta)
	if c.Cmp(d) < 0 {
		return "0"
	}
	c.Sub(c, d)
	return fmtAmt(c)
}

func (s *Store) loadBal(addr, tick string) AddressBalance {
	ab := AddressBalance{Tick: strings.ToUpper(tick), Balance: "0", TransferableBalance: "0"}
	if val, closer, err := s.db.Get(keyBal(addr, tick)); err == nil {
		_ = json.Unmarshal(val, &ab)
		closer.Close()
	}
	return ab
}

func (s *Store) writeBal(batch *pebble.Batch, addr string, ab AddressBalance) error {
	ab.Tick = strings.ToUpper(ab.Tick)
	ab.TransfersCount = len(ab.Transfers)
	ab.TransferableBalance = sumTransfers(ab.Transfers)
	b, err := json.Marshal(ab)
	if err != nil {
		return err
	}
	return batch.Set(keyBal(addr, ab.Tick), b, nil)
}

func sumTransfers(xs []TransferUTXO) string {
	z := big.NewInt(0)
	for _, x := range xs {
		z.Add(z, parseAmt(x.Value))
	}
	return fmtAmt(z)
}

func (s *Store) writeHist(batch *pebble.Batch, row AddressHistoryRow) error {
	hb, err := json.Marshal(row)
	if err != nil {
		return err
	}
	return batch.Set(keyAddrHist(row.Address, row.ID), hb, nil)
}

func (s *Store) applyLedger(batch *pebble.Batch, ins Inscription) error {
	if ins.Kind == "doginal" || ins.Kind == "ordinal" {
		if ins.Outpoint != "" && ins.Address != "" {
			_ = batch.Set(keyOwn(ins.Outpoint), []byte(ins.Address), nil)
		}
		return nil
	}
	if ins.Kind != "drc20" || ins.Tick == "" {
		return nil
	}
	addr := strings.TrimSpace(ins.Address)
	tick := strings.ToUpper(ins.Tick)
	row := AddressHistoryRow{
		ID:        ins.ID,
		Tick:      tick,
		Height:    ins.Height,
		TxID:      ins.TxID,
		Vout:      ins.Vout,
		Address:   addr,
		Recipient: strings.TrimSpace(ins.Recipient),
		Created:   ins.RecordedUnix,
		Amt:       ins.Amount,
	}
	switch ins.Op {
	case "deploy":
		row.Type = "Deploy"
		if addr != "" {
			_ = s.writeHist(batch, row)
		}
	case "mint":
		row.Type = "Mint"
		if addr == "" || strings.TrimSpace(ins.Amount) == "" {
			return nil
		}
		ab := s.loadBal(addr, tick)
		ab.Balance = addAmt(ab.Balance, ins.Amount)
		if err := s.writeBal(batch, addr, ab); err != nil {
			return err
		}
		return s.writeHist(batch, row)
	case "transfer":
		row.Type = "Transfer"
		if addr == "" || strings.TrimSpace(ins.Amount) == "" {
			return nil
		}
		ab := s.loadBal(addr, tick)
		// Move available balance into a transferable inscription UTXO.
		ab.Balance = subAmt(ab.Balance, ins.Amount)
		op := ins.Outpoint
		if op == "" {
			op = outpointKey(ins.TxID, ins.Vout)
		}
		ab.Transfers = append(ab.Transfers, TransferUTXO{Outpoint: op, Value: ins.Amount, Tick: tick})
		pt := pendingTransfer{Tick: tick, Amount: ins.Amount, Address: addr, ID: ins.ID}
		pb, _ := json.Marshal(pt)
		_ = batch.Set(keyXfer(op), pb, nil)
		if err := s.writeBal(batch, addr, ab); err != nil {
			return err
		}
		return s.writeHist(batch, row)
	default:
		row.Type = opTitle(ins.Op)
		if addr != "" {
			return s.writeHist(batch, row)
		}
	}
	return nil
}

// applySpends settles transferable DRC-20 / ownership when inputs spend tracked outpoints.
func (s *Store) applySpends(batch *pebble.Batch, height int64, txid string, spentOutpoints []string, recipient string, recordedUnix int64) error {
	for _, op := range spentOutpoints {
		op = strings.ToLower(strings.TrimSpace(op))
		if op == "" {
			continue
		}
		if val, closer, err := s.db.Get(keyXfer(op)); err == nil {
			var pt pendingTransfer
			_ = json.Unmarshal(val, &pt)
			closer.Close()
			_ = batch.Delete(keyXfer(op), nil)
			from := pt.Address
			if from != "" {
				ab := s.loadBal(from, pt.Tick)
				ab.Transfers = removeTransfer(ab.Transfers, op)
				_ = s.writeBal(batch, from, ab)
			}
			to := strings.TrimSpace(recipient)
			if to == "" {
				to = from
			}
			if to != "" && pt.Amount != "" {
				ab := s.loadBal(to, pt.Tick)
				ab.Balance = addAmt(ab.Balance, pt.Amount)
				_ = s.writeBal(batch, to, ab)
				_ = s.writeHist(batch, AddressHistoryRow{
					ID: txid + ":recv:" + op, Tick: pt.Tick, Height: height, Type: "Receive",
					Amt: pt.Amount, Address: to, Recipient: to, TxID: txid, Created: recordedUnix,
				})
				if from != "" && from != to {
					_ = s.writeHist(batch, AddressHistoryRow{
						ID: txid + ":send:" + op, Tick: pt.Tick, Height: height, Type: "Send",
						Amt: pt.Amount, Address: from, Recipient: to, TxID: txid, Created: recordedUnix,
					})
				}
			}
		}
		if val, closer, err := s.db.Get(keyOwn(op)); err == nil {
			prevOwner := string(val)
			closer.Close()
			_ = batch.Delete(keyOwn(op), nil)
			_ = prevOwner
			// Ownership moves to the first spendable output of this tx when known.
			if recipient != "" && txid != "" {
				_ = batch.Set(keyOwn(outpointKey(txid, 0)), []byte(recipient), nil)
			}
		}
	}
	return nil
}

func removeTransfer(xs []TransferUTXO, outpoint string) []TransferUTXO {
	out := xs[:0]
	for _, x := range xs {
		if !strings.EqualFold(x.Outpoint, outpoint) {
			out = append(out, x)
		}
	}
	return out
}

// GetAddressBalances returns token balance rows for one address.
func (s *Store) GetAddressBalances(address string, limit int) ([]AddressBalance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("store closed")
	}
	addr := normAddr(address)
	if addr == "" {
		return nil, fmt.Errorf("address required")
	}
	if limit <= 0 {
		limit = 50
	}
	prefix := []byte("bal/" + addr + "/")
	it, err := s.db.NewIter(&pebble.IterOptions{LowerBound: prefix, UpperBound: prefixEnd(prefix)})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var out []AddressBalance
	for ok := it.First(); ok && len(out) < limit; ok = it.Next() {
		var row AddressBalance
		if json.Unmarshal(it.Value(), &row) == nil && row.Tick != "" {
			row.TransfersCount = len(row.Transfers)
			if row.TransferableBalance == "" {
				row.TransferableBalance = sumTransfers(row.Transfers)
			}
			out = append(out, row)
		}
	}
	return out, nil
}

// GetAddressHistory returns recent events for an address (optional tick filter).
func (s *Store) GetAddressHistory(address, tick string, limit int) ([]AddressHistoryRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("store closed")
	}
	addr := normAddr(address)
	if addr == "" {
		return nil, fmt.Errorf("address required")
	}
	if limit <= 0 {
		limit = 40
	}
	prefix := []byte("ah/" + addr + "/")
	it, err := s.db.NewIter(&pebble.IterOptions{LowerBound: prefix, UpperBound: prefixEnd(prefix)})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var out []AddressHistoryRow
	tick = strings.ToUpper(strings.TrimSpace(tick))
	for ok := it.Last(); ok && len(out) < limit; ok = it.Prev() {
		var row AddressHistoryRow
		if json.Unmarshal(it.Value(), &row) != nil {
			continue
		}
		if tick != "" && row.Tick != tick {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

// CreditL2Balance credits an off-L1 mint to an address (experimental L2 ledger).
func (s *Store) CreditL2Balance(address, tick, amount string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("store closed")
	}
	addr := normAddr(address)
	tick = strings.ToUpper(strings.TrimSpace(tick))
	if addr == "" || tick == "" || strings.TrimSpace(amount) == "" {
		return fmt.Errorf("address, tick, amount required")
	}
	batch := s.db.NewBatch()
	defer batch.Close()
	ab := s.loadBal(addr, tick)
	ab.Balance = addAmt(ab.Balance, amount)
	if err := s.writeBal(batch, addr, ab); err != nil {
		return err
	}
	_ = s.writeHist(batch, AddressHistoryRow{
		ID: "l2:" + tick + ":" + fmtAmt(parseAmt(amount)) + ":" + fmt.Sprintf("%d", ab.TransfersCount),
		Tick: tick, Type: "MintL2", Amt: amount, Address: addr,
	})
	return batch.Commit(pebble.Sync)
}

// RollbackHeight removes index rows at height (soft reorg) and sets index tip to height-1.
func (s *Store) RollbackHeight(height int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("store closed")
	}
	prefix := []byte(fmt.Sprintf("ih/%012d/", height))
	it, err := s.db.NewIter(&pebble.IterOptions{LowerBound: prefix, UpperBound: prefixEnd(prefix)})
	if err != nil {
		return err
	}
	defer it.Close()
	batch := s.db.NewBatch()
	defer batch.Close()
	for ok := it.First(); ok; ok = it.Next() {
		k := string(it.Key())
		parts := strings.SplitN(k, "/", 3)
		if len(parts) != 3 {
			continue
		}
		id := parts[2]
		_ = batch.Delete(keyIns(id), nil)
		_ = batch.Delete(it.Key(), nil)
	}
	_ = batch.Set(keyMeta("index_height"), []byte(fmt.Sprintf("%d", height-1)), nil)
	return batch.Commit(pebble.Sync)
}
