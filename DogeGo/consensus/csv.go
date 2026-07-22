// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/chain"
	"dogego/wire"
)

const opCheckSequenceVerify = 0xb2

// CSVActiveAt reports whether BIP68/112/113 are active at height on net.
func CSVActiveAt(height int64, net chain.Network) bool {
	if height < 0 {
		return false
	}
	return height >= int64(LookupConsensus(net, height).CSVHeight)
}

// CheckSequence implements BIP112 OP_CHECKSEQUENCEVERIFY (Core TransactionSignatureChecker::CheckSequence).
func CheckSequence(tx *wire.Tx, inputIdx int, sequence int64) error {
	if tx == nil {
		return fmt.Errorf("csv: nil transaction")
	}
	if inputIdx < 0 || inputIdx >= len(tx.Vin) {
		return fmt.Errorf("csv: bad input index")
	}
	if uint32(tx.Version) < 2 {
		return fmt.Errorf("csv: unsatisfied sequence")
	}
	inSeq := tx.Vin[inputIdx].Sequence
	if inSeq&wire.SequenceLocktimeDisableFlag != 0 {
		return fmt.Errorf("csv: sequence disable flag set")
	}
	mask := int64(wire.SequenceLocktimeTypeFlag | wire.SequenceLocktimeMask)
	txMasked := int64(inSeq) & mask
	opMasked := sequence & mask
	heightBased := txMasked < int64(wire.SequenceLocktimeTypeFlag)
	opHeightBased := opMasked < int64(wire.SequenceLocktimeTypeFlag)
	if heightBased != opHeightBased {
		return fmt.Errorf("csv: sequence type mismatch")
	}
	if opMasked > txMasked {
		return fmt.Errorf("csv: unsatisfied sequence")
	}
	return nil
}

// parseCSVP2PKHRedeem decodes <sequence> OP_CHECKSEQUENCEVERIFY OP_DROP OP_DUP OP_HASH160 <20> OP_EQUALVERIFY OP_CHECKSIG.
func parseCSVP2PKHRedeem(script []byte) (sequence int64, p2pkh []byte, err error) {
	if len(script) < 26 {
		return 0, nil, fmt.Errorf("csv: script too short")
	}
	seq, i, err := ReadScriptNumPush(script, 0)
	if err != nil {
		return 0, nil, err
	}
	if i >= len(script) || script[i] != opCheckSequenceVerify {
		return 0, nil, fmt.Errorf("csv: missing OP_CHECKSEQUENCEVERIFY")
	}
	i++
	want := []byte{0x75, 0x76, 0xa9, 0x14}
	if i+len(want)+20+2 > len(script) {
		return 0, nil, fmt.Errorf("csv: truncated p2pkh tail")
	}
	if string(script[i:i+len(want)]) != string(want) {
		return 0, nil, fmt.Errorf("csv: unexpected tail prefix")
	}
	i += len(want)
	var h160 [20]byte
	copy(h160[:], script[i:i+20])
	i += 20
	if script[i] != 0x88 || script[i+1] != 0xac || i+2 != len(script) {
		return 0, nil, fmt.Errorf("csv: bad equalverify/checksig")
	}
	p2pkh = append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	p2pkh = append(p2pkh, 0x88, 0xac)
	return seq, p2pkh, nil
}

func isCSVP2PKHRedeem(script []byte) bool {
	_, _, err := parseCSVP2PKHRedeem(script)
	return err == nil
}

// BuildCSVP2PKHRedeemScript builds a standard P2SH CSV+P2PKH redeem script (tests / RPC tooling).
func BuildCSVP2PKHRedeemScript(relativeSequence int64, pubKeyHash [20]byte) []byte {
	return buildCSVP2PKHRedeemScript(relativeSequence, pubKeyHash)
}

func buildCSVP2PKHRedeemScript(relativeSequence int64, pubKeyHash [20]byte) []byte {
	var b []byte
	b = append(b, encodeScriptNum(relativeSequence)...)
	b = append(b, opCheckSequenceVerify, 0x75, 0x76, 0xa9, 0x14)
	b = append(b, pubKeyHash[:]...)
	b = append(b, 0x88, 0xac)
	return b
}
