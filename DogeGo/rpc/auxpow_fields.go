// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/hex"

	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func auxPowForBlock(pb *wire.ParsedBlock, aux *store.HeaderAuxJournal, height int64) *wire.AuxPow {
	if pb != nil && pb.Aux != nil {
		return pb.Aux
	}
	blob := auxPowBlob(aux, height)
	if len(blob) == 0 {
		return nil
	}
	r := bytes.NewReader(blob)
	ap, err := wire.ReadAuxPow(r)
	if err != nil {
		return nil
	}
	return ap
}

func auxPowBlob(aux *store.HeaderAuxJournal, height int64) []byte {
	if aux == nil || height < 0 {
		return nil
	}
	blob, err := aux.ReadAt(height)
	if err != nil || len(blob) == 0 {
		return nil
	}
	return blob
}

// auxpowToJSON matches Dogecoin Core AuxpowToJSON (blockchain.cpp) for getblock / getblockheader.
func auxpowToJSON(ap *wire.AuxPow) (map[string]interface{}, error) {
	if ap == nil || ap.Coinbase == nil {
		return nil, nil
	}
	txObj, err := txToRPCJSON(ap.Coinbase)
	if err != nil {
		return nil, err
	}
	merkle := make([]interface{}, len(ap.MerkleBranch))
	for i, h := range ap.MerkleBranch {
		merkle[i] = pow.LEUint256DisplayHex(h[:])
	}
	chainMerkle := make([]interface{}, len(ap.ChainBranch))
	for i, h := range ap.ChainBranch {
		chainMerkle[i] = pow.LEUint256DisplayHex(h[:])
	}
	return map[string]interface{}{
		"tx":                 txObj,
		"index":              ap.MerkleIndex,
		"chainindex":         ap.ChainIndex,
		"merklebranch":       merkle,
		"chainmerklebranch":  chainMerkle,
		"parentblock":        hex.EncodeToString(ap.ParentHeader80[:]),
	}, nil
}

func attachAuxPowField(m map[string]interface{}, pb *wire.ParsedBlock, aux *store.HeaderAuxJournal, height int64) {
	ap := auxPowForBlock(pb, aux, height)
	if ap == nil {
		return
	}
	obj, err := auxpowToJSON(ap)
	if err != nil || obj == nil {
		return
	}
	m["auxpow"] = obj
}
