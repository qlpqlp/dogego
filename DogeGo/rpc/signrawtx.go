// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
	"dogego/consensus"
	"dogego/wire"
)

type signRawTxOpts struct {
	keysOnly bool // signrawtransactionwithkey: privkeys required, no wallet key merge
}

// execSignRawTransaction implements signrawtransaction for legacy transactions: P2PKH / P2PK / P2SH
// (with redeemScript, including multisig) prevouts, WIF private keys, prevtxs with scriptPubKey.
func execSignRawTransaction(chainName string, paths *DataPaths, params []json.RawMessage, opts ...signRawTxOpts) (map[string]interface{}, int, string) {
	var o signRawTxOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	if len(params) < 1 {
		return nil, -8, "signrawtransaction: hex string required"
	}
	var hexStr string
	if err := json.Unmarshal(params[0], &hexStr); err != nil {
		return nil, -8, "signrawtransaction: bad hex string"
	}
	hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	raw, err := hex.DecodeString(hexStr)
	if err != nil || len(raw) == 0 {
		return nil, -8, "signrawtransaction: invalid transaction hex"
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return nil, -8, "signrawtransaction: TX decode failed: " + err.Error()
	}
	if tx.HasWitness() {
		return nil, -8, "signrawtransaction: witness transactions are not supported in this build"
	}

	var prevJSON []json.RawMessage
	if len(params) >= 2 && string(params[1]) != "null" {
		if err := json.Unmarshal(params[1], &prevJSON); err != nil {
			return nil, -8, "signrawtransaction: prevtxs must be an array or null"
		}
	}
	walletPrev, werr := buildWalletPrevTxs(tx, paths)
	if werr != nil {
		return nil, -8, "signrawtransaction: " + werr.Error()
	}
	if len(prevJSON) == 0 {
		prevJSON = walletPrev
	} else if len(walletPrev) > 0 {
		prevJSON = mergePrevTxJSON(walletPrev, prevJSON)
	}
	if len(prevJSON) == 0 {
		return nil, -8, "signrawtransaction: prevtxs required (or enable wallet + UTXO cache for auto prevouts)"
	}

	errPrefix := "signrawtransaction"
	if o.keysOnly {
		errPrefix = "signrawtransactionwithkey"
	}
	var privStrs []string
	if len(params) >= 3 && string(params[2]) != "null" {
		if err := json.Unmarshal(params[2], &privStrs); err != nil {
			return nil, -8, errPrefix + ": privkeys must be an array of strings"
		}
	}
	useWalletKeys := !o.keysOnly && (len(params) < 3 || strings.TrimSpace(string(params[2])) == "null")
	if o.keysOnly {
		if len(params) < 3 || strings.TrimSpace(string(params[2])) == "null" || len(privStrs) == 0 {
			return nil, -8, errPrefix + ": privkeys array required"
		}
	} else if len(privStrs) == 0 {
		privStrs = rpcWalletWIFs(paths)
	}
	if len(privStrs) == 0 {
		return nil, -8, errPrefix + ": privkeys array required (or enable built-in wallet)"
	}
	if useWalletKeys && rpcWalletDefaultAddress(paths) != "" {
		if code, msg := rpcWalletRequireUnlocked(paths); code != 0 {
			return nil, code, msg
		}
		applyWalletSpendTimelocks(tx, paths)
	}

	sighashStr := "ALL"
	if len(params) >= 4 && string(params[3]) != "null" {
		if err := json.Unmarshal(params[3], &sighashStr); err != nil {
			return nil, -8, "signrawtransaction: bad sighashtype"
		}
	}
	hashType, err := parseSigHashType(sighashStr)
	if err != nil {
		return nil, -8, "signrawtransaction: " + err.Error()
	}

	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}

	prevMap, err := buildPrevOutMap(prevJSON)
	if err != nil {
		return nil, -8, "signrawtransaction: " + err.Error()
	}

	keys, err := decodeWIFPrivKeys(privStrs, p.PrivKeyWIFVersion)
	if err != nil {
		return nil, -8, "signrawtransaction: " + err.Error()
	}

	var errs []map[string]interface{}
	for idx := range tx.Vin {
		in := &tx.Vin[idx]
		if isCoinbaseWireIn(in) {
			errs = append(errs, signRawErrEntry(in, "coinbase input cannot be signed"))
			continue
		}
		if len(in.Script) > 0 {
			continue
		}
		key := prevMapKey(in.PrevHash, in.PrevIdx)
		ent, ok := prevMap[key]
		if !ok {
			errs = append(errs, signRawErrEntry(in, "missing prevtx for this input"))
			continue
		}
		spk, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(ent.ScriptPubKey), "0x"))
		if err != nil || len(spk) == 0 {
			errs = append(errs, signRawErrEntry(in, "invalid scriptPubKey hex"))
			continue
		}
		var redeem []byte
		if ent.RedeemScript != "" {
			redeem, err = hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(ent.RedeemScript), "0x"))
			if err != nil {
				errs = append(errs, signRawErrEntry(in, "invalid redeemScript hex"))
				continue
			}
		}
		var innerRedeem []byte
		if ent.InnerRedeemScript != "" {
			innerRedeem, err = hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(ent.InnerRedeemScript), "0x"))
			if err != nil {
				errs = append(errs, signRawErrEntry(in, "invalid innerRedeemScript hex"))
				continue
			}
		}
		var doginalPartial []byte
		if ent.DoginalPartial != "" {
			doginalPartial, err = hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(ent.DoginalPartial), "0x"))
			if err != nil {
				errs = append(errs, signRawErrEntry(in, "invalid doginalPartial hex"))
				continue
			}
		}
		scriptSig, signErr := signInputScript(tx, idx, spk, redeem, innerRedeem, doginalPartial, keys, hashType)
		if signErr != nil {
			errs = append(errs, signRawErrEntry(in, signErr.Error()))
			continue
		}
		in.Script = scriptSig
	}

	complete := signRawTxComplete(tx)
	outSer, err := tx.Serialize()
	if err != nil {
		return nil, -8, err.Error()
	}
	return map[string]interface{}{
		"hex":      hex.EncodeToString(outSer),
		"complete": complete,
		"errors":   errs,
	}, 0, ""
}

