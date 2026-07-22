// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// ScriptError is the Core script_tests.json expected result token (SCRIPT_ERR_* name).
type ScriptError string

const (
	ScriptErrOK                      ScriptError = "OK"
	ScriptErrEvalFalse               ScriptError = "EVAL_FALSE"
	ScriptErrBadOpcode               ScriptError = "BAD_OPCODE"
	ScriptErrInvalidStackOperation   ScriptError = "INVALID_STACK_OPERATION"
	ScriptErrStackSize               ScriptError = "STACK_SIZE"
	ScriptErrUnbalancedConditional   ScriptError = "UNBALANCED_CONDITIONAL"
	ScriptErrDisabledOpcode          ScriptError = "DISABLED_OPCODE"
	ScriptErrOpReturn                ScriptError = "OP_RETURN"
	ScriptErrMinimalData             ScriptError = "MINIMALDATA"
	ScriptErrSigNullDummy            ScriptError = "SIG_NULLDUMMY"
	ScriptErrOpCount                 ScriptError = "OP_COUNT"
	ScriptErrDiscourageUpgradable    ScriptError = "DISCOURAGE_UPGRADABLE_NOPS"
	ScriptErrSigDer                  ScriptError = "SIG_DER"
	ScriptErrSigHashType             ScriptError = "SIG_HASHTYPE"
	ScriptErrSigHighS                ScriptError = "SIG_HIGH_S"
	ScriptErrPubKeyType              ScriptError = "PUBKEYTYPE"
	ScriptErrSigNullFail             ScriptError = "NULLFAIL"
	ScriptErrVerify                    ScriptError = "VERIFY"
	ScriptErrCheckSigVerify          ScriptError = "CHECKSIGVERIFY"
	ScriptErrInvalidAltStackOperation  ScriptError = "INVALID_ALTSTACK_OPERATION"
	ScriptErrEqualVerify             ScriptError = "EQUALVERIFY"
	ScriptErrNumEqualVerify          ScriptError = "NUMEQUALVERIFY"
	ScriptErrSigPushOnly             ScriptError = "SIG_PUSHONLY"
	ScriptErrCleanStack              ScriptError = "CLEANSTACK"
	ScriptErrPushSize                ScriptError = "PUSH_SIZE"
	ScriptErrScriptSize              ScriptError = "SCRIPT_SIZE"
	ScriptErrPubKeyCount             ScriptError = "PUBKEY_COUNT"
	ScriptErrSigCount                ScriptError = "SIG_COUNT"
	ScriptErrNegativeLocktime        ScriptError = "NEGATIVE_LOCKTIME"
	ScriptErrUnsatisfiedLocktime     ScriptError = "UNSATISFIED_LOCKTIME"
	ScriptErrUnknown                 ScriptError = "UNKNOWN_ERROR"
)
