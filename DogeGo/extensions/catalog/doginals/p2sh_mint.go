// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/extensions"
)

// apezord doginals.js constants
const (
	maxDoginalChunkLen   = 240
	maxDoginalPayloadLen = 1500
	doginalDustDOGE      = 0.001 // 100000 koinu / satoshis
)

type doginalChunk struct {
	data []byte // nil means opcode-only (OP_0 / OP_1..OP_16)
	op   byte   // set when data is nil and op is small-number or OP_0
}

type p2shMintStep struct {
	PartialHex   string
	RedeemHex    string
	P2SHAddress  string
	ScriptPubKey string
	Drops        int
}

// BuildDoginalInscriptionChunks builds apezord "ord" inscription push list.
func BuildDoginalInscriptionChunks(contentType string, data []byte) ([]doginalChunk, error) {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = sniffContentType(data)
	}
	if len(contentType) > 520 {
		return nil, fmt.Errorf("content type too long")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("no data to mint")
	}
	if len(data) > MaxInscriptionBodyBytes {
		return nil, fmt.Errorf("data exceeds %d bytes", MaxInscriptionBodyBytes)
	}
	var parts [][]byte
	rest := data
	for len(rest) > 0 {
		n := maxDoginalChunkLen
		if n > len(rest) {
			n = len(rest)
		}
		parts = append(parts, append([]byte(nil), rest[:n]...))
		rest = rest[n:]
	}
	out := []doginalChunk{
		{data: []byte("ord")},
		numberChunk(len(parts)),
		{data: []byte(contentType)},
	}
	for i, part := range parts {
		out = append(out, numberChunk(len(parts)-i-1), doginalChunk{data: part})
	}
	return out, nil
}

func numberChunk(n int) doginalChunk {
	switch {
	case n == 0:
		return doginalChunk{op: 0x00}
	case n <= 16:
		return doginalChunk{op: byte(0x50 + n)}
	case n < 128:
		return doginalChunk{data: []byte{byte(n)}}
	default:
		return doginalChunk{data: []byte{byte(n % 256), byte(n / 256)}}
	}
}

func encodeChunk(c doginalChunk) ([]byte, error) {
	if c.data == nil {
		return []byte{c.op}, nil
	}
	return pushData(c.data)
}

func pushData(data []byte) ([]byte, error) {
	n := len(data)
	if n == 0 {
		return []byte{0x00}, nil
	}
	switch {
	case n <= 75:
		return append([]byte{byte(n)}, data...), nil
	case n <= 255:
		return append([]byte{0x4c, byte(n)}, data...), nil
	case n <= 65535:
		return append([]byte{0x4d, byte(n), byte(n >> 8)}, data...), nil
	default:
		return nil, fmt.Errorf("push too large: %d", n)
	}
}

