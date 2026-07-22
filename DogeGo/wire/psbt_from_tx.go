// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import "fmt"

// NewPsbtFromTx builds an empty PSBT wrapper around an unsigned legacy transaction.
func NewPsbtFromTx(tx *Tx) (*Psbt, error) {
	if tx == nil {
		return nil, fmt.Errorf("psbt: nil transaction")
	}
	if tx.HasWitness() {
		return nil, fmt.Errorf("psbt: witness transactions are not supported")
	}
	p := &Psbt{
		UnsignedTx: tx,
		Global: []PsbtKeyValue{{
			Type:  PsbtGlobalUnsignedTx,
			Value: tx.SerializeForHash(),
		}},
	}
	for range tx.Vin {
		p.Inputs = append(p.Inputs, nil)
	}
	for range tx.Vout {
		p.Outputs = append(p.Outputs, nil)
	}
	return p, nil
}
