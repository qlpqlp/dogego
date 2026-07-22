// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// bbpow-ext is the DogeGo BBPoW research subprocess (line JSON-RPC on stdin/stdout).
// Testnet-only experimental extension: verifies Bitcoin-backed proofs off-chain.
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dogego/extensions/catalog/bbpow"
	"dogego/pow"
)

const maxLineBytes = 4 << 20

type rpcReq struct {
	ID     uint64            `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

func main() {
	var (
		mu       sync.Mutex
		network  string
		dataDir  string
		enabled  time.Time
		tipSeen  int64
		verified int64
		model    = bbpow.NewDualDifficultyModel()
	)

	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), maxLineBytes)
	for sc.Scan() {
		line := sc.Bytes()
		var req rpcReq
		if json.Unmarshal(line, &req) != nil {
			writeErr(0, "bad json")
			continue
		}
		switch req.Method {
		case "dogego_on_enable":
			network = envOr("DOGEGO_NETWORK", "testnet")
			dataDir = envOr("DOGEGO_DATA_DIR", "")
			if len(req.Params) > 0 {
				var meta struct {
					Network string `json:"network"`
					DataDir string `json:"data_dir"`
				}
				_ = json.Unmarshal(req.Params[0], &meta)
				if meta.Network != "" {
					network = meta.Network
				}
				if meta.DataDir != "" {
					dataDir = meta.DataDir
				}
			}
			if strings.EqualFold(network, "mainnet") {
				writeErr(req.ID, "dogego.bbpow is testnet-only research (refusing mainnet enable)")
				continue
			}
			enabled = time.Now().UTC()
			writeOK(req.ID, map[string]interface{}{
				"status":  "ready",
				"network": network,
				"mode":    "research_bbpow",
				"warning": "Does not change Dogecoin L1 consensus. BBPoW as a valid block proof would be a hard fork.",
			})
		case "dogego_on_disable":
			writeOK(req.ID, map[string]string{"status": "bye"})
			return
		case "dogego_block_connected":
			atomic.AddInt64(&tipSeen, 1)
			// L1 tips remain Scrypt/AuxPoW; count them on the scrypt research lane.
			model.RecordLaneBlock(bbpow.LaneScrypt, 0)
			writeOK(req.ID, map[string]interface{}{"indexed": true, "tip_events": atomic.LoadInt64(&tipSeen)})
		case "info", "ui_status":
			mu.Lock()
			en := enabled
			net := network
			dd := dataDir
			mu.Unlock()
			warn := model.DominanceWarning()
			summary := "EXPERIMENTAL · testnet · off-L1 · not soft-fork AuxPoW"
			if warn != "" {
				summary += " · " + warn
			}
			writeOK(req.ID, map[string]interface{}{
				"extension":       "dogego.bbpow",
				"name":            "Bitcoin-Backed PoW (research)",
				"network":         net,
				"data_dir":        dd,
				"enabled_utc":     en.Format(time.RFC3339),
				"tip_events":      atomic.LoadInt64(&tipSeen),
				"proofs_verified": atomic.LoadInt64(&verified),
				"dual_difficulty": model.Snapshot(),
				"compare_auxpow":  bbpow.CompareToAuxPoW(),
				"ui": map[string]interface{}{
					"panel_title":  "BBPoW (Bitcoin-backed research)",
					"subtitle":     summary,
					"layout":       "workspace",
					"status_chips": []map[string]interface{}{
						{"id": "mode", "label": "Mode", "value": "research", "tone": "warn", "icon": "science"},
						{"id": "net", "label": "Network", "value": net, "tone": "neutral", "icon": "lan"},
						{"id": "proofs", "label": "Verified", "value": fmt.Sprintf("%d", atomic.LoadInt64(&verified)), "tone": "ok", "icon": "verified"},
					},
					"nav": []map[string]interface{}{
						{"id": "home", "label": "Home", "icon": "home"},
						{"id": "tools", "label": "Tools", "icon": "construction"},
						{"id": "settings", "label": "Settings", "icon": "tune"},
					},
					"sections": map[string]interface{}{
						"home": map[string]interface{}{
							"title": "Overview",
							"lead":  "Off-L1 research verifier. Not consensus. Testnet only.",
							"widgets": []map[string]interface{}{
								{"type": "stats", "items": []map[string]interface{}{
									{"label": "Tip events", "value": fmt.Sprintf("%d", atomic.LoadInt64(&tipSeen)), "icon": "timeline"},
									{"label": "Proofs verified", "value": fmt.Sprintf("%d", atomic.LoadInt64(&verified)), "icon": "verified"},
								}},
								{
									"type":   "metric_chart",
									"title":  "Session activity",
									"chart":  "bar",
									"labels": []string{"Tip events", "Verified"},
									"series": []map[string]interface{}{
										{"label": "Count", "color": "#ea580c", "data": []float64{
											float64(atomic.LoadInt64(&tipSeen)),
											float64(atomic.LoadInt64(&verified)),
										}},
									},
								},
								{
									"type":  "callout",
									"tone":  "warn",
									"icon":  "science",
									"title": "Research only",
									"body":  "BBPoW does not change Dogecoin consensus. Use Tools for commitment / verify forms.",
								},
							},
							"quick_actions": []map[string]interface{}{
								{"id": "refresh", "label": "Refresh", "method": "info", "icon": "refresh"},
								{"id": "compare", "label": "Compare AuxPoW", "method": "compare", "icon": "compare_arrows"},
							},
						},
						"tools": map[string]interface{}{
							"title": "Research tools",
							"lead":  "Modern forms with collapsible advanced actions.",
							"tools": []map[string]interface{}{
								{"id": "buildcommitment", "label": "Build commitment", "method": "buildcommitment", "icon": "tag",
									"fields": []map[string]interface{}{{"name": "doge_hash", "label": "Dogecoin block hash", "type": "text", "placeholder": "64-char hex"}}},
								{"id": "verifyproof", "label": "Verify proof JSON", "method": "verifyproof", "icon": "verified",
									"fields": []map[string]interface{}{{"name": "proof_json", "label": "Proof JSON", "type": "textarea"}}},
								{"id": "dualmodel", "label": "Dual difficulty model", "method": "dualmodel", "icon": "analytics", "advanced": true},
							},
						},
						"settings": map[string]interface{}{
							"title": "Notes",
							"lead":  "No wallet_rpc required. Session counters reset on restart; package data/ is preserved across upgrades.",
							"widgets": []map[string]interface{}{
								{
									"type":  "callout",
									"tone":  "neutral",
									"icon":  "folder_special",
									"title": "Upgrade-safe data/",
									"body":  "Zip install/update keeps extensions/dogego.bbpow/data/ intact.",
								},
							},
						},
					},
				},
			})
		case "compare":
			writeOK(req.ID, bbpow.CompareToAuxPoW())
		case "dualmodel":
			out := model.Snapshot()
			if w := model.DominanceWarning(); w != "" {
				out["dominance_warning"] = w
			}
			writeOK(req.ID, out)
		case "buildcommitment":
			hash, err := firstStringParam(req.Params)
			if err != nil {
				writeErr(req.ID, err.Error())
				continue
			}
			hx, err := bbpow.BuildCommitmentHex(hash)
			if err != nil {
				writeErr(req.ID, err.Error())
				continue
			}
			writeOK(req.ID, map[string]interface{}{
				"doge_block_hash":   strings.ToLower(strings.TrimSpace(hash)),
				"commitment_hex":    hx,
				"commitment_magic":  hexMagic(),
				"embed_hint":        "Include this payload in a Bitcoin coinbase OP_RETURN (or scriptSig) when constructing a research BBPoW proof.",
				"not_consensus":     true,
			})
		case "checkheader":
			hx, err := firstStringParam(req.Params)
			if err != nil {
				writeErr(req.ID, err.Error())
				continue
			}
			raw, err := hex.DecodeString(strings.TrimSpace(hx))
			if err != nil || len(raw) != 80 {
				writeErr(req.ID, "want 80-byte header hex")
				continue
			}
			bits := binary.LittleEndian.Uint32(raw[72:76])
			if err := bbpow.CheckSHA256PoW(raw, bits); err != nil {
				writeOK(req.ID, map[string]interface{}{"ok": false, "error": err.Error(), "bits": fmt.Sprintf("%08x", bits)})
				continue
			}
			writeOK(req.ID, map[string]interface{}{
				"ok":                 true,
				"bits":               fmt.Sprintf("%08x", bits),
				"bitcoin_block_hash": pow.BlockHashHex(raw),
			})
		case "verifyproof":
			if len(req.Params) < 1 {
				writeErr(req.ID, "params: [proof_object]")
				continue
			}
			var p bbpow.Proof
			if err := json.Unmarshal(req.Params[0], &p); err != nil {
				writeErr(req.ID, "proof json: "+err.Error())
				continue
			}
			res := bbpow.ValidateProof(p)
			if res.OK {
				atomic.AddInt64(&verified, 1)
				model.RecordLaneBlock(bbpow.LaneSHA256, res.BitcoinBits)
			}
			writeOK(req.ID, res)
		case "recordlane":
			// params: ["scrypt_auxpow"|"sha256_bbpow", bits_optional]
			lane := bbpow.LaneScrypt
			var bits uint32
			if len(req.Params) >= 1 {
				var s string
				if json.Unmarshal(req.Params[0], &s) == nil && s != "" {
					lane = s
				}
			}
			if len(req.Params) >= 2 {
				var n json.Number
				if json.Unmarshal(req.Params[1], &n) == nil {
					u, _ := n.Int64()
					bits = uint32(u)
				}
			}
			model.RecordLaneBlock(lane, bits)
			writeOK(req.ID, model.Snapshot())
		default:
			writeErr(req.ID, "unknown method "+req.Method)
		}
	}
	if err := sc.Err(); err != nil {
		writeErr(0, "stdin error: "+err.Error())
	}
}

func writeOK(id uint64, result interface{}) {
	raw, _ := json.Marshal(map[string]interface{}{"id": id, "result": result})
	fmt.Println(string(raw))
}

func writeErr(id uint64, msg string) {
	raw, _ := json.Marshal(map[string]interface{}{"id": id, "error": msg})
	fmt.Println(string(raw))
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func firstStringParam(params []json.RawMessage) (string, error) {
	if len(params) < 1 {
		return "", fmt.Errorf("params: [string]")
	}
	var s string
	if err := json.Unmarshal(params[0], &s); err != nil || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("params: [string]")
	}
	return s, nil
}

func hexMagic() string {
	return fmt.Sprintf("%x", bbpow.CommitmentMagic)
}
