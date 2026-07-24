// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package radiodoge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Host is the subset of DogeGo extension host calls RadioDoge needs.
type Host interface {
	Log(line string)
	CallWalletRPC(method string, args ...interface{}) (interface{}, error)
}

// Service owns SoftAP polling, outbound broadcast, and inbound relay.
type Service struct {
	store *configStore
	host  Host

	mu             sync.Mutex
	deviceOK       bool
	deviceReady    bool
	lastStatus     string
	lastError      string
	lastProbeUTC   time.Time
	internetOK     bool
	broadcasts     int
	relays         int
	confirmations  int
	recent         []Activity
	relayed        map[string]time.Time // tx hex hash -> when
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// Activity is a recent TX/RX event for the UI table.
type Activity struct {
	UTC    string `json:"utc"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
	OK     bool   `json:"ok"`
}

func NewService(dataDir string, host Host) *Service {
	return &Service{
		store:   loadConfig(dataDir),
		host:    host,
		relayed: make(map[string]time.Time),
	}
}

func (s *Service) Start() {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.loop(ctx)
	}()
}

func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

func (s *Service) Config() Config { return s.store.Get() }

func (s *Service) SetConfig(patch map[string]interface{}) (Config, error) {
	err := s.store.Update(func(c *Config) {
		if v, ok := patch["base_url"].(string); ok {
			c.BaseURL = strings.TrimSpace(v)
		}
		if v, ok := asBool(patch["enabled"]); ok {
			c.Enabled = v
		}
		if v, ok := asBool(patch["prefer_radio_offline"]); ok {
			c.PreferRadioOffline = v
		}
		if v, ok := asBool(patch["force_radio"]); ok {
			c.ForceRadio = v
		}
		if v, ok := asBool(patch["auto_relay_inbound"]); ok {
			c.AutoRelayInbound = v
		}
		if v, ok := asBool(patch["confirm_via_logs"]); ok {
			c.ConfirmViaLogs = v
		}
		if v, ok := asInt(patch["poll_seconds"]); ok {
			c.PollSeconds = v
		}
		if v, ok := patch["internet_probe_url"].(string); ok {
			c.InternetProbeURL = strings.TrimSpace(v)
		}
		if v, ok := patch["gateway_type"].(string); ok {
			c.GatewayType = strings.TrimSpace(v)
		}
		if v, ok := patch["gateway_ip"].(string); ok {
			c.GatewayIP = strings.TrimSpace(v)
		}
		if v, ok := patch["gateway_port"].(string); ok {
			c.GatewayPort = strings.TrimSpace(v)
		}
		if v, ok := patch["gateway_user"].(string); ok {
			c.GatewayUser = strings.TrimSpace(v)
		}
		if v, ok := patch["gateway_password"].(string); ok {
			c.GatewayPassword = v
		}
		if v, ok := patch["gateway_endpoint"].(string); ok {
			c.GatewayEndpoint = strings.TrimSpace(v)
		}
		if v, ok := patch["direct_target_address"].(string); ok {
			c.DirectTargetAddress = strings.TrimSpace(v)
		}
	})
	return s.store.Get(), err
}

func (s *Service) device() *Device {
	return NewDevice(s.store.Get().BaseURL)
}

func (s *Service) loop(ctx context.Context) {
	s.tick(ctx)
	for {
		cfg := s.store.Get()
		wait := time.Duration(cfg.PollSeconds) * time.Second
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
			s.tick(ctx)
		}
	}
}

func (s *Service) tick(ctx context.Context) {
	cfg := s.store.Get()
	if !cfg.Enabled {
		s.mu.Lock()
		s.deviceOK = false
		s.deviceReady = false
		s.mu.Unlock()
		return
	}
	d := s.device()
	reachable := d.Reachable(ctx)
	ready := false
	statusRaw := ""
	var statusErr string
	if reachable {
		_, raw, rdy, err := d.StatusJSON(ctx)
		statusRaw = raw
		ready = rdy
		if err != nil {
			statusErr = err.Error()
		}
	} else {
		statusErr = "device not reachable at " + cfg.BaseURL
	}
	inet := HasInternet(ctx, cfg.InternetProbeURL)
	s.mu.Lock()
	s.deviceOK = reachable
	s.deviceReady = ready
	s.lastStatus = truncate(statusRaw, 400)
	s.lastError = statusErr
	s.internetOK = inet
	s.lastProbeUTC = time.Now().UTC()
	s.mu.Unlock()

	if cfg.AutoRelayInbound && reachable {
		s.relayInbound(ctx, d)
	}
}

func (s *Service) relayInbound(ctx context.Context, d *Device) {
	if s.host == nil {
		return
	}
	logs, err := d.Logs(ctx)
	if err != nil {
		return
	}
	for _, hexTx := range ExtractTxHexCandidates(logs) {
		key := hashKey(hexTx)
		s.mu.Lock()
		if _, ok := s.relayed[key]; ok {
			s.mu.Unlock()
			continue
		}
		s.relayed[key] = time.Now().UTC()
		s.mu.Unlock()
		txid, err := s.sendLocal(hexTx)
		ok := err == nil
		detail := txid
		if err != nil {
			detail = err.Error()
			s.mu.Lock()
			delete(s.relayed, key) // allow retry
			s.mu.Unlock()
		} else {
			s.mu.Lock()
			s.relays++
			s.mu.Unlock()
			if s.host != nil {
				s.host.Log("radiodoge: relayed inbound tx " + txid)
			}
		}
		s.pushActivity("relay_in", detail, ok)
	}
}

func (s *Service) sendLocal(hexTx string) (string, error) {
	out, err := s.host.CallWalletRPC("sendrawtransaction", hexTx)
	if err != nil {
		return "", err
	}
	switch v := out.(type) {
	case string:
		return v, nil
	default:
		raw, _ := json.Marshal(v)
		return strings.Trim(string(raw), `"`), nil
	}
}

