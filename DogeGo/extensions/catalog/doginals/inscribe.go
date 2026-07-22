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

	"dogego/extensions"
)

// BuildDRC20JSON builds canonical deploy/mint/transfer JSON (OP_RETURN payload).
func BuildDRC20JSON(op, tick, amt, max, lim string) ([]byte, error) {
	op = strings.ToLower(strings.TrimSpace(op))
	tick = strings.ToLower(strings.TrimSpace(tick))
	if tick == "" {
		return nil, fmt.Errorf("tick required (usually 4 letters)")
	}
	if len(tick) > 8 {
		return nil, fmt.Errorf("tick too long")
	}
	m := map[string]string{"p": "drc-20", "op": op, "tick": tick}
	switch op {
	case "deploy":
		max = strings.TrimSpace(max)
		if max == "" {
			return nil, fmt.Errorf("deploy requires max supply")
		}
		m["max"] = max
		if lim = strings.TrimSpace(lim); lim != "" {
			m["lim"] = lim
		}
	case "mint", "transfer":
		amt = strings.TrimSpace(amt)
		if amt == "" {
			return nil, fmt.Errorf("%s requires amt", op)
		}
		m["amt"] = amt
	default:
		return nil, fmt.Errorf("op must be deploy|mint|transfer")
	}
	return json.Marshal(m)
}

// PreviewInscription returns payload text/hex without broadcasting.
func PreviewInscription(op, tick, amt, max, lim string) (map[string]interface{}, error) {
	raw, err := BuildDRC20JSON(op, tick, amt, max, lim)
	if err != nil {
		return nil, err
	}
	if len(raw) > 80 {
		return nil, fmt.Errorf("payload %d bytes exceeds OP_RETURN limit (80); shorten fields", len(raw))
	}
	return map[string]interface{}{
		"op":          op,
		"json":        string(raw),
		"payload_hex": hex.EncodeToString(raw),
		"bytes":       len(raw),
		"note":        "OP_RETURN inscription path (matches doginals L1 indexer). Unlock wallet + enable wallet RPC in Settings to broadcast.",
	}, nil
}

func (e *Extension) walletHost(host extensions.Host) (extensions.WalletRPCHost, ExtConfig, error) {
	st, err := e.storeOrErr()
	if err != nil {
		return nil, ExtConfig{}, err
	}
	cfg := st.GetConfig()
	if !cfg.WalletRPCEnabled {
		return nil, cfg, fmt.Errorf("wallet RPC disabled in extension Settings (enable wallet_rpc_enabled)")
	}
	if host == nil {
		e.mu.Lock()
		host = e.host
		e.mu.Unlock()
	}
	wh, ok := host.(extensions.WalletRPCHost)
	if !ok || wh == nil {
		return nil, cfg, fmt.Errorf("wallet_rpc not available: declare wallet_rpc permission and use authenticated DogeGo RPC")
	}
	return wh, cfg, nil
}

func marshalParam(v interface{}) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// InscribeDRC20 builds an OP_RETURN DRC-20 tx via wallet RPC and optionally broadcasts.
func (e *Extension) InscribeDRC20(host extensions.Host, op, tick, amt, max, lim string, broadcast bool) (map[string]interface{}, error) {
	preview, err := PreviewInscription(op, tick, amt, max, lim)
	if err != nil {
		return nil, err
	}
	wh, cfg, err := e.walletHost(host)
	if err != nil {
		preview["broadcast"] = false
		preview["wallet_error"] = err.Error()
		return preview, nil
	}
	payloadHex, _ := preview["payload_hex"].(string)
	inputs, _ := marshalParam([]interface{}{})
	outputs, _ := marshalParam(map[string]interface{}{"data": payloadHex})
	rawHex, err := wh.CallWalletRPC("createrawtransaction", []json.RawMessage{inputs, outputs})
	if err != nil {
		return nil, fmt.Errorf("createrawtransaction: %w", err)
	}
	hexStr, _ := rawHex.(string)
	if hexStr == "" {
		return nil, fmt.Errorf("createrawtransaction returned empty hex")
	}
	hexParam, _ := marshalParam(hexStr)
	funded, err := wh.CallWalletRPC("fundrawtransaction", []json.RawMessage{hexParam})
	if err != nil {
		return nil, fmt.Errorf("fundrawtransaction: %w (unlock wallet via dashboard passphrase)", err)
	}
	fundedMap, _ := funded.(map[string]interface{})
	fundedHex, _ := fundedMap["hex"].(string)
	if fundedHex == "" {
		return nil, fmt.Errorf("fundrawtransaction missing hex")
	}
	out := map[string]interface{}{
		"preview":     preview,
		"funded_hex":  fundedHex,
		"fee":         fundedMap["fee"],
		"changepos":   fundedMap["changepos"],
		"preferred_address": cfg.PreferredAddress,
		"broadcast":   false,
	}
	doBroadcast := broadcast || cfg.AutoBroadcast
	if !doBroadcast {
		out["next"] = "Review funded_hex, then call inscribe with broadcast=true (wallet must stay unlocked)."
		return out, nil
	}
	fh, _ := marshalParam(fundedHex)
	signed, err := wh.CallWalletRPC("signrawtransactionwithwallet", []json.RawMessage{fh})
	if err != nil {
		return nil, fmt.Errorf("signrawtransactionwithwallet: %w", err)
	}
	signedMap, _ := signed.(map[string]interface{})
	signedHex, _ := signedMap["hex"].(string)
	complete, _ := signedMap["complete"].(bool)
	if !complete || signedHex == "" {
		out["signed"] = signed
		return nil, fmt.Errorf("wallet could not fully sign transaction")
	}
	sh, _ := marshalParam(signedHex)
	txid, err := wh.CallWalletRPC("sendrawtransaction", []json.RawMessage{sh})
	if err != nil {
		return nil, fmt.Errorf("sendrawtransaction: %w", err)
	}
	out["broadcast"] = true
	out["txid"] = txid
	out["signed_hex"] = signedHex
	return out, nil
}
