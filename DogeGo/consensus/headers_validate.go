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
	"slices"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func nVersionLE(h80 []byte) uint32 {
	return binary.LittleEndian.Uint32(h80[0:4])
}

// chainIDFromVersion returns the merge-mining chain ID in bits 16..29 (same as Dogecoin Core
// CPureBlockHeader::GetChainId). The wire field is unsigned; using uint32 avoids Go's signed
// right-shift sign extension on negative int32 nVersion values (BIP9 / high-bit layouts).
func chainIDFromVersion(verLE uint32) int32 {
	return int32(verLE >> 16)
}

func isLegacyVersionU(verLE uint32) bool {
	v := int32(verLE)
	return v == 1 || (v == 2 && chainIDFromVersion(verLE) == 0)
}

func isAuxpowVersionU(verLE uint32) bool {
	return verLE&(1<<8) != 0
}

func networkLabel(net chain.Network) string {
	switch net {
	case chain.MainnetDogecoin:
		return "mainnet"
	case chain.RebootTestnet:
		return "testnet"
	default:
		return fmt.Sprintf("network(%d)", int(net))
	}
}

// ValidateHeaders performs Core-like checks for a batch of decoded P2P headers (prev hash, difficulty, auxpow, PoW).
func ValidateHeaders(j *store.HeaderJournal, p chain.Params, decoded []wire.DecodedHeader, nowUnix int64) error {
	tipBefore, err := j.TipHeight()
	if err != nil {
		return err
	}
	prevTip, err := j.LastTipHash()
	if err != nil {
		return err
	}
	cur := prevTip
	v := &batchView{j: j, tip0: tipBefore}
	for i := range decoded {
		d := decoded[i]
		h80 := d.Header80
		if len(h80) != 80 {
			return fmt.Errorf("header %d: bad len", i)
		}
		height := tipBefore + int64(i) + 1
		dc := LookupConsensus(p.Net, height)
		verLE := nVersionLE(h80)

		if !dc.AllowLegacyBlocks && isLegacyVersionU(verLE) {
			return fmt.Errorf("header batch index %d (chain height %d on %s): legacy scrypt header after auxpow activation (mainnet merge-mining from height 371337); if headers.bin is from another network or a bad sync, delete headers.bin and restart or try another peer",
				i, height, networkLabel(p.Net))
		}
		if dc.AllowLegacyBlocks && isAuxpowVersionU(verLE) {
			return fmt.Errorf("header batch index %d (chain height %d on %s): auxpow header before activation (mainnet legacy scrypt through height 371336)",
				i, height, networkLabel(p.Net))
		}
		if !isLegacyVersionU(verLE) && dc.StrictChainID && chainIDFromVersion(verLE) != dc.AuxpowChainID {
			return fmt.Errorf("header %d at chain height %d (%s): wrong auxpow chain id %d want %d (nVersion LE=0x%08x; if you meant reboot testnet use -network testnet; if you switched networks delete headers.bin; or try another -peer)",
				i, height, networkLabel(p.Net), chainIDFromVersion(verLE), dc.AuxpowChainID, verLE)
		}

		var prev [32]byte
		copy(prev[:], h80[4:36])
		if prev != cur {
			if i == 0 && tipBefore == 0 {
				return fmt.Errorf("header %d: bad prev (peer chain does not extend local genesis; check -network matches datadir, delete headers.bin if you switched chains, or try another -peer)", i)
			}
			return fmt.Errorf("header %d: bad prev", i)
		}

		candTime := binary.LittleEndian.Uint32(h80[68:72])
		// Core only requires nTime > median-time-past of the parent (not monotonic vs parent's nTime).
		// A strict parent-nTime check rejects valid early-chain headers in a single P2P batch.
		if err := checkHeaderCheckpoint(p.Net, height, h80); err != nil {
			return fmt.Errorf("header batch index %d: %w", i, err)
		}

		if isAuxpowVersionU(verLE) {
			if d.Aux == nil {
				return fmt.Errorf("header %d: missing auxpow", i)
			}
			if err := checkAuxPow(h80, d.Aux, dc); err != nil {
				return fmt.Errorf("header %d aux: %w", i, err)
			}
		} else {
			if d.Aux != nil {
				return fmt.Errorf("header %d: unexpected auxpow", i)
			}
			bits := binary.LittleEndian.Uint32(h80[72:76])
			if !p.RelaxedPoW {
				ph := pow.ScryptHashLE(h80)
				if err := pow.CheckProofOfWorkLE(ph, bits, PowLimitHex); err != nil {
					return fmt.Errorf("header %d pow: %w", i, err)
				}
			}
		}

		prevH := height - 1
		if prevH >= 0 {
			// Core uses consensus params at pindexLast (prevH), not the child height.
			expBits, err := getNextWorkRequired(v, prevH, candTime, LookupConsensus(p.Net, prevH))
			if err != nil {
				return fmt.Errorf("header %d nextwork: %w", i, err)
			}
			gotBits := binary.LittleEndian.Uint32(h80[72:76])
			if gotBits != expBits {
				return fmt.Errorf("header batch index %d (chain height %d on %s): bad nBits want 0x%x got 0x%x - often stale header timestamps after partial sync (DogeGo auto-rewinds on large nTime jumps); else try another peer or reboot testnet (-network testnet)",
					i, height, networkLabel(p.Net), expBits, gotBits)
			}
		}

		if prevH >= 0 {
			mtp, err := medianTimePast(v, prevH)
			if err != nil {
				return err
			}
			if int64(candTime) <= mtp {
				return fmt.Errorf("header %d: time too old vs MTP", i)
			}
			if int64(candTime) > nowUnix+2*60*60 {
				return fmt.Errorf("header %d: time too new", i)
			}
		}

		base := BlockBaseVersion(verLE)
		if (base < 3 && height >= int64(dc.BIP66Height)) || (base < 4 && height >= int64(dc.BIP65Height)) {
			return fmt.Errorf("header %d: obsolete block version 0x%x at height %d", i, verLE, height)
		}

		cur = pow.BlockHashLE(h80)
		v.times = append(v.times, candTime)
		v.bits = append(v.bits, binary.LittleEndian.Uint32(h80[72:76]))
	}
	return nil
}

