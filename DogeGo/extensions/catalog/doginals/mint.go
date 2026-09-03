// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/extensions"
)

// mintPrepare builds an unsigned L2 mint + sign_message for wallets.
func (e *Extension) mintPrepare(host extensions.Host, st *Store, raw map[string]interface{}) (map[string]interface{}, error) {
	net := ""
	if host != nil {
		net = host.Network()
	}
	rec, body, err := PrepareL2Mint(raw, net)
	if err != nil {
		return nil, err
	}
	msg, err := rec.CanonicalSignMessage()
	if err != nil {
		return nil, err
	}
	signVia := "Unlock wallet, then call mintcommit with signature from signmessage(address, sign_message), or call mint with wallet_rpc enabled."
	out := map[string]interface{}{
		"destination":  "l2",
		"record":       rec,
		"sign_message": msg,
		"sign_via":     signVia,
		"size":         len(body),
		"note":         "L2 mint is wallet-signed and gossiped among doginals-v1 peers. Not Dogecoin L1 consensus. Classic P2SH/OP_RETURN are indexed from L1 only.",
	}
	if len(body) > 0 && len(body) <= 64*1024 {
		out["content_b64"] = encodeB64(body)
	}
	return out, nil
}

// mintCommit verifies signature and stores + broadcasts an L2 mint.
func (e *Extension) mintCommit(host extensions.Host, st *Store, raw map[string]interface{}) (map[string]interface{}, error) {
	if raw == nil {
		return nil, fmt.Errorf("json body required")
	}
	var rec L2MintRecord
	if r, ok := raw["record"].(map[string]interface{}); ok {
		b, _ := json.Marshal(r)
		_ = json.Unmarshal(b, &rec)
	} else {
		b, _ := json.Marshal(raw)
		_ = json.Unmarshal(b, &rec)
	}
	if sig, ok := raw["signature"].(string); ok && strings.TrimSpace(sig) != "" {
		rec.Signature = strings.TrimSpace(sig)
	}
	net := rec.Network
	if host != nil && net == "" {
		net = host.Network()
	}
	var body []byte
	if b64, ok := raw["content_b64"].(string); ok && strings.TrimSpace(b64) != "" {
		body, _ = decodeB64(strings.TrimSpace(b64))
	}
	if len(body) == 0 && rec.ContentB64 != "" {
		body, _ = decodeB64(rec.ContentB64)
	}
	accepted, body, err := AcceptL2Mint(rec, body, net)
	if err != nil {
		return nil, err
	}
	if err := st.PutL2Mint(accepted, body); err != nil {
		return nil, err
	}
	// Token ledger credit for DRC-20-style L2 mints.
	if accepted.Kind == "token" && accepted.Tick != "" && accepted.Amt != "" &&
		(accepted.Op == "mint" || accepted.Op == "deploy") {
		to := accepted.To
		if to == "" {
			to = accepted.Address
		}
		_ = st.CreditL2Balance(to, accepted.Tick, accepted.Amt)
	}
	// Gallery asset for images/files/nfts.
	a := Asset{
		ID:          "l2mint:" + accepted.ID,
		Kind:        mapMintKindToAsset(accepted.Kind),
		Name:        accepted.Name,
		ContentType: accepted.ContentType,
		URI:         accepted.URI,
		CreatorNote: "Signed L2 mint " + accepted.ID,
	}
	if len(body) > 0 && len(body) <= 256*1024 {
		a.ContentB64 = encodeB64(body)
	}
	if a2, err := NormalizeAsset(a); err == nil {
		_ = st.PutAsset(a2)
		if host != nil {
			e.broadcastAsset(host, a2.ID)
		}
	}
	if host != nil {
		e.broadcastMint(host, accepted.ID)
	}
	out := map[string]interface{}{
		"ok": true, "destination": "l2", "mint_id": accepted.ID,
		"kind": accepted.Kind, "media_kind": accepted.MediaKind,
		"address": accepted.Address, "tick": accepted.Tick, "amount": accepted.Amt,
		"content_type": accepted.ContentType, "size": accepted.Size,
		"has_content": accepted.HasContent,
		"note":        "Accepted signed L2 mint. Indexed by Doginals-enabled DogeGo peers via doginals-v1.",
	}
	return out, nil
}

