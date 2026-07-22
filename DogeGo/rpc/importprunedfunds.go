// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"strings"

	"dogego/chain"
	"dogego/pow"
	"dogego/primitives"
	"dogego/wire"
)

// merkleProofChainContext returns block height and display hash when proof is valid on the header chain.
func merkleProofChainContext(j HeaderJournal, proof []byte) (height int64, blockHash string, matches [][32]byte, err error) {
	if j == nil {
		return 0, "", nil, errImportPrunedNoChain
	}
	if len(proof) < 84 {
		return 0, "", nil, errImportPrunedInvalidProof
	}
	header80, pmt, err := wire.ParseMerkleBlockProof(proof)
	if err != nil {
		return 0, "", nil, errImportPrunedInvalidProof
	}
	var hdr primitives.BlockHeader
	if err := hdr.DecodeWire80(header80); err != nil {
		return 0, "", nil, errImportPrunedInvalidProof
	}
	root, matchList, _, ok := pmt.ExtractMatches()
	if !ok || root != hdr.MerkleRoot {
		return 0, "", nil, errImportPrunedInvalidProof
	}
	blockID := pow.BlockHashLE(header80)
	height, err = j.HeightByDisplayHash(pow.LEUint256DisplayHex(blockID[:]))
	if err != nil {
		return 0, "", nil, errImportPrunedBlockNotInChain
	}
	stored, err := j.ReadHeaderAt(height)
	if err != nil || !bytes.Equal(stored, header80) {
		return 0, "", nil, errImportPrunedBlockNotInChain
	}
	return height, pow.LEUint256DisplayHex(blockID[:]), matchList, nil
}

var (
	errImportPrunedNoChain         = &importPrunedErr{"No chain data"}
	errImportPrunedInvalidProof    = &importPrunedErr{"Invalid or incomplete proof of work"}
	errImportPrunedBlockNotInChain = &importPrunedErr{"Block not found in chain"}
	errImportPrunedTxNotInProof    = &importPrunedErr{"Transaction not in proof"}
	errImportPrunedNoWalletCredits = &importPrunedErr{"No wallet outputs in transaction"}
)

type importPrunedErr struct{ msg string }

func (e *importPrunedErr) Error() string { return e.msg }

// execImportPrunedFunds matches Core importprunedfunds: verify CMerkleBlock proof + credit wallet watch outputs.
func execImportPrunedFunds(chainName string, paths *DataPaths, j HeaderJournal, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var hexStr string
	if err := json.Unmarshal(params[0], &hexStr); err != nil {
		return nil, -8, "importprunedfunds: rawtransaction must be a string"
	}
	hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	txb, err := hex.DecodeString(hexStr)
	if err != nil || len(txb) == 0 {
		return nil, -22, "TX decode failed"
	}
	tx, err := wire.DeserializeTx(txb)
	if err != nil {
		return nil, -22, "TX decode failed"
	}
	var proofHex string
	if err := json.Unmarshal(params[1], &proofHex); err != nil {
		return nil, -8, "importprunedfunds: txoutproof must be a string"
	}
	proofHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(proofHex), "0x"))
	if len(proofHex) < 2 || len(proofHex)%2 != 0 {
		return nil, -8, "importprunedfunds: invalid txoutproof"
	}
	proof, err := hex.DecodeString(proofHex)
	if err != nil {
		return nil, -8, "importprunedfunds: invalid txoutproof"
	}

	height, blockHash, matches, err := merkleProofChainContext(j, proof)
	if err != nil {
		if pe, ok := err.(*importPrunedErr); ok {
			return nil, -5, "importprunedfunds: "+pe.msg
		}
		return nil, -5, "importprunedfunds: "+err.Error()
	}
	want := tx.TxHash()
	inProof := false
	for _, th := range matches {
		if th == want {
			inProof = true
			break
		}
	}
	if !inProof {
		return nil, -5, "importprunedfunds: "+errImportPrunedTxNotInProof.msg
	}

	if paths == nil || paths.WalletImportPrunedReceive == nil {
		return nil, 0, ""
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}
	tip, _, _ := activeChainFromJournal(j, nil, paths)
	var credited int
	for i, out := range tx.Vout {
		if out.Value <= 0 {
			continue
		}
		if paths.WalletOwnsScript == nil || !paths.WalletOwnsScript(out.PkScript) {
			continue
		}
		if err := paths.WalletImportPrunedReceive(txidToRPC(want), height, blockHash, uint32(i), out.Value, out.PkScript); err != nil {
			return nil, -1, "importprunedfunds: "+err.Error()
		}
		credited++
		_ = p
	}
	if credited == 0 {
		return nil, -5, "importprunedfunds: "+errImportPrunedNoWalletCredits.msg
	}
	_ = tip
	return nil, 0, ""
}