func signRawTxComplete(tx *wire.Tx) bool {
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if isCoinbaseWireIn(in) {
			continue
		}
		if len(in.Script) == 0 {
			return false
		}
	}
	return true
}

func signRawErrEntry(in *wire.TxIn, msg string) map[string]interface{} {
	return map[string]interface{}{
		"txid":      txidToRPC(in.PrevHash),
		"vout":      float64(in.PrevIdx),
		"scriptSig": hex.EncodeToString(in.Script),
		"sequence":  float64(in.Sequence),
		"error":     msg,
	}
}

func isCoinbaseWireIn(in *wire.TxIn) bool {
	var z [32]byte
	return in.PrevHash == z && in.PrevIdx == 0xffffffff
}

type prevOutEnt struct {
	ScriptPubKey      string
	RedeemScript      string
	InnerRedeemScript string
	DoginalPartial    string // hex of inscription pushdatas prepended to doginal P2SH unlock
}

func signInputScript(tx *wire.Tx, idx int, spk, redeem, innerRedeem, doginalPartial []byte, keys []wifPriv, hashType uint32) ([]byte, error) {
	scriptCode, p2sh, redeemPushes, err := consensus.SigningScriptAndRedeem(spk, redeem, innerRedeem)
	if err != nil {
		return nil, err
	}
	if consensus.IsMultisigRedeemScript(scriptCode) {
		return signMultisigScriptSig(tx, idx, scriptCode, redeemPushes, keys, hashType)
	}
	if pub, _, ok := consensus.ParseDoginalLockRedeem(scriptCode); ok {
		if len(doginalPartial) == 0 {
			return nil, fmt.Errorf("doginalPartial required for doginal P2SH redeem")
		}
		priv, ok := findPrivForPubkeyBytes(keys, pub)
		if !ok {
			return nil, fmt.Errorf("no matching private key for doginal lock pubkey")
		}
		digest, err := wire.CalcSignatureHashLegacy(scriptCode, hashType, tx, idx)
		if err != nil {
			return nil, err
		}
		sig := ecdsa.Sign(priv, digest[:])
		sigWithType := append(sig.Serialize(), byte(hashType&0xff))
		sigPush, err := pushScriptData(sigWithType)
		if err != nil {
			return nil, err
		}
		out := append([]byte(nil), doginalPartial...)
		out = append(out, sigPush...)
		for _, r := range redeemPushes {
			p, err := pushScriptData(r)
			if err != nil {
				return nil, err
			}
			out = append(out, p...)
		}
		return out, nil
	}
	var wantH160 [20]byte
	switch {
	case isP2PKHScript(scriptCode):
		wantH160 = p2pkhPubKeyHash160(scriptCode)
	case len(scriptCode) >= 3 && scriptCode[len(scriptCode)-1] == 0xac:
		pub, perr := parseP2PKPubKeyForSign(scriptCode)
		if perr != nil {
			return nil, perr
		}
		wantH160 = pubkeyHash160(pub)
	default:
		return nil, fmt.Errorf("unsupported signing script template")
	}
	priv, compressed, ok := findPrivForP2PKH(keys, wantH160)
	if !ok {
		return nil, fmt.Errorf("no matching private key for this prevout")
	}
	digest, err := wire.CalcSignatureHashLegacy(scriptCode, hashType, tx, idx)
	if err != nil {
		return nil, err
	}
	sig := ecdsa.Sign(priv, digest[:])
	sigWithType := append(sig.Serialize(), byte(hashType&0xff))
	pub := priv.PubKey()
	var pubBytes []byte
	if compressed {
		pubBytes = pub.SerializeCompressed()
	} else {
		pubBytes = pub.SerializeUncompressed()
	}
	if p2sh {
		if len(redeemPushes) == 0 {
			return nil, fmt.Errorf("redeemScript required for P2SH")
		}
		parts := [][]byte{sigWithType, pubBytes}
		if !isP2PKHScript(scriptCode) {
			parts = [][]byte{sigWithType}
		}
		parts = append(parts, redeemPushes...)
		return concatPushes(parts...)
	}
	if isP2PKHScript(scriptCode) {
		return concatPushes(sigWithType, pubBytes)
	}
	return concatPushes(sigWithType)
}

