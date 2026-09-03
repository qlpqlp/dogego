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
		if ins, ok := DetectInscriptionFromWitness(height, txid, uint32(vin), in.Witness); ok {
			ins.Address = sender
			ins.Recipient = recipient
			ins.RecordedUnix = now
			if len(tx.Vout) > 0 {
				ins.Vout = 0
				ins.Outpoint = outpointKey(txid, 0)
			}
			if err := st.PutInscription(ins); err == nil {
				n++
			}
			continue
		}
		if nP2SH := e.indexP2SHInput(st, height, txid, uint32(vin), in, tx, sender, recipient, now); nP2SH > 0 {
			n += nP2SH
		}
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

// indexP2SHInput indexes apezord/booktoshi P2SH doginals revealed in scriptSig.
func (e *Extension) indexP2SHInput(st *Store, height int64, txid string, vin uint32, in wire.TxIn, tx *wire.Tx, sender, recipient string, now int64) int {
	if len(in.Script) == 0 {
		return 0
	}
	partial, ok := ExtractP2SHInscriptionPartial(in.Script)
	if !ok {
		return 0
	}

	var asm p2shAssembly
	continued := false
	if !partial.StartsOrd && in.PrevIdx != 0xffffffff {
		prevOP := outpointKey(TxDisplayHex(in.PrevHash), in.PrevIdx)
		if prev, found, err := st.TakeP2SHPending(prevOP); err == nil && found {
			asm = prev
			continued = true
		}
	}
	if partial.StartsOrd {
		asm = p2shAssembly{
			StartTxID:   txid,
			StartHeight: height,
			Vin:         vin,
		}
	} else if !continued {
		return 0
	}

	trial := asm
	if !trial.applyPartial(partial) {
		if continued && in.PrevIdx != 0xffffffff {
			_ = st.PutP2SHPending(outpointKey(TxDisplayHex(in.PrevHash), in.PrevIdx), asm)
		}
		return 0
	}
	asm = trial

	if asm.complete() {
		ins := InscriptionFromBody(asm.StartHeight, asm.StartTxID, asm.Vin, asm.ContentType, asm.Data, "p2sh")
		// Prefer the reveal/completion tx for lookup convenience.
		ins.TxID = txid
		ins.Height = height
		ins.ID = fmtInscriptionIDVin(height, txid, vin)
		ins.Address = sender
		ins.Recipient = recipient
		ins.RecordedUnix = now
		if len(tx.Vout) > 0 {
			ins.Vout = 0
			ins.Outpoint = outpointKey(txid, 0)
		}
		if err := st.PutInscription(ins); err != nil {
			return 0
		}
		return 1
	}

	// Incomplete: chain continues via first output (apezord mint path).
	if len(tx.Vout) == 0 {
		return 0
	}
	nextOP := outpointKey(txid, 0)
	_ = st.PutP2SHPending(nextOP, asm)
	return 0
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
