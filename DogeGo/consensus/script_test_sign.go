// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/wire"
)

// scriptTestKey0 is Core script_tests vchKey0 (generator point).
var scriptTestKey0 = func() []byte {
	b := make([]byte, 32)
	b[31] = 1
	return b
}()

func signScriptTestSpend(sec []byte, scriptPubKey []byte, hashType uint32) ([]byte, error) {
	spend, _ := buildScriptTestCreditSpend(nil, scriptPubKey)
	priv, _ := secp256k1.PrivKeyFromBytes(sec)
	digest, err := wire.CalcSignatureHashLegacy(scriptPubKey, hashType, spend, 0)
	if err != nil {
		return nil, err
	}
	sig := append(ecdsa.Sign(priv, digest[:]).Serialize(), byte(hashType))
	switch {
	case isP2PKScript(scriptPubKey):
		return buildSinglePushScript(sig), nil
	case isP2PKHScript(scriptPubKey):
		_, pub := secp256k1.PrivKeyFromBytes(sec)
		return buildP2PKHScriptSig(sig, pub.SerializeCompressed()), nil
	default:
		return buildSinglePushScript(sig), nil
	}
}
