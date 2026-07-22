// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dogego/extensions"
	"dogego/pow"
	"dogego/wire"
)

// Extension implements dogego.doginals (L1 observe index + L2 asset overlay).
type Extension struct {
	manifest extensions.Manifest
	mu       sync.Mutex
	store    *Store
	host     extensions.Host
}

// NewExtension builds the extension.
func NewExtension(m extensions.Manifest) (extensions.Extension, error) {
	if m.ID != "" && m.ID != ExtensionID {
		return nil, fmt.Errorf("doginals: unexpected id %q", m.ID)
	}
	return &Extension{manifest: DefaultManifest()}, nil
}

// DefaultManifest is the catalog entry.
func DefaultManifest() extensions.Manifest {
	return extensions.Manifest{
		ManifestVersion:  extensions.ManifestVersion,
		ID:               ExtensionID,
		Name:             "Doginals / DRC-20 L2",
		Version:          "0.3.0",
		Author:           "DogeGo",
		Description:      "Experimental L2 for Doginals and DRC-20: modern forms, searchable tables, metrics, settings backup. Index L1 inscriptions/tokens; mint via wallet RPC. Does not change Dogecoin consensus.",
		Homepage:         "https://github.com/qlpqlp/dogego",
		Repository:       "https://github.com/qlpqlp/dogego/tree/main/DogeGo/extensions/catalog/doginals",
		DogeGoMinVersion: "0.1.0",
		Permissions:      []string{"chain_read", "chain_index", "datadir_write", "rpc_register", "p2p_extension", "ui_panel", "wallet_rpc"},
		Networks:         []string{"mainnet", "testnet"},
		UI:               extensions.ManifestUI{StatusMethod: "info"},
		Entry: extensions.Entry{
			Type:   extensions.EntrySubprocess,
			Module: ExtensionID,
			Binary: "doginals-ext",
		},
		Capabilities: []string{"rpc", "indexer", "l2_sync", "p2p", "ui_panel", "doginals", "drc20", "wallet_rpc"},
		DocsPath:     "extensions/catalog/doginals/docs/USER_GUIDE.md",
		Icon:         "icon.png",
		RPC: []extensions.RPCMethod{
			{Name: "info", Help: "Status, index height, counts, config, and modern UI workspace."},
			{Name: "listinscriptions", Help: "List recent L1-indexed inscriptions / DRC-20 events. Param: [limit?]."},
			{Name: "getinscription", Help: "Fetch one inscription by id. Param: [id]."},
			{Name: "indexrange", Help: "Scan L1 heights into the local index. Params: [from_height, to_height]."},
			{Name: "listtokens", Help: "List indexed DRC-20 token summaries. Param: [limit?]."},
			{Name: "gettoken", Help: "Token summary by ticker. Param: [tick]."},
			{Name: "listbytick", Help: "Inscription events for a ticker. Params: [tick, limit?]."},
			{Name: "previewinscription", Help: "Build DRC-20 JSON/hex without broadcasting. Params: [op, tick, amt?, max?, lim?] or object."},
			{Name: "inscribe", Help: "Fund/sign/optional broadcast OP_RETURN DRC-20 via wallet_rpc. Params: object {op,tick,amt,max,lim,broadcast}."},
			{Name: "putasset", Help: "Create/update an off-L1 L2 asset (nft|token|image|collection). Param: [asset_object]."},
			{Name: "getasset", Help: "Fetch L2 asset by id. Param: [id]."},
			{Name: "listassets", Help: "List recent L2 assets. Param: [limit?]."},
			{Name: "getconfig", Help: "Extension settings (wallet RPC toggle, preferred address)."},
			{Name: "setconfig", Help: "Save extension settings. Param: [config_object]."},
			{Name: "exportbackup", Help: "Write settings backup under data/backups/ and return JSON."},
			{Name: "importbackup", Help: "Restore settings from backup JSON. Param: [backup_object]."},
			{Name: "syncstatus", Help: "Overlay protocol and peer hint."},
		},
	}
}

func (e *Extension) Manifest() extensions.Manifest {
	if e.manifest.ID == "" {
		e.manifest = DefaultManifest()
	}
	return e.manifest
}

