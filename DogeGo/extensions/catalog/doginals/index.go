// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"fmt"
	"time"

	"dogego/extensions"
	"dogego/wire"

	"github.com/cockroachdb/pebble"
)

func (e *Extension) indexHeight(host extensions.Host, height int64) error {
	st, err := e.storeOrErr()
	if err != nil {
		return err
	}
	raw, err := host.GetRawBlockByHeight(height)
	if err != nil || len(raw) < 80 {
		return err
	}
	n := 0
	_ = wire.ForEachBlockTx(raw, func(_ uint32, tx *wire.Tx) error {
		n += e.indexTx(host, st, height, tx)
		return nil
	})
	_ = st.SetIndexHeight(height)
	if n > 0 && host != nil {
		host.Log(fmt.Sprintf("doginals: height %d indexed %d inscription(s)", height, n))
	}
	return nil
}

func (e *Extension) indexTx(host extensions.Host, st *Store, height int64, tx *wire.Tx) int {
	if tx == nil {
		return 0
	}
	txid := TxDisplayHex(tx.TxHash())
	sender := resolveInputAddress(host, tx)
	recipient := firstSpendableOutputAddress(host, tx)
	now := time.Now().Unix()

	var spent []string
	for _, in := range tx.Vin {
		if in.PrevIdx == 0xffffffff {
			continue
		}
		spent = append(spent, outpointKey(TxDisplayHex(in.PrevHash), in.PrevIdx))
	}
	_ = st.ApplySpends(height, txid, spent, recipient, now)

	n := 0
	for vin, in := range tx.Vin {
		ins, ok := DetectInscriptionFromWitness(height, txid, uint32(vin), in.Witness)
		if !ok {
			continue
		}
		ins.Address = sender
		ins.Recipient = recipient
		ins.RecordedUnix = now
		if len(tx.Vout) > 0 {
			ins.Vout = 0
			ins.Outpoint = outpointKey(txid, 0)
		}
		if err := st.PutInscription(ins); err != nil {
			continue
		}
		n++
	}
	for vout, o := range tx.Vout {
		ins, ok := DetectInscriptionFromOutput(height, txid, uint32(vout), o)
		if !ok {
			continue
		}
		ins.Address = sender
		ins.Recipient = recipient
		ins.RecordedUnix = now
		ins.Outpoint = outpointKey(txid, uint32(vout))
		if err := st.PutInscription(ins); err != nil {
			continue
		}
		n++
	}
	return n
}

// ApplySpends settles transferable DRC-20 when inputs spend tracked outpoints.
func (s *Store) ApplySpends(height int64, txid string, spent []string, recipient string, recordedUnix int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return fmt.Errorf("store closed")
	}
	batch := s.db.NewBatch()
	defer batch.Close()
	if err := s.applySpends(batch, height, txid, spent, recipient, recordedUnix); err != nil {
		return err
	}
	return batch.Commit(pebble.Sync)
}