// ShouldUseRadio mirrors dogecoin-wallet: enabled + no internet + device up.
func (s *Service) ShouldUseRadio(ctx context.Context) (bool, string) {
	cfg := s.store.Get()
	if !cfg.Enabled {
		return false, "RadioDoge disabled in extension settings"
	}
	d := s.device()
	if !d.Reachable(ctx) {
		return false, "RadioDoge SoftAP not reachable at " + cfg.BaseURL
	}
	if cfg.ForceRadio {
		return true, "force_radio"
	}
	if cfg.PreferRadioOffline {
		if HasInternet(ctx, cfg.InternetProbeURL) {
			return false, "internet available; use normal P2P/RPC broadcast"
		}
		return true, "offline + RadioDoge reachable"
	}
	return false, "prefer_radio_offline is off"
}

// BroadcastHex sends via SoftAP /api/broadcast (mesh). Optionally waits for log confirmation.
func (s *Service) BroadcastHex(ctx context.Context, hexTx, expectTxid string) (map[string]interface{}, error) {
	cfg := s.store.Get()
	if !cfg.Enabled {
		return nil, fmt.Errorf("RadioDoge disabled")
	}
	d := s.device()
	body, err := d.BroadcastTransaction(ctx, hexTx)
	ok := err == nil
	detail := "broadcast"
	if err != nil {
		detail = err.Error()
	}
	s.pushActivity("broadcast", detail, ok)
	if err != nil {
		return map[string]interface{}{"ok": false, "response": body}, err
	}
	s.mu.Lock()
	s.broadcasts++
	s.mu.Unlock()

	confirmed := false
	if cfg.ConfirmViaLogs && expectTxid != "" {
		time.Sleep(3 * time.Second)
		logs, lerr := d.Logs(ctx)
		if lerr == nil && MatchConfirmation(logs, expectTxid) {
			confirmed = true
			s.mu.Lock()
			s.confirmations++
			s.mu.Unlock()
			s.pushActivity("confirm", expectTxid, true)
		}
	}
	return map[string]interface{}{
		"ok":        true,
		"response":  truncate(body, 500),
		"confirmed": confirmed,
		"path":      "radiodoge_broadcast",
	}, nil
}

