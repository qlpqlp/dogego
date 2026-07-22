// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// ValidateStoredHeaders re-checks headers already in the journal from startHeight through endHeight
// (inclusive). It enforces prev-hash linkage, contextual Digishield nBits, MTP, and block-version
// gates. Non-auxpow headers also get scrypt PoW checks. Auxpow-version headers require headers_aux.bin
// with a valid merge-mining proof and parent scrypt PoW.
func ValidateStoredHeaders(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, startHeight, endHeight, nowUnix int64) error {
	if startHeight < 0 || endHeight < startHeight {
		return fmt.Errorf("invalid height range %d..%d", startHeight, endHeight)
	}
	tipBefore, err := j.TipHeight()
	if err != nil {
		return err
	}
	if endHeight > tipBefore {
		return fmt.Errorf("end height %d beyond tip %d", endHeight, tipBefore)
	}

	var prevTip [32]byte
	if startHeight > 0 {
		prev80, err := j.ReadHeaderAt(startHeight - 1)
		if err != nil {
			return err
		}
		prevTip = pow.BlockHashLE(prev80)
	}

	v := &batchView{j: j, tip0: startHeight - 1}
	var auxData []byte
	var auxOffs []int64
	if aux != nil {
		needsAux, err := StoredHeaderRangeNeedsAux(j, startHeight, endHeight)
		if err != nil {
			return err
		}
		if needsAux {
			auxData, auxOffs, err = aux.SnapshotForBackfill()
			if err != nil {
				return err
			}
		}
	}
	for h := startHeight; h <= endHeight; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return err
		}
		if len(h80) != 80 {
			return fmt.Errorf("height %d: bad header len", h)
		}
		dc := LookupConsensus(p.Net, h)
		verLE := nVersionLE(h80)

		if !dc.AllowLegacyBlocks && isLegacyVersionU(verLE) {
			return fmt.Errorf("height %d: legacy block after auxpow start", h)
		}
		if dc.AllowLegacyBlocks && isAuxpowVersionU(verLE) {
			return fmt.Errorf("height %d: auxpow before activation", h)
		}
		if !isLegacyVersionU(verLE) && dc.StrictChainID && chainIDFromVersion(verLE) != dc.AuxpowChainID {
			return fmt.Errorf("height %d: wrong auxpow chain id", h)
		}

		var prev [32]byte
		copy(prev[:], h80[4:36])
		if h == 0 {
			var z [32]byte
			if prev != z {
				return fmt.Errorf("height 0: genesis prev not zero")
			}
		} else if prev != prevTip {
			return fmt.Errorf("height %d: bad prev", h)
		}

		candTime := binary.LittleEndian.Uint32(h80[68:72])
		gotBits := binary.LittleEndian.Uint32(h80[72:76])

		if isAuxpowVersionU(verLE) {
			if aux == nil {
				return fmt.Errorf("height %d: missing auxpow (no headers_aux.bin)", h)
			}
			var auxBytes []byte
			var rerr error
			if len(auxOffs) > 0 && h < int64(len(auxOffs)) {
				auxBytes, rerr = store.DecodeAuxRecordAt(auxData, auxOffs, h)
			} else {
				auxBytes, rerr = aux.ReadAt(h)
			}
			if rerr != nil {
				return fmt.Errorf("height %d aux read: %w", h, rerr)
			}
			if len(auxBytes) == 0 {
				return fmt.Errorf("height %d: missing auxpow payload (headers_aux.bin empty at this height - wait for block IBD or run aux backfill)", h)
			}
			ap, err := wire.ReadAuxPow(bytes.NewReader(auxBytes))
			if err != nil {
				return fmt.Errorf("height %d aux decode: %w", h, err)
			}
			if err := checkAuxPow(h80, ap, dc); err != nil {
				return fmt.Errorf("height %d aux: %w", h, err)
			}
		} else {
			if !p.RelaxedPoW {
				ph := pow.ScryptHashLE(h80)
				if err := pow.CheckProofOfWorkLE(ph, gotBits, PowLimitHex); err != nil {
					return fmt.Errorf("height %d pow: %w", h, err)
				}
			}
		}

		if h > 0 {
			expBits, err := getNextWorkRequired(v, h-1, candTime, LookupConsensus(p.Net, h-1))
			if err != nil {
				return fmt.Errorf("height %d nextwork: %w", h, err)
			}
			if gotBits != expBits {
				return fmt.Errorf("height %d: bad nBits want 0x%x got 0x%x", h, expBits, gotBits)
			}
		}

		if h > 0 {
			mtp, err := medianTimePast(v, h-1)
			if err != nil {
				return err
			}
			if int64(candTime) <= mtp {
				return fmt.Errorf("height %d: time too old vs MTP", h)
			}
			if int64(candTime) > nowUnix+2*60*60 {
				return fmt.Errorf("height %d: time too new", h)
			}
		}

		base := BlockBaseVersion(verLE)
		if (base < 3 && h >= int64(dc.BIP66Height)) || (base < 4 && h >= int64(dc.BIP65Height)) {
			return fmt.Errorf("height %d: obsolete block version 0x%x", h, verLE)
		}

		prevTip = pow.BlockHashLE(h80)
		v.times = append(v.times, candTime)
		v.bits = append(v.bits, gotBits)
	}
	return nil
}
