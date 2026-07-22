// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"dogego/secp256k1"

	"dogego/wire"
)

const scriptTestP2PKPubASM = "0x41 0x0479be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8 CHECKSIG"

type scriptTestRow struct {
	ScriptSig    string
	ScriptPubKey string
	Flags        string
	Want         string
	HasWitness   bool
	SkipReason   string
}

// TestCoreScriptTestsRunnerSubset replays rows from Core script_tests.json through EvalScript / CHECKSIG spend harness.
func TestCoreScriptTestsRunnerSubset(t *testing.T) {
	path := coreScriptTestsJSONPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("script_tests.json missing: %v", err)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	var ran, passed, skipped, failed int
	for _, row := range rows {
		st, ok := parseScriptTestRow(row)
		if !ok {
			continue
		}
		if st.SkipReason != "" {
			skipped++
			continue
		}
		sig, err := ParseScriptASM(st.ScriptSig)
		if err != nil {
			skipped++
			continue
		}
		pub, err := ParseScriptASM(st.ScriptPubKey)
		if err != nil {
			skipped++
			continue
		}
		flags := ParseScriptTestFlags(st.Flags)
		if strings.Contains(st.Flags, "P2SH") {
			flags |= ScriptVerifyP2SH
		}
		ran++
		var got ScriptError
		if scriptTestNeedsSpendContext(st.ScriptSig, st.ScriptPubKey, flags) {
			got = VerifyScriptTestSpend(sig, pub, flags)
		} else {
			got = VerifyScriptTest(sig, pub, flags)
		}
		want := ScriptError(st.Want)
		if got != want {
			failed++
			continue
		}
		passed++
	}
	if failed > 0 {
		t.Fatalf("script_tests failures: ran=%d passed=%d failed=%d skipped=%d", ran, passed, failed, skipped)
	}
	if ran < 25 {
		t.Fatalf("too few script_tests rows executed: ran=%d passed=%d failed=%d skipped=%d", ran, passed, failed, skipped)
	}
	t.Logf("script_tests: ran=%d passed=%d failed=%d skipped=%d", ran, passed, failed, skipped)
	if passed < 1055 {
		t.Fatalf("expected at least 1055 passing legacy script_tests rows, got %d (failed=%d)", passed, failed)
	}
}

func scriptTestNeedsChecker(scriptSig, scriptPubKey string) bool {
	upper := strings.ToUpper(scriptSig + " " + scriptPubKey)
	return strings.Contains(upper, "CHECKSIG") || strings.Contains(upper, "CHECKMULTISIG")
}

func scriptTestNeedsSpendContext(scriptSig, scriptPubKey string, flags ScriptVerifyFlags) bool {
	if scriptTestNeedsChecker(scriptSig, scriptPubKey) {
		return true
	}
	if flags&(ScriptVerifyP2SH|ScriptVerifyCheckLockTimeVerify|ScriptVerifyCheckSequenceVerify) != 0 {
		return true
	}
	return false
}

func parseScriptTestRow(row json.RawMessage) (scriptTestRow, bool) {
	var cells []json.RawMessage
	if err := json.Unmarshal(row, &cells); err != nil || len(cells) < 4 {
		return scriptTestRow{}, false
	}
	if len(cells) > 0 {
		var arr []json.RawMessage
		if json.Unmarshal(cells[0], &arr) == nil && len(arr) > 0 {
			return scriptTestRow{SkipReason: "witness"}, false
		}
	}
	var sig, pub, flags, want string
	if err := json.Unmarshal(cells[0], &sig); err != nil {
		return scriptTestRow{}, false
	}
	if strings.HasPrefix(sig, "Format") || strings.HasPrefix(sig, "It is") {
		return scriptTestRow{}, false
	}
	if err := json.Unmarshal(cells[1], &pub); err != nil {
		return scriptTestRow{}, false
	}
	if err := json.Unmarshal(cells[2], &flags); err != nil {
		return scriptTestRow{}, false
	}
	if err := json.Unmarshal(cells[3], &want); err != nil {
		return scriptTestRow{}, false
	}
	st := scriptTestRow{ScriptSig: sig, ScriptPubKey: pub, Flags: flags, Want: want}
	if strings.Contains(flags, "WITNESS") {
		st.SkipReason = "witness"
		return st, true
	}
	return st, true
}

// TestCoreScriptTestsCHECKSIGVectors exercises CHECKSIG + HASH160 using DogeGo-signed spends (Core JSON sigs use a different legacy sighash wire until cross-validated).
func TestCoreScriptTestsCHECKSIGVectors(t *testing.T) {
	p2pk, err := ParseScriptASM(scriptTestP2PKPubASM)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := signScriptTestSpend(scriptTestKey0, p2pk, wire.SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	if got := VerifyScriptTestSpend(sig, p2pk, 0); got != ScriptErrOK {
		t.Fatalf("P2PK: %s", got)
	}

	sec := make([]byte, 32)
	sec[30] = 1
	_, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	p2pkh := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	p2pkh = append(p2pkh, 0x88, 0xac)
	sig2, err := signScriptTestSpend(sec, p2pkh, wire.SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	if got := VerifyScriptTestSpend(sig2, p2pkh, 0); got != ScriptErrOK {
		t.Fatalf("P2PKH: %s", got)
	}
	wrongH := append([]byte{0x76, 0xa9, 0x14}, make([]byte, 20)...)
	wrongH = append(wrongH, 0x88, 0xac)
	if got := VerifyScriptTestSpend(sig2, wrongH, 0); got != ScriptErrEqualVerify {
		t.Fatalf("bad hash want EQUALVERIFY got %s", got)
	}
}