func (e *Extension) OnEnable(_ context.Context, host extensions.Host) error {
	if host == nil {
		return fmt.Errorf("doginals: host required")
	}
	dir, err := host.ExtensionDataDir(ExtensionID)
	if err != nil {
		return err
	}
	st, err := OpenStore(filepath.Clean(dir))
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.store = st
	e.host = host
	e.mu.Unlock()
	host.Log("doginals: enabled (L1 index + L2 assets; no consensus change)")
	return nil
}

func (e *Extension) OnDisable() error {
	e.mu.Lock()
	st := e.store
	e.store = nil
	e.host = nil
	e.mu.Unlock()
	if st != nil {
		return st.Close()
	}
	return nil
}

func (e *Extension) storeOrErr() (*Store, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.store == nil {
		return nil, fmt.Errorf("doginals: not enabled")
	}
	return e.store, nil
}

// OnBlockConnected indexes one new tip block (observe-only).
func (e *Extension) OnBlockConnected(height int64, host extensions.Host) error {
	if host == nil {
		e.mu.Lock()
		host = e.host
		e.mu.Unlock()
	}
	if host == nil {
		return nil
	}
	return e.indexHeight(host, height)
}

func (e *Extension) indexHeight(host extensions.Host, height int64) error {
	st, err := e.storeOrErr()
	if err != nil {
		return err
	}
	raw, err := host.GetRawBlockByHeight(height)
	if err != nil || len(raw) < 80 {
		return err
	}
	n := 0
	_ = wire.ForEachBlockTx(raw, func(_ uint32, tx *wire.Tx) error {
		txid := TxDisplayHex(tx.TxHash())
		for vout, o := range tx.Vout {
			ins, ok := DetectInscriptionFromOutput(height, txid, uint32(vout), o)
			if !ok {
				continue
			}
			if err := st.PutInscription(ins); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	_ = st.SetIndexHeight(height)
	if n > 0 {
		host.Log(fmt.Sprintf("doginals: height %d indexed %d inscription(s)", height, n))
	}
	return nil
}

// HandleRPC dispatches extension methods.
func (e *Extension) HandleRPC(method string, params []json.RawMessage, host extensions.Host) (interface{}, error) {
	st, err := e.storeOrErr()
	if err != nil {
		return nil, err
	}
	switch method {
	case "info", "ui_status":
		return e.info(host, st), nil
	case "listinscriptions":
		limit := 40
		if len(params) > 0 {
			_ = json.Unmarshal(params[0], &limit)
		}
		return st.ListInscriptions(limit)
	case "getinscription":
		id, err := stringParam(params, 0)
		if err != nil {
			return nil, err
		}
		ins, ok, err := st.GetInscription(id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("inscription not found")
		}
		return ins, nil
	case "indexrange":
		if host == nil {
			return nil, fmt.Errorf("host required")
		}
		var from, to int64
		if len(params) < 2 {
			return nil, fmt.Errorf("params: [from_height, to_height]")
		}
		if err := json.Unmarshal(params[0], &from); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(params[1], &to); err != nil {
			return nil, err
		}
		if to < from {
			from, to = to, from
		}
		if to-from > 5000 {
			return nil, fmt.Errorf("range too large (max 5000 heights)")
		}
		scanned := 0
		for h := from; h <= to; h++ {
			if err := e.indexHeight(host, h); err != nil {
				return map[string]interface{}{"ok": false, "through": h - 1, "error": err.Error(), "scanned": scanned}, nil
			}
			scanned++
		}
		return map[string]interface{}{"ok": true, "from": from, "to": to, "scanned": scanned, "inscriptions": st.CountInscriptions()}, nil
	case "putasset":
		a, err := parseAssetParams(params)
		if err != nil {
			return nil, err
		}
		a, err = NormalizeAsset(a)
		if err != nil {
			return nil, err
		}
		if err := st.PutAsset(a); err != nil {
			return nil, err
		}
		return a, nil
	case "getasset":
		id, err := stringParam(params, 0)
		if err != nil {
			return nil, err
		}
		a, ok, err := st.GetAsset(id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("asset not found")
		}
		return a, nil
	case "listassets":
		limit := 40
		if len(params) > 0 {
			_ = json.Unmarshal(params[0], &limit)
		}
		return st.ListAssets(limit)
	case "listtokens":
		limit := 40
		if len(params) > 0 {
			_ = json.Unmarshal(params[0], &limit)
		}
		return st.ListTokens(limit)
	case "gettoken":
		tick, err := stringParam(params, 0)
		if err != nil {
			return nil, err
		}
		tok, ok, err := st.GetToken(tick)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("token not found")
		}
		return tok, nil
	case "listbytick":
		tick, err := stringParam(params, 0)
		if err != nil {
			return nil, err
		}
		limit := 40
		if len(params) > 1 {
			_ = json.Unmarshal(params[1], &limit)
		}
		return st.ListByTick(tick, limit)
	case "previewinscription":
		op, tick, amt, max, lim, _, err := parseInscribeParams(params)
		if err != nil {
			return nil, err
		}
		return PreviewInscription(op, tick, amt, max, lim)
	case "inscribe":
		op, tick, amt, max, lim, broadcast, err := parseInscribeParams(params)
		if err != nil {
			return nil, err
		}
		return e.InscribeDRC20(host, op, tick, amt, max, lim, broadcast)
	case "getconfig":
		return st.GetConfig(), nil
	case "setconfig":
		cfg, err := parseConfigParams(params, st.GetConfig())
		if err != nil {
			return nil, err
		}
		if err := st.SetConfig(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	case "exportbackup":
		return st.ExportBackup()
	case "importbackup":
		raw, err := parseBackupParams(params)
		if err != nil {
			return nil, err
		}
		cfg, err := st.ImportBackup(raw)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"ok": true, "config": cfg}, nil
	case "syncstatus":
		return map[string]interface{}{
			"protocol_id": ProtocolID,
			"commands":    []string{CmdDInv, CmdGetAsset, CmdAsset},
			"note":        "L2 assets sync among DogeGo peers that enable dogego.doginals. L1 index is local observe-only. Wallet mint uses authenticated wallet_rpc.",
		}, nil
	default:
		return nil, fmt.Errorf("unknown method %s", method)
	}
}

func (e *Extension) info(host extensions.Host, st *Store) map[string]interface{} {
	net := ""
	tip := int64(-1)
	if host != nil {
		net = host.Network()
		tip, _ = host.TipHeight()
	}
	cfg := st.GetConfig()
	recentTok, _ := st.ListTokens(12)
	out := map[string]interface{}{
		"extension":       ExtensionID,
		"protocol_id":     ProtocolID,
		"network":         net,
		"chain_tip":       tip,
		"index_height":    st.IndexHeight(),
		"inscriptions":    st.CountInscriptions(),
		"l2_assets":       st.CountAssets(),
		"tokens":          st.CountTokens(),
		"recent_tokens":   recentTok,
		"config":          cfg,
		"not_consensus":   true,
		"experimental":    true,
		"l1_observe_only": true,
		"l2_off_chain":    true,
		"wallet_rpc":      true,
		"recorded_unix":   time.Now().Unix(),
	}
	out["ui"] = buildUIPanel(out)
	return out
}

func stringParam(params []json.RawMessage, i int) (string, error) {
	if len(params) <= i {
		return "", fmt.Errorf("missing string param")
	}
	var s string
	if err := json.Unmarshal(params[i], &s); err != nil || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("want string param")
	}
	return strings.TrimSpace(s), nil
}

func parseInscribeParams(params []json.RawMessage) (op, tick, amt, max, lim string, broadcast bool, err error) {
	if len(params) == 0 {
		return "", "", "", "", "", false, fmt.Errorf("params required")
	}
	var obj map[string]interface{}
	if len(params) == 1 {
		if json.Unmarshal(params[0], &obj) == nil && obj["op"] != nil {
			op, _ = obj["op"].(string)
			tick, _ = obj["tick"].(string)
			amt, _ = obj["amt"].(string)
			max, _ = obj["max"].(string)
			lim, _ = obj["lim"].(string)
			switch v := obj["broadcast"].(type) {
			case bool:
				broadcast = v
			case string:
				broadcast = strings.EqualFold(v, "true") || v == "1"
			}
			return strings.TrimSpace(op), strings.TrimSpace(tick), strings.TrimSpace(amt), strings.TrimSpace(max), strings.TrimSpace(lim), broadcast, nil
		}
	}
	get := func(i int) string {
		if i >= len(params) {
			return ""
		}
		var s string
		_ = json.Unmarshal(params[i], &s)
		return strings.TrimSpace(s)
	}
	op, tick, amt, max, lim = get(0), get(1), get(2), get(3), get(4)
	if len(params) > 5 {
		var b bool
		if json.Unmarshal(params[5], &b) == nil {
			broadcast = b
		} else if strings.EqualFold(get(5), "true") {
			broadcast = true
		}
	}
	return op, tick, amt, max, lim, broadcast, nil
}

func parseConfigParams(params []json.RawMessage, base ExtConfig) (ExtConfig, error) {
	cfg := base
	if len(params) == 0 {
		return cfg, fmt.Errorf("params: [config_object]")
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(params[0], &obj); err != nil {
		// Positional: wallet_rpc_enabled, auto_broadcast, preferred_address
		get := func(i int) string {
			if i >= len(params) {
				return ""
			}
			var s string
			_ = json.Unmarshal(params[i], &s)
			return strings.TrimSpace(s)
		}
		if v := get(0); v != "" {
			cfg.WalletRPCEnabled = strings.EqualFold(v, "true") || v == "1"
		}
		if v := get(1); v != "" {
			cfg.AutoBroadcast = strings.EqualFold(v, "true") || v == "1"
		}
		cfg.PreferredAddress = get(2)
		return cfg, nil
	}
	if v, ok := obj["wallet_rpc_enabled"]; ok {
		switch t := v.(type) {
		case bool:
			cfg.WalletRPCEnabled = t
		case string:
			cfg.WalletRPCEnabled = strings.EqualFold(t, "true") || t == "1"
		}
	}
	if v, ok := obj["auto_broadcast"]; ok {
		switch t := v.(type) {
		case bool:
			cfg.AutoBroadcast = t
		case string:
			cfg.AutoBroadcast = strings.EqualFold(t, "true") || t == "1"
		}
	}
	if v, ok := obj["preferred_address"].(string); ok {
		cfg.PreferredAddress = strings.TrimSpace(v)
	}
	return cfg, nil
}

func parseBackupParams(params []json.RawMessage) (map[string]interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("params: [backup_object]")
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(params[0], &obj); err == nil && obj != nil {
		return obj, nil
	}
	var s string
	if json.Unmarshal(params[0], &s) == nil && strings.HasPrefix(strings.TrimSpace(s), "{") {
		if err := json.Unmarshal([]byte(s), &obj); err == nil && obj != nil {
			return obj, nil
		}
	}
	return nil, fmt.Errorf("want backup JSON object")
}

func parseAssetParams(params []json.RawMessage) (Asset, error) {
	var a Asset
	if len(params) == 0 {
		return a, fmt.Errorf("params: [asset_object] or field list")
	}
	// Single object form.
	if len(params) == 1 {
		if err := json.Unmarshal(params[0], &a); err == nil && (a.Name != "" || a.Kind != "") {
			return a, nil
		}
		// Maybe a JSON string of the object.
		var s string
		if json.Unmarshal(params[0], &s) == nil && strings.HasPrefix(strings.TrimSpace(s), "{") {
			if err := json.Unmarshal([]byte(s), &a); err == nil {
				return a, nil
			}
		}
	}
	// Dashboard tools send positional field values.
	get := func(i int) string {
		if i >= len(params) {
			return ""
		}
		var s string
		_ = json.Unmarshal(params[i], &s)
		return strings.TrimSpace(s)
	}
	a.Kind = get(0)
	a.Name = get(1)
	a.Description = get(2)
	a.ContentType = get(3)
	a.URI = get(4)
	a.L1InscriptionID = get(5)
	a.CreatorNote = get(6)
	if a.Name == "" {
		return a, fmt.Errorf("asset name required")
	}
	return a, nil
}

// TxDisplayHex returns explorer-style txid from wire hash bytes (LE).
func TxDisplayHex(h [32]byte) string {
	return pow.LEUint256DisplayHex(h[:])
}

// HexEncode is a tiny helper for tests/cmd.
func HexEncode(b []byte) string { return hex.EncodeToString(b) }