func parseP2PKPubKeyForSign(pkScript []byte) ([]byte, error) {
	if len(pkScript) < 3 || pkScript[len(pkScript)-1] != 0xac {
		return nil, fmt.Errorf("bad P2PK script")
	}
	op := pkScript[0]
	var want int
	switch op {
	case 0x21:
		want = 33
	case 0x41:
		want = 65
	default:
		return nil, fmt.Errorf("bad P2PK push opcode")
	}
	if len(pkScript) != 1+want+1 {
		return nil, fmt.Errorf("bad P2PK script length")
	}
	return pkScript[1 : 1+want], nil
}

func signMultisigScriptSig(tx *wire.Tx, idx int, multisigRedeem []byte, p2shRedeemPushes [][]byte, keys []wifPriv, hashType uint32) ([]byte, error) {
	nReq, pubs, err := consensus.ParseMultisigRedeemScript(multisigRedeem)
	if err != nil {
		return nil, err
	}
	parts := [][]byte{{0x00}} // CHECKMULTISIG OP_0 dummy
	signed := 0
	for _, wantPub := range pubs {
		if signed >= nReq {
			break
		}
		priv, ok := findPrivForPubkeyBytes(keys, wantPub)
		if !ok {
			continue
		}
		digest, err := wire.CalcSignatureHashLegacy(multisigRedeem, hashType, tx, idx)
		if err != nil {
			return nil, err
		}
		sig := ecdsa.Sign(priv, digest[:])
		parts = append(parts, append(sig.Serialize(), byte(hashType&0xff)))
		signed++
	}
	if signed < nReq {
		return nil, fmt.Errorf("not enough private keys for multisig (%d of %d signatures)", signed, nReq)
	}
	parts = append(parts, p2shRedeemPushes...)
	return concatPushes(parts...)
}

func findPrivForPubkeyBytes(keys []wifPriv, wantPub []byte) (*secp256k1.PrivateKey, bool) {
	for _, k := range keys {
		pub := k.key.PubKey()
		if bytes.Equal(pub.SerializeCompressed(), wantPub) || bytes.Equal(pub.SerializeUncompressed(), wantPub) {
			return k.key, true
		}
	}
	return nil, false
}

func concatPushes(parts ...[]byte) ([]byte, error) {
	var out []byte
	for _, p := range parts {
		chunk, err := pushScriptData(p)
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
	}
	return out, nil
}

func buildPrevOutMap(prevJSON []json.RawMessage) (map[string]prevOutEnt, error) {
	m := make(map[string]prevOutEnt, len(prevJSON))
	for _, raw := range prevJSON {
		var o struct {
			Txid              string `json:"txid"`
			Vout              uint32 `json:"vout"`
			ScriptPubKey      string `json:"scriptPubKey"`
			RedeemScript      string `json:"redeemScript"`
			InnerRedeemScript string `json:"innerRedeemScript"`
			DoginalPartial    string `json:"doginalPartial"`
		}
		if err := json.Unmarshal(raw, &o); err != nil {
			return nil, fmt.Errorf("prevtxs entry must be an object with txid, vout, scriptPubKey")
		}
		txid := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(o.Txid), "0x"))
		if len(txid) != 64 {
			return nil, fmt.Errorf("prevtxs txid must be 64 hex characters")
		}
		for _, c := range txid {
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
				continue
			}
			return nil, fmt.Errorf("prevtxs txid must be hex")
		}
		key := fmt.Sprintf("%s:%d", txid, o.Vout)
		if _, dup := m[key]; dup {
			return nil, fmt.Errorf("duplicate prevtxs entry for %s", key)
		}
		m[key] = prevOutEnt{
			ScriptPubKey:      o.ScriptPubKey,
			RedeemScript:      o.RedeemScript,
			InnerRedeemScript: o.InnerRedeemScript,
			DoginalPartial:    o.DoginalPartial,
		}
	}
	return m, nil
}