// BroadcastSmart chooses local sendrawtransaction or RadioDoge mesh.
func (s *Service) BroadcastSmart(ctx context.Context, hexTx, expectTxid string) (map[string]interface{}, error) {
	use, reason := s.ShouldUseRadio(ctx)
	if use {
		out, err := s.BroadcastHex(ctx, hexTx, expectTxid)
		if out == nil {
			out = map[string]interface{}{}
		}
		out["reason"] = reason
		return out, err
	}
	if s.host == nil {
		return nil, fmt.Errorf("no wallet host; enable wallet_rpc and unlock wallet")
	}
	txid, err := s.sendLocal(hexTx)
	if err != nil {
		// Fall back to RadioDoge if device is up.
		d := s.device()
		if d.Reachable(ctx) {
			out, err2 := s.BroadcastHex(ctx, hexTx, expectTxid)
			if out == nil {
				out = map[string]interface{}{}
			}
			out["reason"] = "local broadcast failed; fell back to RadioDoge: " + err.Error()
			return out, err2
		}
		return nil, err
	}
	s.pushActivity("local", txid, true)
	return map[string]interface{}{
		"ok":     true,
		"txid":   txid,
		"path":   "sendrawtransaction",
		"reason": reason,
	}, nil
}

// SendDirect posts to /api/transaction for a LoRa node address.
func (s *Service) SendDirect(ctx context.Context, address, hexTx string) (map[string]interface{}, error) {
	if address == "" {
		address = s.store.Get().DirectTargetAddress
	}
	body, err := s.device().SendDirectTransaction(ctx, address, hexTx)
	ok := err == nil
	s.pushActivity("direct", address, ok)
	if err != nil {
		return map[string]interface{}{"ok": false, "response": body}, err
	}
	return map[string]interface{}{"ok": true, "response": truncate(body, 500), "address": address}, nil
}

// ConfigureGateway pushes gateway settings to the SoftAP device.
func (s *Service) ConfigureGateway(ctx context.Context) (map[string]interface{}, error) {
	cfg := s.store.Get()
	body, err := s.device().SaveGateway(ctx, cfg.GatewayType, cfg.GatewayIP, cfg.GatewayPort, cfg.GatewayUser, cfg.GatewayPassword, cfg.GatewayEndpoint)
	if err != nil {
		return map[string]interface{}{"ok": false, "response": body}, err
	}
	return map[string]interface{}{"ok": true, "response": truncate(body, 500)}, nil
}

// Snapshot returns status for info / UI.
func (s *Service) Snapshot() map[string]interface{} {
	cfg := s.store.Get()
	s.mu.Lock()
	defer s.mu.Unlock()
	recent := append([]Activity(nil), s.recent...)
	return map[string]interface{}{
		"extension":      "dogego.radiodoge",
		"firmware":       "heltec-firmware-v3 SoftAP HTTP",
		"base_url":       cfg.BaseURL,
		"enabled":        cfg.Enabled,
		"device_ok":      s.deviceOK,
		"device_ready":   s.deviceReady,
		"internet_ok":    s.internetOK,
		"last_probe_utc": s.lastProbeUTC.Format(time.RFC3339),
		"last_error":     s.lastError,
		"broadcasts":     s.broadcasts,
		"relays":         s.relays,
		"confirmations":  s.confirmations,
		"config":         cfg,
		"recent":         recent,
		"docs":           "https://github.com/dogecoinfoundation/radiodoge/tree/0.0.1-Beta-1/heltec-firmware-v3",
	}
}

func (s *Service) pushActivity(kind, detail string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recent = append([]Activity{{
		UTC:    time.Now().UTC().Format(time.RFC3339),
		Kind:   kind,
		Detail: truncate(detail, 120),
		OK:     ok,
	}}, s.recent...)
	if len(s.recent) > 40 {
		s.recent = s.recent[:40]
	}
}

func hashKey(hexTx string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(hexTx))))
	return hex.EncodeToString(sum[:8])
}

func asBool(v interface{}) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "1", "true", "yes", "on":
			return true, true
		case "0", "false", "no", "off":
			return false, true
		}
	case float64:
		return t != 0, true
	}
	return false, false
}

func asInt(v interface{}) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case string:
		var n int
		_, err := fmt.Sscanf(strings.TrimSpace(t), "%d", &n)
		return n, err == nil
	}
	return 0, false
}

