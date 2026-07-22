// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"strings"
)

// ScriptVerifyP2SH enables P2SH redeem validation (SCRIPT_VERIFY_P2SH).
const ScriptVerifyP2SH ScriptVerifyFlags = 1 << 0

// ScriptVerifyWitness enables witness validation (SCRIPT_VERIFY_WITNESS).
const ScriptVerifyWitness ScriptVerifyFlags = 1 << 11

// ScriptVerifySigPushOnly requires scriptSig to be push-only (SCRIPT_VERIFY_SIGPUSHONLY).
const ScriptVerifySigPushOnly ScriptVerifyFlags = 1 << 5

// ScriptVerifyCleanStack requires exactly one stack item after successful verification (SCRIPT_VERIFY_CLEANSTACK).
const ScriptVerifyCleanStack ScriptVerifyFlags = 1 << 8

// ParseScriptTestFlags maps Core script_tests.json flag strings to DogeGo flags.
func ParseScriptTestFlags(s string) ScriptVerifyFlags {
	s = strings.TrimSpace(s)
	if s == "" || s == "NONE" {
		return 0
	}
	var f ScriptVerifyFlags
	for _, word := range strings.Split(s, ",") {
		word = strings.TrimSpace(word)
		switch word {
		case "NONE", "":
		case "P2SH":
			f |= ScriptVerifyP2SH
		case "STRICTENC":
			f |= ScriptVerifyStrictEnc
		case "DERSIG":
			f |= ScriptVerifyDERSig
		case "LOW_S":
			f |= ScriptVerifyLowS
		case "MINIMALDATA":
			f |= ScriptVerifyMinimalData
		case "NULLDUMMY":
			f |= ScriptVerifyNullDummy
		case "DISCOURAGE_UPGRADABLE_NOPS":
			f |= ScriptVerifyDiscourageUpgradableNops
		case "NULLFAIL":
			f |= ScriptVerifyNullFail
		case "CHECKLOCKTIMEVERIFY":
			f |= ScriptVerifyCheckLockTimeVerify
		case "CHECKSEQUENCEVERIFY":
			f |= ScriptVerifyCheckSequenceVerify
		case "WITNESS":
			f |= ScriptVerifyWitness
		case "SIGPUSHONLY":
			f |= ScriptVerifySigPushOnly
		case "CLEANSTACK":
			f |= ScriptVerifyCleanStack
		case "MINIMALIF",
			"DISCOURAGE_UPGRADABLE_WITNESS_PROGRAM", "WITNESS_PUBKEYTYPE":
			// Known Core flags not used by current DogeGo script_tests subset.
		}
	}
	return f
}
