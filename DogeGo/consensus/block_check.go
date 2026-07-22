// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"fmt"

	"dogego/chain"
	"dogego/primitives"
	"dogego/wire"
)

// CheckBlockPayload validates header/merkle, weight, size, auxpow, and per-tx rules on raw bytes before full parse.
func CheckBlockPayload(blockRaw []byte, wantBlockID [32]byte, height int64, net chain.Network) error {
	if err := wire.ValidateBlockPayload(blockRaw, wantBlockID); err != nil {
		return err
	}
	hdr, aux, err := wire.BlockHeaderAuxFromPayload(blockRaw)
	if err != nil {
		return err
	}
	if err := checkBlockAuxPowHdr(hdr, aux, height, net); err != nil {
		return err
	}
	if err := CheckBlockDuplicateTxidsRaw(blockRaw); err != nil {
		return err
	}
	if err := CheckBlockDuplicateSpendsRaw(blockRaw); err != nil {
		return err
	}
	txBytes, err := wire.BlockTxPayloadBytesRaw(blockRaw)
	if err != nil {
		return err
	}
	if txBytes > MaxBlockBaseSize {
		return fmt.Errorf("bad-blk-oversize: %d > %d", txBytes, MaxBlockBaseSize)
	}
	if err := CheckBlockWeightRaw(blockRaw); err != nil {
		return err
	}
	cb, _, err := wire.ReadTxAtIndex(blockRaw, 0)
	if err != nil {
		return fmt.Errorf("bad-blk-length: %w", err)
	}
	if !IsCoinbaseTx(cb) {
		return fmt.Errorf("bad-cb-missing")
	}
	dc := LookupConsensus(net, height)
	if height >= int64(dc.BIP34Height) {
		h, ok := CoinbaseHeightFromScript(cb.Vin[0].Script)
		if !ok || h != height {
			return fmt.Errorf("bad-cb-height: want %d", height)
		}
	}
	return wire.ForEachBlockTx(blockRaw, func(i uint32, tx *wire.Tx) error {
		if err := CheckTransaction(tx, true); err != nil {
			return fmt.Errorf("tx %d: %w", i, err)
		}
		if i > 0 && IsCoinbaseTx(tx) {
			return fmt.Errorf("bad-cb-multiple")
		}
		return nil
	})
}

// CheckBlock validates a parsed block's transactions and coinbase rules at height.
func CheckBlock(pb *wire.ParsedBlock, height int64, net chain.Network) error {
	if pb == nil || len(pb.Txs) == 0 {
		return fmt.Errorf("bad-blk-length")
	}
	if err := wire.VerifyBlockMerkle(pb); err != nil {
		return fmt.Errorf("bad-txnmrklroot: %w", err)
	}
	if !IsCoinbaseTx(pb.Txs[0]) {
		return fmt.Errorf("bad-cb-missing")
	}
	dc := LookupConsensus(net, height)
	if height >= int64(dc.BIP34Height) {
		h, ok := CoinbaseHeightFromScript(pb.Txs[0].Vin[0].Script)
		if !ok || h != height {
			return fmt.Errorf("bad-cb-height: want %d", height)
		}
	}
	if err := CheckBlockDuplicateSpends(pb); err != nil {
		return err
	}
	if err := CheckBlockDuplicateTxids(pb); err != nil {
		return err
	}
	if err := checkBlockSerializedSize(pb); err != nil {
		return err
	}
	if err := CheckBlockWeight(pb); err != nil {
		return err
	}
	if err := checkBlockAuxPowHdr(pb.Header, pb.Aux, height, net); err != nil {
		return err
	}
	return nil
}

func checkBlockAuxPow(pb *wire.ParsedBlock, height int64, net chain.Network) error {
	if pb == nil {
		return nil
	}
	return checkBlockAuxPowHdr(pb.Header, pb.Aux, height, net)
}

func checkBlockAuxPowHdr(hdr primitives.BlockHeader, aux *wire.AuxPow, height int64, net chain.Network) error {
	h80 := hdr.EncodeWire80()
	verLE := binary.LittleEndian.Uint32(h80[0:4])
	auxVer := verLE&(1<<8) != 0
	if !auxVer {
		if aux != nil {
			return fmt.Errorf("unexpected auxpow payload")
		}
		return nil
	}
	if aux == nil {
		return fmt.Errorf("missing auxpow")
	}
	dc := LookupConsensus(net, height)
	if !isLegacyVersionU(verLE) && dc.StrictChainID && chainIDFromVersion(verLE) != dc.AuxpowChainID {
		return fmt.Errorf("bad-chain-id: got %d want %d", chainIDFromVersion(verLE), dc.AuxpowChainID)
	}
	if err := checkAuxPow(h80[:], aux, dc); err != nil {
		return fmt.Errorf("auxpow: %w", err)
	}
	return nil
}

func checkBlockSerializedSize(pb *wire.ParsedBlock) error {
	sz := wire.BlockTxPayloadBytes(pb)
	if sz > MaxBlockBaseSize {
		return fmt.Errorf("bad-blk-oversize: %d > %d", sz, MaxBlockBaseSize)
	}
	return nil
}