func chunksToScript(chunks []doginalChunk) ([]byte, error) {
	var out []byte
	for _, c := range chunks {
		b, err := encodeChunk(c)
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	return out, nil
}

func scriptLen(chunks []doginalChunk) (int, error) {
	b, err := chunksToScript(chunks)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

// PackDoginalPartials splits inscription chunks into apezord partial scripts.
func PackDoginalPartials(chunks []doginalChunk) ([][]doginalChunk, error) {
	remaining := append([]doginalChunk(nil), chunks...)
	var partials [][]doginalChunk
	first := true
	for len(remaining) > 0 {
		var partial []doginalChunk
		if first {
			partial = append(partial, remaining[0])
			remaining = remaining[1:]
			first = false
		}
		for len(remaining) >= 2 {
			trial := append(append([]doginalChunk(nil), partial...), remaining[0], remaining[1])
			n, err := scriptLen(trial)
			if err != nil {
				return nil, err
			}
			if n > maxDoginalPayloadLen {
				break
			}
			partial = trial
			remaining = remaining[2:]
		}
		if len(partial) == 0 {
			return nil, fmt.Errorf("unable to pack doginal partial (chunk too large)")
		}
		partials = append(partials, partial)
	}
	return partials, nil
}

// BuildDoginalLockRedeem builds <pubkey> CHECKSIGVERIFY DROP* TRUE.
func BuildDoginalLockRedeem(pubkey []byte, drops int) ([]byte, error) {
	if len(pubkey) != 33 && len(pubkey) != 65 {
		return nil, fmt.Errorf("pubkey must be 33 or 65 bytes")
	}
	if drops < 1 {
		return nil, fmt.Errorf("doginal lock needs at least one DROP")
	}
	pubPush, err := pushData(pubkey)
	if err != nil {
		return nil, err
	}
	out := append([]byte(nil), pubPush...)
	out = append(out, 0xad) // OP_CHECKSIGVERIFY
	for i := 0; i < drops; i++ {
		out = append(out, 0x75) // OP_DROP
	}
	out = append(out, 0x51) // OP_TRUE
	return out, nil
}

func p2shScriptPubKey(redeem []byte) []byte {
	h := chain.Hash160(redeem)
	out := []byte{0xa9, 0x14} // OP_HASH160 PUSH20
	out = append(out, h...)
	out = append(out, 0x87) // OP_EQUAL
	return out
}

func planP2SHMint(pubkey []byte, contentType string, data []byte, shVer byte) ([]p2shMintStep, error) {
	chunks, err := BuildDoginalInscriptionChunks(contentType, data)
	if err != nil {
		return nil, err
	}
	partials, err := PackDoginalPartials(chunks)
	if err != nil {
		return nil, err
	}
	steps := make([]p2shMintStep, 0, len(partials))
	for _, partial := range partials {
		partialScript, err := chunksToScript(partial)
		if err != nil {
			return nil, err
		}
		lock, err := BuildDoginalLockRedeem(pubkey, len(partial))
		if err != nil {
			return nil, err
		}
		spk := p2shScriptPubKey(lock)
		addr := chain.Base58CheckEncode(shVer, chain.Hash160(lock))
		steps = append(steps, p2shMintStep{
			PartialHex:   hex.EncodeToString(partialScript),
			RedeemHex:    hex.EncodeToString(lock),
			P2SHAddress:  addr,
			ScriptPubKey: hex.EncodeToString(spk),
			Drops:        len(partial),
		})
	}
	return steps, nil
}

// MintP2SHInscription creates classic apezord P2SH doginals on L1 via wallet_rpc.
func (e *Extension) MintP2SHInscription(host extensions.Host, raw map[string]interface{}) (map[string]interface{}, error) {
	wh, cfg, err := e.walletHost(host)
	if err != nil {
		return nil, err
	}
	addr := strings.TrimSpace(fmt.Sprint(raw["address"]))
	if addr == "" {
		addr = strings.TrimSpace(cfg.PreferredAddress)
	}
	if addr == "" {
		return nil, fmt.Errorf("address required (P2PKH signer / recipient)")
	}
	contentType := strings.TrimSpace(fmt.Sprint(raw["content_type"]))
	var body []byte
	if b64 := strings.TrimSpace(fmt.Sprint(raw["content_b64"])); b64 != "" && b64 != "<nil>" {
		body, err = decodeB64(b64)
		if err != nil {
			return nil, fmt.Errorf("content_b64: %w", err)
		}
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("select an image/file (content_b64 required for L1 P2SH mint)")
	}
	if contentType == "" {
		contentType = sniffContentType(body)
	}
	broadcast := cfg.AutoBroadcast
	if v, ok := raw["broadcast"].(bool); ok {
		broadcast = v
	} else if s := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw["broadcast"]))); s == "true" || s == "1" {
		broadcast = true
	}

	addrParam, _ := marshalParam(addr)
	infoRaw, err := wh.CallWalletRPC("getaddressinfo", []json.RawMessage{addrParam})
	if err != nil {
		return nil, fmt.Errorf("getaddressinfo: %w", err)
	}
	info, _ := infoRaw.(map[string]interface{})
	pubHex, _ := info["pubkey"].(string)
	pubHex = strings.TrimSpace(pubHex)
	if pubHex == "" {
		return nil, fmt.Errorf("wallet pubkey unavailable for %s (ismine address required)", addr)
	}
	pubkey, err := hex.DecodeString(pubHex)
	if err != nil || (len(pubkey) != 33 && len(pubkey) != 65) {
		return nil, fmt.Errorf("invalid wallet pubkey")
	}

	net := "mainnet"
	if host != nil {
		net = host.Network()
	}
	cnet, err := chain.ParseNetwork(net)
	if err != nil {
		cnet = chain.MainnetDogecoin
	}
	params, err := chain.ParamsFor(cnet)
	if err != nil {
		return nil, err
	}
	steps, err := planP2SHMint(pubkey, contentType, body, params.ScriptHashAddrID)
	if err != nil {
		return nil, err
	}

	out := map[string]interface{}{
		"destination":  "p2sh",
		"protocol":     "apezord-doginals",
		"content_type": contentType,
		"size":         len(body),
		"steps":        len(steps),
		"address":      addr,
		"broadcast":    false,
		"note":         "Classic L1 P2SH Doginals (apezord). Indexed by this extension after confirmation.",
	}
	if !broadcast {
		plan := make([]map[string]interface{}, 0, len(steps))
		for i, s := range steps {
			plan = append(plan, map[string]interface{}{
				"i": i, "p2sh_address": s.P2SHAddress, "drops": s.Drops,
				"redeem_hex": s.RedeemHex, "partial_hex": s.PartialHex,
			})
		}
		out["plan"] = plan
		out["next"] = "Set broadcast=true with wallet unlocked to fund, sign, and send the commit/reveal chain."
		return out, nil
	}

	txids := make([]string, 0, len(steps)+1)
	var prevTxid string
	var prevVout uint32
	var prevStep *p2shMintStep

	for i, step := range steps {
		inputs := []interface{}{}
		if prevTxid != "" {
			inputs = append(inputs, map[string]interface{}{"txid": prevTxid, "vout": prevVout})
		}
		outputs := map[string]interface{}{step.P2SHAddress: doginalDustDOGE}
		txid, err := e.broadcastDoginalStep(wh, inputs, outputs, prevTxid, prevVout, prevStep)
		if err != nil {
			out["txids"] = txids
			out["failed_at"] = i
			return out, fmt.Errorf("p2sh mint step %d: %w", i, err)
		}
		txids = append(txids, txid)
		prevTxid = txid
		prevVout = 0
		s := step
		prevStep = &s
	}

	// Final reveal: spend last P2SH to recipient address.
	inputs := []interface{}{map[string]interface{}{"txid": prevTxid, "vout": prevVout}}
	outputs := map[string]interface{}{addr: doginalDustDOGE}
	finalTxid, err := e.broadcastDoginalStep(wh, inputs, outputs, prevTxid, prevVout, prevStep)
	if err != nil {
		out["txids"] = txids
		return out, fmt.Errorf("p2sh mint final reveal: %w", err)
	}
	txids = append(txids, finalTxid)
	out["broadcast"] = true
	out["txids"] = txids
	out["inscription_txid"] = finalTxid
	if len(txids) > 1 {
		out["inscription_txid"] = txids[1] // matches apezord: second tx hash often cited
		out["reveal_txid"] = finalTxid
	}
	out["media_kind"] = ClassifyMediaKind(contentType, body, false)
	return out, nil
}