func checkAuxPow(child80 []byte, a *wire.AuxPow, dc DogeConsensus) error {
	if a.MerkleIndex != 0 {
		return fmt.Errorf("auxpow not coinbase")
	}
	pverLE := nVersionLE(a.ParentHeader80[:])
	if pverLE == 0 {
		return fmt.Errorf("aux parent has zero version")
	}
	if isAuxpowVersionU(pverLE) {
		return fmt.Errorf("aux parent must not be auxpow block")
	}
	var zero32 [32]byte
	if bytes.Equal(a.ParentHeader80[4:36], zero32[:]) {
		return fmt.Errorf("aux parent has zero prev block hash")
	}
	parentTime := binary.LittleEndian.Uint32(a.ParentHeader80[68:72])
	if parentTime == 0 {
		return fmt.Errorf("aux parent has zero timestamp")
	}
	childTime := binary.LittleEndian.Uint32(child80[68:72])
	if childTime != 0 && parentTime > childTime+7200 {
		return fmt.Errorf("aux parent timestamp too far ahead of child")
	}
	// Core CAuxPow::check: reject only when the parent header encodes our chain ID (0x62).
	// Parent may use any other chain ID (e.g. Litecoin or other merge-mining parents).
	if dc.StrictChainID && chainIDFromVersion(pverLE) == dc.AuxpowChainID {
		return fmt.Errorf("aux parent has same chain id")
	}
	if len(a.Coinbase.Vin) != 1 {
		return fmt.Errorf("aux parent coinbase must have exactly one input")
	}
	if !IsNullOutpoint(&a.Coinbase.Vin[0]) {
		return fmt.Errorf("aux parent coinbase must not spend outputs")
	}
	if !IsCoinbaseTx(a.Coinbase) {
		return fmt.Errorf("aux parent coinbase is not coinbase")
	}
	if err := CheckTransaction(a.Coinbase, true); err != nil {
		return fmt.Errorf("aux parent coinbase invalid: %w", err)
	}
	// Core CAuxPow::check does not compare CMerkleTx::hashBlock to parentBlock.GetHash(); merkle
	// branches + coinbase script prove inclusion. Some valid mainnet aux headers carry a stale
	// hashBlock on the wire while still passing Core validation.
	if len(a.MerkleBranch) > 30 {
		return fmt.Errorf("merkle branch too long")
	}
	if len(a.ChainBranch) > 30 {
		return fmt.Errorf("chain merkle branch too long")
	}
	merkleHeight := uint(len(a.ChainBranch))
	if merkleHeight > 0 {
		limit := uint(1) << merkleHeight
		if a.ChainIndex < 0 || uint(a.ChainIndex) >= limit {
			return fmt.Errorf("aux chain index out of range")
		}
	}
	if raw, err := a.Coinbase.Serialize(); err == nil && len(raw) > MaxBlockBaseSize {
		return fmt.Errorf("aux parent coinbase too large")
	}
	childBits := binary.LittleEndian.Uint32(child80[72:76])
	if childBits == 0 {
		return fmt.Errorf("aux child has zero nBits")
	}
	parentBits := binary.LittleEndian.Uint32(a.ParentHeader80[72:76])
	if parentBits == 0 {
		return fmt.Errorf("aux parent has zero nBits")
	}
	if bytes.Equal(a.ParentHeader80[36:68], zero32[:]) {
		return fmt.Errorf("aux parent has zero merkle root")
	}
	childHash := pow.BlockHashLE(child80)
	if bytes.Equal(a.ParentHeader80[4:36], childHash[:]) {
		return fmt.Errorf("aux parent prev hash equals child block hash")
	}
	parentID := pow.BlockHashLE(a.ParentHeader80[:])
	if parentID == childHash {
		return fmt.Errorf("aux parent block id equals child block id")
	}
	nRoot := pow.CheckMerkleBranch(childHash, a.ChainBranch, a.ChainIndex)
	rootLE := nRoot[:]
	rootScript := slices.Clone(rootLE)
	slices.Reverse(rootScript)

	txh := a.Coinbase.TxHash()
	merkleCalc := pow.CheckMerkleBranch(txh, a.MerkleBranch, a.MerkleIndex)
	var wantMerkle [32]byte
	copy(wantMerkle[:], a.ParentHeader80[36:68])
	if merkleCalc != wantMerkle {
		return fmt.Errorf("aux merkle root mismatch")
	}
	if len(a.Coinbase.Vin) < 1 {
		return fmt.Errorf("aux coinbase no inputs")
	}
	sig := a.Coinbase.Vin[0].Script
	if !bytes.Contains(sig, rootScript) {
		return fmt.Errorf("aux missing chain merkle root in coinbase")
	}
	pcRoot := bytes.Index(sig, rootScript)
	if pcRoot < 0 {
		return fmt.Errorf("no root in script")
	}
	if bytes.Count(sig, rootScript) > 1 {
		return fmt.Errorf("duplicate chain merkle root in parent coinbase")
	}
	pcHead := bytes.Index(sig, wire.MergedMiningHeader)
	if pcHead != -1 {
		if bytes.Contains(sig[pcHead+1:], wire.MergedMiningHeader) {
			return fmt.Errorf("multiple merged mining headers")
		}
		if pcHead+len(wire.MergedMiningHeader) != pcRoot {
			return fmt.Errorf("merged mining header not just before chain merkle root")
		}
	} else if pcRoot > 20 {
		return fmt.Errorf("aux POW chain merkle root must start in the first 20 bytes of the parent coinbase")
	}
	pc := pcRoot
	pc += len(rootScript)
	if len(sig)-pc < 8 {
		return fmt.Errorf("missing size/nonce after root")
	}
	nSize := binary.LittleEndian.Uint32(sig[pc : pc+4])
	if nSize != uint32(1)<<merkleHeight {
		return fmt.Errorf("aux merkle size mismatch")
	}
	nNonce := binary.LittleEndian.Uint32(sig[pc+4 : pc+8])
	if int(a.ChainIndex) != pow.AuxExpectedIndex(nNonce, chainIDFromVersion(nVersionLE(child80)), merkleHeight) {
		return fmt.Errorf("aux wrong chain index")
	}
	// Core CheckAuxPowProofOfWork: parent scrypt hash must meet the child header nBits target.
	parentPow := pow.ScryptHashLE(a.ParentHeader80[:])
	if err := pow.CheckProofOfWorkLE(parentPow, childBits, PowLimitHex); err != nil {
		return fmt.Errorf("aux parent pow: %w", err)
	}
	return nil
}
