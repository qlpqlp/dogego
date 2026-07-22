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

// ScriptVerifyDiscourageUpgradableNops rejects reserved NOPs and disabled soft-fork opcodes in mempool scripts.
const ScriptVerifyDiscourageUpgradableNops ScriptVerifyFlags = 1 << 7

func opcodeDiscouraged(op byte, flags ScriptVerifyFlags) bool {
	switch op {
	case 0xb0, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9: // OP_NOP1, OP_NOP4..10
		return true
	case opCheckLockTimeVerify:
		return flags&ScriptVerifyCheckLockTimeVerify == 0
	case opCheckSequenceVerify:
		return flags&ScriptVerifyCheckSequenceVerify == 0
	default:
		return false
	}
}

// checkScriptDiscouragedOps scans a script for executed discouraged opcodes (mempool policy; whole-script scan).
func checkScriptDiscouragedOps(script []byte, flags ScriptVerifyFlags) error {
	if flags&ScriptVerifyDiscourageUpgradableNops == 0 || len(script) == 0 {
		return nil
	}
	i := 0
	for i < len(script) {
		op := script[i]
		i++
		if opcodeDiscouraged(op, flags) {
			return fmt.Errorf("script-verify: DISCOURAGE_UPGRADABLE_NOPS")
		}
		switch {
		case op == 0x00:
		case op >= 0x01 && op <= 0x4b:
			n := int(op)
			if i+n > len(script) {
				return fmt.Errorf("script-verify: truncated push")
			}
			i += n
		case op == 0x4c:
			if i >= len(script) {
				return fmt.Errorf("script-verify: truncated pushdata1")
			}
			n := int(script[i])
			i++
			if i+n > len(script) {
				return fmt.Errorf("script-verify: truncated pushdata1 data")
			}
			i += n
		case op == 0x4d:
			if i+1 >= len(script) {
				return fmt.Errorf("script-verify: truncated pushdata2")
			}
			n := int(script[i]) | int(script[i+1])<<8
			i += 2
			if i+n > len(script) {
				return fmt.Errorf("script-verify: truncated pushdata2 data")
			}
			i += n
		case op == 0x4e:
			if i+3 >= len(script) {
				return fmt.Errorf("script-verify: truncated pushdata4")
			}
			n := int(script[i]) | int(script[i+1])<<8 | int(script[i+2])<<16 | int(script[i+3])<<24
			i += 4
			if i+n > len(script) {
				return fmt.Errorf("script-verify: truncated pushdata4 data")
			}
			i += n
		case op >= 0x51 && op <= 0x60, op == 0x4f:
		default:
			// Non-push opcodes in scriptSig/redeem are rare for standard templates; still walk.
		}
	}
	return nil
}

func checkInputDiscouragedOps(tx *wire.Tx, idx int, pkScript []byte, flags ScriptVerifyFlags) error {
	if err := checkScriptDiscouragedOps(tx.Vin[idx].Script, flags); err != nil {
		return err
	}
	if err := checkScriptDiscouragedOps(pkScript, flags); err != nil {
		return err
	}
	if isP2SHScript(pkScript) {
		pushes, err := allScriptPushes(tx.Vin[idx].Script)
		if err != nil {
			return err
		}
		if len(pushes) < 2 {
			return nil
		}
		if err := checkScriptDiscouragedOps(pushes[len(pushes)-1], flags); err != nil {
			return err
		}
	}
	return nil
}