// mintAuto prepares and signs via wallet_rpc when possible, then commits.
func (e *Extension) mintAuto(host extensions.Host, st *Store, raw map[string]interface{}) (map[string]interface{}, error) {
	target := strings.ToLower(strings.TrimSpace(fmt.Sprint(raw["target"])))
	if target == "" {
		target = "l2"
	}
	if target == "l1" {
		return nil, fmt.Errorf("for L1 OP_RETURN use method inscribe; P2SH is index-only (no L1 mint builder). Default mint target is L2")
	}
	// If caller already provided a signature, commit directly.
	if sig, _ := raw["signature"].(string); strings.TrimSpace(sig) != "" {
		return e.mintCommit(host, st, raw)
	}
	prep, err := e.mintPrepare(host, st, raw)
	if err != nil {
		return nil, err
	}
	rec := recordFromPrep(prep["record"])
	msg, _ := prep["sign_message"].(string)
	wh, cfg, err := e.walletHost(host)
	if err != nil || !cfg.WalletRPCEnabled {
		prep["next"] = "POST mintcommit with {record, signature, content_b64?}"
		prep["need_signature"] = true
		return prep, nil
	}
	addr := rec.Address
	if pref := strings.TrimSpace(cfg.PreferredAddress); pref != "" && addr == "" {
		addr = pref
		rec.Address = pref
	}
	addrRaw, _ := json.Marshal(addr)
	msgRaw, _ := json.Marshal(msg)
	sig, err := wh.CallWalletRPC("signmessage", []json.RawMessage{addrRaw, msgRaw})
	if err != nil {
		prep["sign_error"] = err.Error()
		prep["need_signature"] = true
		prep["next"] = "Unlock wallet (walletpassphrase) or sign sign_message externally, then mintcommit"
		return prep, nil
	}
	sigStr, _ := sig.(string)
	commitBody := map[string]interface{}{
		"record":    rec,
		"signature": sigStr,
	}
	if b64, ok := prep["content_b64"].(string); ok {
		commitBody["content_b64"] = b64
	}
	if b64, ok := raw["content_b64"].(string); ok && b64 != "" {
		commitBody["content_b64"] = b64
	}
	return e.mintCommit(host, st, commitBody)
}

func mapMintKindToAsset(kind string) string {
	switch kind {
	case "token":
		return "token"
	case "image":
		return "image"
	case "file":
		return "nft"
	default:
		return "nft"
	}
}

func decodeB64(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return base64.RawStdEncoding.DecodeString(s)
	}
	return b, nil
}

func (e *Extension) mintContentResponse(st *Store, id string) map[string]interface{} {
	r, ok, err := st.GetL2Mint(id)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if !ok {
		return map[string]interface{}{"error": "mint not found"}
	}
	body, has, _ := st.GetL2MintBody(id)
	out := map[string]interface{}{
		"id": r.ID, "kind": r.Kind, "media_kind": r.MediaKind,
		"content_type": r.ContentType, "size": r.Size, "has_content": has,
		"address": r.Address, "tick": r.Tick, "name": r.Name,
	}
	if has {
		ct := r.ContentType
		if ct == "" {
			ct = sniffContentType(body)
		}
		out["content_b64"] = encodeB64(body)
		out["data_url"] = "data:" + ct + ";base64," + encodeB64(body)
		out["content_type"] = ct
		out["size"] = len(body)
	}
	return out
}

func recordFromPrep(v interface{}) L2MintRecord {
	switch x := v.(type) {
	case L2MintRecord:
		return x
	case map[string]interface{}:
		b, _ := json.Marshal(x)
		var r L2MintRecord
		_ = json.Unmarshal(b, &r)
		return r
	default:
		return L2MintRecord{}
	}
}