// mergePrevTxJSON overlays provided prevtxs on wallet auto prevtxs (provided wins).
func mergePrevTxJSON(wallet, provided []json.RawMessage) []json.RawMessage {
	type key struct {
		txid string
		vout uint32
	}
	order := make([]key, 0, len(wallet)+len(provided))
	seen := map[key]struct{}{}
	by := map[key]json.RawMessage{}
	ingest := func(list []json.RawMessage) {
		for _, raw := range list {
			var o struct {
				Txid string `json:"txid"`
				Vout uint32 `json:"vout"`
			}
			if json.Unmarshal(raw, &o) != nil {
				continue
			}
			txid := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(o.Txid), "0x"))
			k := key{txid: txid, vout: o.Vout}
			if _, ok := seen[k]; !ok {
				seen[k] = struct{}{}
				order = append(order, k)
			}
			by[k] = raw
		}
	}
	ingest(wallet)
	ingest(provided)
	out := make([]json.RawMessage, 0, len(order))
	for _, k := range order {
		out = append(out, by[k])
	}
	return out
}

func prevMapKey(prevHash [32]byte, vout uint32) string {
	return fmt.Sprintf("%s:%d", txidToRPC(prevHash), vout)
}

func parseSigHashType(s string) (uint32, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return wire.SigHashAll, nil
	}
	u := strings.ToUpper(s)
	acp := strings.Contains(u, "ANYONECANPAY")
	u = strings.ReplaceAll(u, "ANYONECANPAY", "")
	u = strings.Trim(u, "| \t")
	var base uint32
	switch u {
	case "", "ALL":
		base = wire.SigHashAll
	case "NONE":
		base = wire.SigHashNone
	case "SINGLE":
		base = wire.SigHashSingle
	default:
		return 0, fmt.Errorf("invalid sighashtype %q", s)
	}
	if acp {
		base |= wire.SigHashAnyoneCanPay
	}
	return base, nil
}

type wifPriv struct {
	key        *secp256k1.PrivateKey
	compressed bool
}

func decodeWIFPrivKeys(strs []string, wifVer byte) ([]wifPriv, error) {
	out := make([]wifPriv, 0, len(strs))
	for _, s := range strs {
		sec, compressed, err := chain.DecodeWIF(strings.TrimSpace(s), wifVer)
		if err != nil {
			return nil, fmt.Errorf("invalid WIF: %w", err)
		}
		sk, _ := secp256k1.PrivKeyFromBytes(sec)
		out = append(out, wifPriv{key: sk, compressed: compressed})
	}
	return out, nil
}

func isP2PKHScript(pk []byte) bool {
	return len(pk) == 25 && pk[0] == 0x76 && pk[1] == 0xa9 && pk[2] == 0x14 && pk[23] == 0x88 && pk[24] == 0xac
}

func isP2SHScript(pk []byte) bool {
	return len(pk) == 23 && pk[0] == 0xa9 && pk[1] == 0x14 && pk[22] == 0x87
}

func p2pkhPubKeyHash160(pk []byte) [20]byte {
	var h [20]byte
	copy(h[:], pk[3:23])
	return h
}

func findPrivForP2PKH(keys []wifPriv, want [20]byte) (*secp256k1.PrivateKey, bool, bool) {
	for _, k := range keys {
		pub := k.key.PubKey()
		var pubBytes []byte
		if k.compressed {
			pubBytes = pub.SerializeCompressed()
		} else {
			pubBytes = pub.SerializeUncompressed()
		}
		got := pubkeyHash160(pubBytes)
		if got == want {
			return k.key, k.compressed, true
		}
	}
	return nil, false, false
}

func pushScriptData(data []byte) ([]byte, error) {
	n := len(data)
	if n == 0 {
		return []byte{0x00}, nil // OP_0
	}
	if n > 0xffffff {
		return nil, fmt.Errorf("invalid push length %d", n)
	}
	var head []byte
	switch {
	case n <= 75:
		head = []byte{byte(n)}
	case n <= 0xff:
		head = []byte{0x4c, byte(n)}
	case n <= 0xffff:
		head = []byte{0x4d, byte(n), byte(n >> 8)}
	default:
		buf := make([]byte, 5)
		buf[0] = 0x4e
		binary.LittleEndian.PutUint32(buf[1:], uint32(n))
		head = buf
	}
	return append(head, data...), nil
}