func (e *Extension) broadcastDoginalStep(
	wh extensions.WalletRPCHost,
	inputs []interface{},
	outputs map[string]interface{},
	prevTxid string,
	prevVout uint32,
	prevStep *p2shMintStep,
) (string, error) {
	inParam, err := marshalParam(inputs)
	if err != nil {
		return "", err
	}
	outParam, err := marshalParam(outputs)
	if err != nil {
		return "", err
	}
	rawHex, err := wh.CallWalletRPC("createrawtransaction", []json.RawMessage{inParam, outParam})
	if err != nil {
		return "", fmt.Errorf("createrawtransaction: %w", err)
	}
	hexStr, _ := rawHex.(string)
	if hexStr == "" {
		return "", fmt.Errorf("createrawtransaction returned empty hex")
	}
	hexParam, _ := marshalParam(hexStr)
	funded, err := wh.CallWalletRPC("fundrawtransaction", []json.RawMessage{hexParam})
	if err != nil {
		return "", fmt.Errorf("fundrawtransaction: %w", err)
	}
	fundedMap, _ := funded.(map[string]interface{})
	fundedHex, _ := fundedMap["hex"].(string)
	if fundedHex == "" {
		return "", fmt.Errorf("fundrawtransaction missing hex")
	}

	signParams := []json.RawMessage{}
	fh, _ := marshalParam(fundedHex)
	signParams = append(signParams, fh)
	if prevStep != nil && prevTxid != "" {
		prev := []map[string]interface{}{{
			"txid":           prevTxid,
			"vout":           prevVout,
			"scriptPubKey":   prevStep.ScriptPubKey,
			"redeemScript":   prevStep.RedeemHex,
			"doginalPartial": prevStep.PartialHex,
		}}
		prevParam, _ := marshalParam(prev)
		signParams = append(signParams, prevParam)
	}
	signed, err := wh.CallWalletRPC("signrawtransactionwithwallet", signParams)
	if err != nil {
		return "", fmt.Errorf("signrawtransactionwithwallet: %w", err)
	}
	signedMap, _ := signed.(map[string]interface{})
	signedHex, _ := signedMap["hex"].(string)
	complete, _ := signedMap["complete"].(bool)
	if !complete || signedHex == "" {
		b, _ := json.Marshal(signed)
		return "", fmt.Errorf("wallet could not fully sign doginal step: %s", string(b))
	}
	sh, _ := marshalParam(signedHex)
	txidRaw, err := wh.CallWalletRPC("sendrawtransaction", []json.RawMessage{sh})
	if err != nil {
		return "", fmt.Errorf("sendrawtransaction: %w", err)
	}
	txid, _ := txidRaw.(string)
	if txid == "" {
		return "", fmt.Errorf("sendrawtransaction returned empty txid")
	}
	return txid, nil
}
