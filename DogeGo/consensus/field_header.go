// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/pow"
	"dogego/wire"
)

// verifyFieldHeaderPoW checks scrypt or auxpow PoW for a committed mainnet field header80.
func verifyFieldHeaderPoW(net chain.Network, height int64, h80 []byte) error {
	if len(h80) != 80 {
		return fmt.Errorf("header len %d", len(h80))
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return err
	}
	dc := LookupConsensus(net, height)
	verLE := nVersionLE(h80)
	if isAuxpowVersionU(verLE) {
		return fmt.Errorf("field_header PoW at height %d requires aux journal (use TestCoreMainnetFieldHeaderPoW or mainnet_field_auxpow.json)", height)
	}
	if dc.AllowLegacyBlocks && isAuxpowVersionU(verLE) {
		return fmt.Errorf("unexpected auxpow layout")
	}
	bits := binary.LittleEndian.Uint32(h80[72:76])
	ph := pow.ScryptHashLE(h80)
	if err := pow.CheckProofOfWorkLE(ph, bits, PowLimitHex); err != nil {
		return fmt.Errorf("height %d pow: %w", height, err)
	}
	if p.RelaxedPoW {
		return fmt.Errorf("field_header on relaxed network")
	}
	return nil
}

// verifyCommittedFieldHeader validates a committed field_header row (checkpoint, auxpow fixture, or legacy scrypt).
func verifyCommittedFieldHeader(net chain.Network, height int64, h80 []byte) error {
	if len(h80) != 80 {
		return fmt.Errorf("header len %d", len(h80))
	}
	if _, ok := chain.CheckpointHashAt(net, height); ok {
		return checkHeaderCheckpoint(net, height, h80)
	}
	if isAuxpowVersionU(nVersionLE(h80)) {
		return verifyCommittedFieldHeaderAuxPow(net, height, h80)
	}
	return verifyFieldHeaderPoW(net, height, h80)
}

func verifyCommittedFieldHeaderAuxPow(net chain.Network, height int64, h80 []byte) error {
	entries, err := LoadMainnetFieldAuxpowEntries()
	if err != nil {
		return fmt.Errorf("auxpow fixture: %w", err)
	}
	wantHx := strings.ToUpper(hex.EncodeToString(h80))
	for _, e := range entries {
		if e.Height != height {
			continue
		}
		gotHx := strings.ToUpper(strings.TrimSpace(e.HeaderHex))
		if gotHx != wantHx {
			return fmt.Errorf("header_hex mismatch with mainnet_field_auxpow.json at height %d", height)
		}
		auxB, err := hex.DecodeString(strings.TrimSpace(e.AuxHex))
		if err != nil || len(auxB) == 0 {
			return fmt.Errorf("aux_hex at height %d: %v len=%d", height, err, len(auxB))
		}
		ap, err := wire.ReadAuxPow(bytes.NewReader(auxB))
		if err != nil {
			return err
		}
		dc := LookupConsensus(net, height)
		return checkAuxPow(h80, ap, dc)
	}
	return fmt.Errorf("field_header PoW at height %d requires aux journal or mainnet_field_auxpow.json row", height)
}