// BuildUI returns host-rendered workspace JSON.
func BuildUI(snap map[string]interface{}) map[string]interface{} {
	cfg, _ := snap["config"].(Config)
	deviceOK, _ := snap["device_ok"].(bool)
	ready, _ := snap["device_ready"].(bool)
	inet, _ := snap["internet_ok"].(bool)
	tone := "warn"
	state := "Searching"
	if deviceOK && ready {
		tone = "ok"
		state = "Ready"
	} else if deviceOK {
		tone = "warn"
		state = "Reachable"
	} else if !cfg.Enabled {
		tone = "neutral"
		state = "Disabled"
	}
	inetLabel := "No"
	inetTone := "warn"
	if inet {
		inetLabel = "Yes"
		inetTone = "ok"
	}
	recent, _ := snap["recent"].([]Activity)
	rows := make([]map[string]interface{}, 0, len(recent))
	for _, a := range recent {
		ok := "fail"
		if a.OK {
			ok = "ok"
		}
		rows = append(rows, map[string]interface{}{
			"utc": a.UTC, "kind": a.Kind, "detail": a.Detail, "ok": ok,
		})
	}
	return map[string]interface{}{
		"panel_title": "RadioDoge",
		"subtitle":    "Heltec V3 SoftAP · mesh LoRa when offline · gateway relay when online",
		"layout":      "workspace",
		"status_chips": []map[string]interface{}{
			{"id": "dev", "label": "Device", "value": state, "tone": tone, "icon": "sensors"},
			{"id": "net", "label": "Internet", "value": inetLabel, "tone": inetTone, "icon": "public"},
			{"id": "tx", "label": "Broadcasts", "value": fmt.Sprintf("%v", snap["broadcasts"]), "tone": "neutral", "icon": "cell_tower"},
			{"id": "rx", "label": "Relays", "value": fmt.Sprintf("%v", snap["relays"]), "tone": "neutral", "icon": "input"},
		},
		"nav": []map[string]interface{}{
			{"id": "home", "label": "Home", "icon": "home"},
			{"id": "tools", "label": "Tools", "icon": "construction"},
			{"id": "settings", "label": "Settings", "icon": "tune"},
		},
		"sections": map[string]interface{}{
			"home": map[string]interface{}{
				"title": "Overview",
				"lead":  "Join WiFi SSID RadioDoge (AP 192.168.4.1) or keep SoftAP reachable from this host. Same HTTP API as Dogecoin Wallet RadioDoge support.",
				"widgets": []map[string]interface{}{
					{"type": "stats", "items": []map[string]interface{}{
						{"label": "Confirmations", "value": fmt.Sprintf("%v", snap["confirmations"]), "icon": "verified"},
						{"label": "Broadcasts", "value": fmt.Sprintf("%v", snap["broadcasts"]), "icon": "cell_tower"},
						{"label": "Relays", "value": fmt.Sprintf("%v", snap["relays"]), "icon": "input"},
					}},
					{
						"type":  "callout",
						"tone":  "neutral",
						"icon":  "link",
						"title": "SoftAP base URL",
						"body":  cfg.BaseURL,
					},
					{
						"type":  "callout",
						"tone":  "neutral",
						"icon":  "info",
						"title": "Modes",
						"body":  "Offline client: broadcast signed hex via LoRa mesh. Online gateway: auto-relay inbound mesh txs with sendrawtransaction. Firmware: dogecoinfoundation/radiodoge heltec-firmware-v3.",
					},
					{
						"type": "table", "title": "Recent activity", "search": true, "page_size": 8,
						"columns": []map[string]interface{}{
							{"key": "utc", "label": "UTC"},
							{"key": "kind", "label": "Kind"},
							{"key": "ok", "label": "OK"},
							{"key": "detail", "label": "Detail"},
						},
						"rows": rows,
					},
				},
				"quick_actions": []map[string]interface{}{
					{"id": "refresh", "label": "Refresh", "method": "info", "icon": "refresh"},
					{"id": "probe", "label": "Probe device", "method": "probe", "icon": "sensors"},
				},
			},
			"tools": map[string]interface{}{
				"title": "Tools",
				"lead":  "Paste a signed raw transaction hex. Prefer Broadcast (mesh) when offline.",
				"tools": []map[string]interface{}{
					{
						"id": "broadcast", "label": "Broadcast via RadioDoge", "method": "broadcast", "icon": "cell_tower",
						"params_as": "object",
						"fields": []map[string]interface{}{
							{"name": "hex", "label": "Signed tx hex", "type": "textarea", "placeholder": "01000000…"},
							{"name": "txid", "label": "Expected txid (optional confirm)", "type": "text"},
						},
					},
					{
						"id": "broadcast_smart", "label": "Smart broadcast (P2P or RadioDoge)", "method": "broadcast_smart", "icon": "alt_route",
						"params_as": "object",
						"fields": []map[string]interface{}{
							{"name": "hex", "label": "Signed tx hex", "type": "textarea"},
							{"name": "txid", "label": "Expected txid (optional)", "type": "text"},
						},
					},
					{
						"id": "send_direct", "label": "Direct LoRa transaction", "method": "send_direct", "icon": "near_me", "advanced": true,
						"params_as": "object",
						"fields": []map[string]interface{}{
							{"name": "address", "label": "LoRa address (e.g. 10.1.2)", "type": "text"},
							{"name": "hex", "label": "Signed tx hex", "type": "textarea"},
						},
					},
					{
						"id": "configure_gateway", "label": "Push gateway config to device", "method": "configure_gateway", "icon": "router", "advanced": true,
					},
					{"id": "should_use", "label": "Should use RadioDoge?", "method": "should_use_radio", "icon": "help"},
					{"id": "logs", "label": "Fetch device logs", "method": "logs", "icon": "receipt_long", "advanced": true},
				},
			},
			"settings": map[string]interface{}{
				"title": "Settings",
				"lead":  "Stored under extensions/dogego.radiodoge/data/ (survives upgrades).",
				"tools": []map[string]interface{}{
					{
						"id": "setconfig", "label": "Save preferences", "method": "setconfig", "icon": "save", "params_as": "object",
						"fields": []map[string]interface{}{
							{"name": "enabled", "label": "Enable RadioDoge", "type": "switch", "default": boolStr(cfg.Enabled)},
							{"name": "prefer_radio_offline", "label": "Prefer RadioDoge when offline", "type": "switch", "default": boolStr(cfg.PreferRadioOffline)},
							{"name": "force_radio", "label": "Always use RadioDoge", "type": "switch", "default": boolStr(cfg.ForceRadio)},
							{"name": "auto_relay_inbound", "label": "Auto-relay inbound mesh txs", "type": "switch", "default": boolStr(cfg.AutoRelayInbound)},
							{"name": "confirm_via_logs", "label": "Confirm broadcasts via /api/logs", "type": "switch", "default": boolStr(cfg.ConfirmViaLogs)},
							{"name": "base_url", "label": "SoftAP base URL", "type": "text", "default": cfg.BaseURL},
							{"name": "poll_seconds", "label": "Poll interval (seconds)", "type": "text", "default": fmt.Sprintf("%d", cfg.PollSeconds)},
							{"name": "internet_probe_url", "label": "Internet probe URL", "type": "text", "default": cfg.InternetProbeURL},
							{"name": "direct_target_address", "label": "Default LoRa target", "type": "text", "default": cfg.DirectTargetAddress},
							{"name": "gateway_type", "label": "Gateway type (core/dogebox/wallet/custom)", "type": "text", "default": cfg.GatewayType},
							{"name": "gateway_ip", "label": "Gateway IP", "type": "text", "default": cfg.GatewayIP},
							{"name": "gateway_port", "label": "Gateway port", "type": "text", "default": cfg.GatewayPort},
							{"name": "gateway_user", "label": "Gateway RPC user", "type": "text", "default": cfg.GatewayUser},
							{"name": "gateway_password", "label": "Gateway RPC password", "type": "password"},
							{"name": "gateway_endpoint", "label": "Custom gateway endpoint", "type": "text", "default": cfg.GatewayEndpoint},
						},
					},
				},
			},
		},
	}
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
