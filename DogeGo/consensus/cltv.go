// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"fmt"

	"dogego/wire"
)

const opCheckLockTimeVerify = 0xb1

// CheckLockTime implements BIP65 CHECKLOCKTIMEVERIFY against the spending transaction.
func CheckLockTime(tx *wire.Tx, inputIdx int, lockTime int64) error {
	if tx == nil {
		return fmt.Errorf("cltv: nil transaction")
	}
	if inputIdx < 0 || inputIdx >= len(tx.Vin) {
		return fmt.Errorf("cltv: bad input index")
	}
	if lockTime < 0 {
		return fmt.Errorf("cltv: negative locktime")
	}
	txLT := int64(tx.LockTime)
	lt := lockTime
	heightBased := txLT < wire.LocktimeThreshold
	operandHeight := lt < wire.LocktimeThreshold
	if heightBased != operandHeight {
		return fmt.Errorf("cltv: locktime type mismatch")
	}
	if lt > txLT {
		return fmt.Errorf("cltv: unsatisfied locktime")
	}
	if tx.Vin[inputIdx].Sequence == wire.SequenceFinal {
		return fmt.Errorf("cltv: input finalized")
	}
	return nil
}

// parseCLTVP2PKHRedeem decodes <locktime> OP_CHECKLOCKTIMEVERIFY OP_DROP OP_DUP OP_HASH160 <20> OP_EQUALVERIFY OP_CHECKSIG.
func parseCLTVP2PKHRedeem(script []byte) (lockTime int64, p2pkh []byte, err error) {
	if len(script) < 26 {
		return 0, nil, fmt.Errorf("cltv: script too short")
	}
	lt, i, err := ReadScriptNumPush(script, 0)
	if err != nil {
		return 0, nil, err
	}
	if i >= len(script) || script[i] != opCheckLockTimeVerify {
		return 0, nil, fmt.Errorf("cltv: missing OP_CHECKLOCKTIMEVERIFY")
	}
	i++
	want := []byte{0x75, 0x76, 0xa9, 0x14}
	if i+len(want)+20+2 > len(script) {
		return 0, nil, fmt.Errorf("cltv: truncated p2pkh tail")
	}
	if string(script[i:i+len(want)]) != string(want) {
		return 0, nil, fmt.Errorf("cltv: unexpected tail prefix")
	}
	i += len(want)
	var h160 [20]byte
	copy(h160[:], script[i:i+20])
	i += 20
	if script[i] != 0x88 || script[i+1] != 0xac || i+2 != len(script) {
		return 0, nil, fmt.Errorf("cltv: bad equalverify/checksig")
	}
	p2pkh = append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	p2pkh = append(p2pkh, 0x88, 0xac)
	return lt, p2pkh, nil
}

func isCLTVP2PKHRedeem(script []byte) bool {
	_, _, err := parseCLTVP2PKHRedeem(script)
	return err == nil
}

// BuildCLTVP2PKHRedeemScript builds a standard P2SH CLTV+P2PKH redeem script (tests / RPC tooling).
func BuildCLTVP2PKHRedeemScript(lockTime int64, pubKeyHash [20]byte) []byte {
	return buildCLTVP2PKHRedeemScript(lockTime, pubKeyHash)
}

func buildCLTVP2PKHRedeemScript(lockTime int64, pubKeyHash [20]byte) []byte {
	var b []byte
	b = append(b, encodeScriptNum(lockTime)...)
	b = append(b, opCheckLockTimeVerify, 0x75, 0x76, 0xa9, 0x14)
	b = append(b, pubKeyHash[:]...)
	b = append(b, 0x88, 0xac)
	return b
}

func encodeScriptNum(n int64) []byte {
	if n == 0 {
		return []byte{0x00}
	}
	raw := scriptNumRawBytes(n)
	if len(raw) <= 75 {
		out := make([]byte, 1+len(raw))
		out[0] = byte(len(raw))
		copy(out[1:], raw)
		return out
	}
	out := make([]byte, 2+len(raw))
	out[0] = 0x4c
	out[1] = byte(len(raw))
	copy(out[2:], raw)
	return out
}

