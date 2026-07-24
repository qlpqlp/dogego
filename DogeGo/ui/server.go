// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dogego/analytics"
	"dogego/applog"
	"dogego/autostart"
	"dogego/chain"
	"dogego/config"
	"dogego/consensus"
	"dogego/docs"
	"dogego/extensions"
	"dogego/httptls"
	"dogego/mempool"
	"dogego/rpc"
	"dogego/store"
	"dogego/ui/websecurity"
	"dogego/version"
	"dogego/wallet"
)

// StartConfig configures the embedded web dashboard.
type StartConfig struct {
	ListenAddr   string
	ChainDisplay string
	Network      string
	// NodeMode is "full" or "spv" (for dashboard copy and /api/summary).
	NodeMode string
	// PeerLabel, if non-nil, is read on each /api/summary so the node can update it after connect.
	PeerLabel *string
	// RPCAddr is the configured JSON-RPC listen address (empty when disabled).
	RPCAddr string
	// RPCSnapshot returns live JSON-RPC listener state (listening port, dispatch wired).
	RPCSnapshot func() (listening, dispatchReady bool)
	Journal     *store.HeaderJournal
	// GenesisHash is the network genesis block id (avoids reading all of headers.bin on each poll).
	GenesisHash string
	// RawBlocks counts full block payloads stored on disk (Phase 2+); may be nil.
	RawBlocks *store.RawBlockStore
	// TxIndex maps confirmed txids to stored blocks; may be nil when -no_tx_index or SPV.
	TxIndex *store.TxIndex
	// AddrIndex maps hash160 to receive/spend history; built with tx index (reindextx).
	AddrIndex     *store.AddrIndex
	Pool          *mempool.Pool
	OpenBrowser   bool
	Wallet        *wallet.Disk
	MineRequested bool
	// MiningActive is set while the solo background miner is running (optional).
	MiningActive *atomic.Bool

	// ConfSavePath is where dogecoinconf.json is written from the Settings page (empty disables save).
	ConfSavePath string
	// EffectiveFile is the merged config shown in Settings (GET /api/config).
	EffectiveFile config.File
	// Stop requests graceful shutdown (same as interrupt).
	Stop func()
	// Restart spawns a replacement node after a short delay then stops this process.
	Restart func() error
	// ApplyUpdate launches a verified update binary then stops this process.
	ApplyUpdate func(newExePath string) error
	// BaseDataDir is the user -datadir (config); chain files live under ChainDataDir.
	BaseDataDir string
	// ChainDataDir is headers.bin / rawblocks for this network (e.g. datadir/mainnet).
	ChainDataDir string
	// ActivityLog optional ring buffer for the Debug log page (fed by node P2P / RPC).
	ActivityLog *applog.Ring
	// PubkeyHashAddrID is the P2PKH base58 version byte for this network (e.g. 0x1e main, 0x71 test).
	PubkeyHashAddrID byte
	// P2PSnapshot returns live P2P fields for GET /api/p2p (from the embedded node); nil disables the endpoint body wiring.
	P2PSnapshot func() map[string]any
	// DGRSnapshot returns DogeGo relay CGNAT metrics for GET /api/dgr.
	DGRSnapshot func() map[string]any
	// WalletSend is set after the node wires sendtoaddress (may be nil until sync/RPC ready).
	WalletSend *WalletSendBridge
	// WalletTxs lists wallet transactions when wired (same source as listtransactions).
	WalletTxs *WalletTxsBridge
	// EmbeddedAnalyticsSidecar true when the node runs the background Pebble analytics indexer (headers + optional raw bins).
	EmbeddedAnalyticsSidecar bool
	// AnalyticsRead optional shared-store reader (avoids Pebble double-open while sidecar runs).
	AnalyticsRead func() (*analytics.SideDetail, error)
	// Services optional runtime start/stop (analytics, mempool pause, etc.).
	Services ServiceController
	// StorageSummary returns native layout fields (headers, rawblocks, tx index).
	StorageSummary func() map[string]interface{}
	// ContiguousRawHeight returns cached contiguous raw-body height (-1 if unknown).
	// The node should wire this to BlockStoreCtx to avoid O(tip) scans on /api/summary.
	ContiguousRawHeight func() int64
	// ChainIBDSync returns Core-shaped IBD state (initialblockdownload latch, min chain work, maxtipage).
	ChainIBDSync func() rpc.ChainIBDSnapshot
	// HeaderSyncDiag returns getblockchaininfo-style header sync diagnostics (optional).
	HeaderSyncDiag func() map[string]interface{}
	// RecoverHeaderJournal runs operator header journal recovery (same as dogego_recoverheaders).
	RecoverHeaderJournal func() (tipBefore, tipAfter int64, rewound bool, err error)
	// UpdateChecker polls GitHub for newer releases (optional; nil disables update banner).
	UpdateChecker *version.UpdateChecker
	// RPCInvoke runs a JSON-RPC method in-process (web Console; loopback only).
	RPCInvoke func(method string, params []json.RawMessage) map[string]interface{}
	// Extensions optional manager for icon assets and panel routes.
	Extensions *extensions.Manager
	// ExtensionManager resolves the live manager (may be nil until node startup finishes).
	ExtensionManager func() *extensions.Manager
	// UtxoCache returns the live in-memory UTXO set when wired (may lag chain tip during sync).
	UtxoCache func() *store.UtxoCache
	// OrphanCount returns P2P orphan transaction pool size (pending parents during sync).
	OrphanCount func() int
	// TLS optional PEM cert/key for the dashboard listener (Core rpcssl analogue).
	TLS httptls.Pair
}

func analyticsSidecarLive(cfg StartConfig) bool {
	if cfg.Services != nil {
		return cfg.Services.AnalyticsRunning()
	}
	return cfg.EmbeddedAnalyticsSidecar
}

func walletAddr(w *wallet.Disk) string {
	if w == nil {
		return ""
	}
	return w.Address()
}

func isLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	// Trusted dashboard client (not literal loopback): DogeBox and similar reverse
	// proxies connect from a private container/host address. Opt in with
	// DOGEGO_TRUST_PRIVATE_CLIENTS=1. Uses RemoteAddr only (ignores X-Forwarded-For).
	// When enabled, ALL loopback-gated UI routes (wallet backup/send/unlock, config
	// writes, updates, Console RPC, …) treat RFC1918 / link-local clients as local.
	// Safe only when the web UI is not reachable by untrusted LAN/WAN hosts
	// (bind to the pup IP; front with Dogebox auth; do not publish :2013 to the world).
	if trustPrivateDashboardClients() && (ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return true
	}
	return false
}

func trustPrivateDashboardClients() bool {
	switch strings.TrimSpace(strings.ToLower(os.Getenv("DOGEGO_TRUST_PRIVATE_CLIENTS"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Start runs the HTTP server until ctx is cancelled. Returns the base URL (e.g. http://localhost:2013/).
func Start(ctx context.Context, cfg StartConfig) (baseURL string, err error) {
	if cfg.ListenAddr == "" || cfg.Journal == nil {
		return "", nil
	}
	indexHTML, err := fs.ReadFile(static, "static/index.html")
	if err != nil {
		return "", err
	}
	analyticsHTML, err := fs.ReadFile(static, "static/analytics.html")
	if err != nil {
		return "", err
	}
	ln, scheme, err := httptls.Listen(cfg.ListenAddr, cfg.TLS)
	if err != nil {
		return "", err
	}
	baseURL = publicDashboardURL(scheme, cfg.ListenAddr, ln)
	if cfg.ActivityLog != nil {
		cfg.ActivityLog.Add("ui", "Web dashboard listening at "+baseURL)
	}
	if trustPrivateDashboardClients() {
		msg := "DOGEGO_TRUST_PRIVATE_CLIENTS: private/link-local clients are treated as local for dashboard APIs (DogeBox); keep webui bound to the pup IP and behind the host gateway"
		fmt.Fprintln(os.Stderr, "DogeGo:", msg)
		if cfg.ActivityLog != nil {
			cfg.ActivityLog.Add("ui", msg)
		}
	}

	mux := http.NewServeMux()
	addBrandingRoutes(mux)
	var webGate *websecurity.Gate
	if cfg.ChainDataDir != "" {
		if g, err := websecurity.NewGate(cfg.ChainDataDir); err == nil {
			webGate = g
		}
	}
	remoteAuthActive := remoteDashboardAuthRequired(cfg.EffectiveFile, cfg.ListenAddr)
	registerSecurityRoutes(mux, webGate, securityRouteOpts{
		AllowRemoteUnlock: remoteAuthActive,
		RemoteAuthActive:  remoteAuthActive,
		SecureCookies:     cfg.TLS.Enabled(),
	})
	registerTLSRoutes(mux, cfg)
	registerFirewallRoutes(mux)
	readAuth := func(w http.ResponseWriter, r *http.Request) bool {
		return requireDashboardRead(w, r, webGate, cfg.EffectiveFile, cfg.ListenAddr)
	}
	live := StartLiveFeed(ctx, cfg, 750*time.Millisecond)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	mux.HandleFunc("/analytics.html", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(analyticsHTML)
	})
	mux.HandleFunc("/api/analytics/summary", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !readAuth(w, r) {
			return
		}
		dbPath := filepath.Join(cfg.ChainDataDir, "dogego_analytics.db")
		var detail *analytics.SideDetail
		var err error
		if cfg.AnalyticsRead != nil {
			detail, err = cfg.AnalyticsRead()
		} else {
			detail, err = analytics.ReadSideDetail(dbPath)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		exists := detail.Exists
		var size int64
		schema := 0
		var rawBin *int
		if exists {
			size = detail.Size
			schema = detail.Schema
			rawBin = detail.RawBinCount
		}
		tip, hdrCount, _ := journalTipForDashboard(cfg.Journal)
		chainActive := rpc.ActiveChainBlockHeight(cfg.Journal, cfg.RawBlocks)
		rbLive := 0
		if cfg.RawBlocks != nil {
			if n, err := cfg.RawBlocks.Count(); err == nil {
				rbLive = n
			}
		}
		out := map[string]interface{}{
			"chain_data_dir":       cfg.ChainDataDir,
			"analytics_db_path":    dbPath,
			"analytics_db_exists":  exists,
			"analytics_db_bytes":   size,
			"schema_version":       schema,
			"headers_tip_height":   tip,
			"chain_active_height":  chainActive,
			"headers_count":        hdrCount,
			"rawblocks_live_count": rbLive,
		}
		if storedBodies := contiguousHeightForAPI(cfg); storedBodies >= 0 {
			out["stored_bodies_height"] = storedBodies
			if rbLive <= 0 {
				out["rawblocks_live_count"] = int(storedBodies) + 1
			}
		}
		if rawBin != nil {
			out["rawblocks_analytics_count"] = *rawBin
		}
		if exists && len(detail.Meta) > 0 {
			out["meta"] = detail.Meta
		}
		if exists && len(detail.IndexProgress) > 0 {
			out["index_progress"] = detail.IndexProgress
		}
		if len(detail.MetricTimeline) > 0 {
			out["metric_timeline"] = detail.MetricTimeline
		}
		if len(detail.ReorgEvents) > 0 {
			out["reorg_events"] = detail.ReorgEvents
			out["reorg_summary"] = detail.ReorgSummary
		} else if detail.Exists {
			out["reorg_events"] = []analytics.ReorgEvent{}
			out["reorg_summary"] = detail.ReorgSummary
		}
		out["max_block_weight"] = consensus.MaxBlockWeight
		if cfg.Journal != nil {
			ca, sb := chainStatsHints(cfg)
			light := r.URL.Query().Get("light") == "1"
			if !light && cfg.ChainIBDSync != nil {
				if snap := cfg.ChainIBDSync(); snap.IBD {
					light = true
				}
			}
			out["chainstats"] = BuildChainStats(cfg.Journal, cfg.RawBlocks, cfg.PubkeyHashAddrID, time.Now(), ca, sb, light)
		}
		if cfg.UtxoCache != nil {
			if u := cfg.UtxoCache(); u != nil {
				pub, sh, _ := chainVersions(cfg.Network)
				out["top_utxo_holders"] = BuildTopUtxoHolders(u, cfg.Journal, cfg.AddrIndex, pub, sh, topUtxoHolderLimit)
			}
		}
		if cfg.StorageSummary != nil {
			if st := cfg.StorageSummary(); st != nil {
				out["storage"] = st
			}
		}
		if cfg.Network != "" {
			out["network"] = cfg.Network
		}
		live.RememberAnalytics(out)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/api/forks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !readAuth(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(BuildForksStatus(cfg))
	})
	mux.HandleFunc("/api/analytics/metrics.csv", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !readAuth(w, r) {
			return
		}
		dbPath := filepath.Join(cfg.ChainDataDir, "dogego_analytics.db")
		var detail *analytics.SideDetail
		var err error
		if cfg.AnalyticsRead != nil {
			detail, err = cfg.AnalyticsRead()
		} else {
			detail, err = analytics.ReadSideDetail(dbPath)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="dogego-metrics.csv"`)
		_, _ = w.Write(analytics.MetricSamplesCSV(detail.MetricTimeline))
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/live", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !readAuth(w, r) {
			return
		}
		live.writeLive(w)
	})

	mux.HandleFunc("/api/summary", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !readAuth(w, r) {
			return
		}
		live.writeSummary(w)
	})

	mux.HandleFunc("/api/p2p", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !readAuth(w, r) {
			return
		}
		live.writeP2P(w)
	})

	mux.HandleFunc("/api/lan-peer-hint", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !readAuth(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(BuildLanPeerHint(cfg.Network))
	})

	mux.HandleFunc("/api/dgr", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !readAuth(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if cfg.DGRSnapshot != nil {
			_ = json.NewEncoder(w).Encode(cfg.DGRSnapshot())
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": false})
	})

	registerPeersAPI(mux, cfg, readAuth)

	mux.HandleFunc("/api/core-cert", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(DefaultCoreCertManifest())
	})

	mux.HandleFunc("/api/core-compare", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		probeConf := probeConfigFromStart(cfg)
		_ = json.NewEncoder(w).Encode(ProbeCoreCompare(cfg.Network, cfg.RPCAddr, probeConf, cfg.RPCInvoke))
	})

	mux.HandleFunc("/api/core-maintenance", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		probeConf := probeConfigFromStart(cfg)
		_ = json.NewEncoder(w).Encode(ProbeCoreMaintenance(cfg.Network, probeConf, cfg.RPCInvoke))
	})

	mux.HandleFunc("/api/core-probes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		probeConf := probeConfigFromStart(cfg)
		refresh := r.URL.Query().Get("refresh") == "1"
		if cert, ok := coreProbeCache.peek(); ok && !refresh && cert.Probes.CheckedAt != "" {
			_ = json.NewEncoder(w).Encode(cert.Probes)
			return
		}
		cert := coreProbeCache.operatorCert(cfg.Network, cfg.RPCAddr, cfg.ChainDataDir, probeConf, cfg.RPCInvoke, true)
		_ = json.NewEncoder(w).Encode(cert.Probes)
	})

	mux.HandleFunc("/api/core-operator-cert", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		probeConf := probeConfigFromStart(cfg)
		refresh := r.URL.Query().Get("refresh") == "1"
		if r.URL.Query().Get("matrix") == "1" {
			var cached *CoreOperatorCertResult
			if c, ok := coreProbeCache.peek(); ok {
				cp := c
				cached = &cp
			}
			_ = json.NewEncoder(w).Encode(CoreOperatorCertMatrix(cached))
			return
		}
		_ = json.NewEncoder(w).Encode(coreProbeCache.operatorCert(cfg.Network, cfg.RPCAddr, cfg.ChainDataDir, probeConf, cfg.RPCInvoke, refresh))
	})

	mux.HandleFunc("/api/core-end-to-end-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		probeConf := probeConfigFromStart(cfg)
		refresh := r.URL.Query().Get("refresh") == "1"
		if cert, ok := coreProbeCache.peek(); ok && !refresh && cert.Probes.EndToEnd.CheckedAt != "" {
			_ = json.NewEncoder(w).Encode(cert.Probes.EndToEnd)
			return
		}
		cert := coreProbeCache.operatorCert(cfg.Network, cfg.RPCAddr, cfg.ChainDataDir, probeConf, cfg.RPCInvoke, true)
		_ = json.NewEncoder(w).Encode(cert.Probes.EndToEnd)
	})

	mux.HandleFunc("/api/core-status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		probeConf := probeConfigFromStart(cfg)
		_ = json.NewEncoder(w).Encode(BuildCoreStatus(cfg.Network, probeConf))
	})

	mux.HandleFunc("/api/core-test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		probeConf := probeConfigFromStart(cfg)
		var body struct {
			CoreRPCAddr     string `json:"core_rpc_addr"`
			CoreRPCUser     string `json:"core_rpc_user"`
			CoreRPCPassword string `json:"core_rpc_password"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		probeConf = ApplyCoreRPCFormOverride(probeConf, body.CoreRPCAddr, body.CoreRPCUser, body.CoreRPCPassword)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeCoreTest(cfg.Network, probeConf))
	})

	mux.HandleFunc("/api/signer-test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		probeConf := probeConfigFromStart(cfg)
		var body struct {
			SignerCmd string `json:"signer_cmd"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeSignerTest(probeConf, body.SignerCmd))
	})

	mux.HandleFunc("/api/core-reindex-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		probeConf := probeConfigFromStart(cfg)
		_ = json.NewEncoder(w).Encode(ProbeCoreReindex(cfg.Network, probeConf, cfg.RPCInvoke))
	})

	mux.HandleFunc("/api/core-bip152-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		probeConf := probeConfigFromStart(cfg)
		_ = json.NewEncoder(w).Encode(ProbeCoreBip152(cfg.Network, probeConf, cfg.RPCInvoke))
	})

	mux.HandleFunc("/api/core-mining-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		probeConf := probeConfigFromStart(cfg)
		_ = json.NewEncoder(w).Encode(ProbeCoreMining(cfg.Network, probeConf, cfg.RPCInvoke))
	})

	mux.HandleFunc("/api/core-wallet-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeCoreWallet(cfg.RPCInvoke))
	})

	mux.HandleFunc("/api/core-pq-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeCorePQ())
	})

	mux.HandleFunc("/api/core-addrman-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeCoreAddrman(cfg.RPCInvoke))
	})

	mux.HandleFunc("/api/core-ibd-convergence-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeCoreIbdConvergence(cfg.Network, cfg.ChainDataDir, cfg.RPCAddr, cfg.EffectiveFile))
	})

	mux.HandleFunc("/api/core-restart-resume", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeCoreRestartResume(cfg.Network, cfg.ChainDataDir, cfg.EffectiveFile, cfg.RPCInvoke))
	})

	mux.HandleFunc("/api/core-field-evidence-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeCoreFieldEvidence(cfg.Network))
	})

	mux.HandleFunc("/api/core-autostart-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeAutostartLogin(cfg.EffectiveFile))
	})

	mux.HandleFunc("/api/core-founder-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeFounder(cfg.Network, cfg.EffectiveFile))
	})

	mux.HandleFunc("/api/core-operational-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeOperational(cfg.Network, cfg.ChainDataDir, cfg.EffectiveFile, cfg.BaseDataDir))
	})

	mux.HandleFunc("/api/core-runner-probes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		requireCore := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("require_core")), "1") ||
			strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("require_core")), "true")
		requireWalletDat := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("require_wallet_dat")), "1") ||
			strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("require_wallet_dat")), "true")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeRunner(RunnerProbeOptions{RequireCore: requireCore, RequireWalletDat: requireWalletDat}))
	})

	mux.HandleFunc("/api/core-workflow10-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		requireWalletDat := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("require_wallet_dat")), "1") ||
			strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("require_wallet_dat")), "true")
		skipProvision := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("skip_provision")), "1") ||
			strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("skip_provision")), "true")
		mineBootstrap := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("mine_bootstrap")), "1") ||
			strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("mine_bootstrap")), "true")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeWorkflow10ForNetwork(cfg.Network, Workflow10ProbeOptions{
			RequireWalletDat: requireWalletDat,
			SkipProvision:    skipProvision,
			MineBootstrap:    mineBootstrap,
		}))
	})

	mux.HandleFunc("/api/core-setup-parity", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(ProbeSetupParity(cfg.Network))
	})

	mux.HandleFunc("/api/mempool/parity-probe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("corpus")), "full") {
			_ = json.NewEncoder(w).Encode(RunMempoolParityFullCorpusProbe())
			return
		}
		if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("corpus")), "stateful") {
			_ = json.NewEncoder(w).Encode(RunMempoolParityStatefulCorpusProbe())
			return
		}
		probeConf := probeConfigFromStart(cfg)
		_ = json.NewEncoder(w).Encode(RunMempoolParityProbe(cfg.Network, probeConf, cfg.RPCInvoke))
	})

	mux.HandleFunc("/api/mempool/stateful-status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		probeConf := probeConfigFromStart(cfg)
		_ = json.NewEncoder(w).Encode(RunMempoolStatefulStatusProbe(cfg.Network, probeConf))
	})

	mux.HandleFunc("/api/rpc", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if cfg.RPCInvoke == nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "1.0", "id": 1,
				"error": map[string]interface{}{"code": -1, "message": "RPC not available"},
			})
			return
		}
		var body struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			ID     json.RawMessage `json:"id"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		method := strings.TrimSpace(body.Method)
		if method == "" {
			http.Error(w, "method required", http.StatusBadRequest)
			return
		}
		var params []json.RawMessage
		if len(body.Params) > 0 && string(bytes.TrimSpace(body.Params)) != "null" {
			if err := json.Unmarshal(body.Params, &params); err != nil {
				http.Error(w, "params must be a JSON array", http.StatusBadRequest)
				return
			}
		}
		out := cfg.RPCInvoke(method, params)
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/api/chain/recover-headers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if cfg.RPCInvoke != nil {
			out := cfg.RPCInvoke("dogego_recoverheaders", nil)
			if errObj, ok := out["error"].(map[string]interface{}); ok && errObj != nil {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": errObj["message"]})
				return
			}
			if res, ok := out["result"].(map[string]interface{}); ok {
				if cfg.ActivityLog != nil {
					if tb, ok := res["tip_before"].(float64); ok {
						if ta, ok2 := res["tip_after"].(float64); ok2 {
							cfg.ActivityLog.Add("headers", "web UI: recovered header journal "+strconv.FormatInt(int64(tb), 10)+" → "+strconv.FormatInt(int64(ta), 10))
						}
					}
				}
				_ = json.NewEncoder(w).Encode(res)
				return
			}
		}
		if cfg.RecoverHeaderJournal == nil {
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "header recovery not available"})
			return
		}
		tipBefore, tipAfter, rewound, err := cfg.RecoverHeaderJournal()
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(), "tip_before": tipBefore, "tip_after": tipAfter, "rewound": rewound,
			})
			return
		}
		if cfg.ActivityLog != nil {
			cfg.ActivityLog.Add("headers", "web UI: recovered header journal "+strconv.FormatInt(tipBefore, 10)+" → "+strconv.FormatInt(tipAfter, 10))
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"rewound":    rewound,
			"tip_before": tipBefore,
			"tip_after":  tipAfter,
			"message":    "header journal recovered; header sync will continue automatically",
		})
	})
	mux.HandleFunc("/api/chainstats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		light := r.URL.Query().Get("light") == "1"
		interval := 12 * time.Second
		if !light {
			interval = 30 * time.Second
		}
		if b := live.ChainStatsCached(cfg, light, interval); len(b) > 0 {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		ca, sb := chainStatsHints(cfg)
		st := BuildChainStats(cfg.Journal, cfg.RawBlocks, cfg.PubkeyHashAddrID, time.Now(), ca, sb, light)
		_ = json.NewEncoder(w).Encode(st)
	})

	mux.HandleFunc("/api/guide", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(DefaultGuideManifest())
	})

	mux.HandleFunc("/api/docs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(DefaultDocsManifest())
	})
	mux.HandleFunc("/api/docs/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"files": EmbeddedMarkdownFiles()})
	})
	mux.HandleFunc("/api/docs/resolve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		base := strings.TrimSpace(r.URL.Query().Get("base"))
		href := strings.TrimSpace(r.URL.Query().Get("href"))
		if href == "" {
			http.Error(w, "href required", http.StatusBadRequest)
			return
		}
		fetchPath, anchor, external, err := docs.ResolveMarkdownLink(base, href)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"path":     fetchPath,
			"anchor":   anchor,
			"external": external,
			"href":     href,
		})
	})
	mux.HandleFunc("/api/docs/md", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rel := strings.TrimSpace(r.URL.Query().Get("path"))
		if rel == "" {
			http.Error(w, "path required", http.StatusBadRequest)
			return
		}
		base := strings.TrimSpace(r.URL.Query().Get("base"))
		if base != "" {
			resolved, _, external, err := docs.ResolveMarkdownLink(base, rel)
			if err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			if external {
				http.Error(w, "external link", http.StatusBadRequest)
				return
			}
			rel = resolved
		}
		b, name, err := ReadEmbeddedMarkdown(rel)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			hint := "Pick another file from the list below, or open this path from a full DogeGo source checkout."
			if strings.EqualFold(rel, "ROADMAP.md") {
				hint = "ROADMAP.md ships beside the DogeGo module (go.mod). Run dogego from a source tree, or browse ROADMAP on GitHub."
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
				"path":  rel,
				"hint":  hint,
			})
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"path":     name,
			"markdown": string(b),
		})
	})

	mux.HandleFunc("/api/openrpc.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(rpc.BuildOpenRPCDocument())
	})

	mux.HandleFunc("/api/rpc/cookbook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"count":   len(rpc.SupportedMethods()),
			"entries": rpc.BuildRPCCookbook(),
		})
	})

	mux.HandleFunc("/api/rpc/reference.html", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(rpc.BuildRPCReferenceHTML()))
	})

	mux.HandleFunc("/api/integration/guides", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"guides": rpc.BuildIntegrationGuides()})
	})

	mux.HandleFunc("/api/capabilities", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		cap := DefaultCapabilitiesManifest()
		live := map[string]any{
			"rpc_enabled":                cfg.RPCAddr != "",
			"rpc_addr":                   cfg.RPCAddr,
			"wallet_enabled":             walletLoaded(cfg),
			"wallet_rpc_ready":           walletRPCReady(cfg),
			"wallet_address_ready":       walletAddressReady(cfg),
			"wallet_address":             walletAddr(cfg.Wallet),
			"tx_index":                   cfg.TxIndex != nil,
			"raw_blocks":                 cfg.RawBlocks != nil,
			"embedded_analytics_sidecar": analyticsSidecarLive(cfg),
			"node_mode":                  cfg.NodeMode,
			"network":                    cfg.Network,
		}
		if cfg.MiningActive != nil {
			live["mining_active"] = cfg.MiningActive.Load()
		}
		p2pCfg := strings.TrimSpace(cfg.EffectiveFile.P2PConnectivity)
		if p2pCfg == "" {
			p2pCfg = "both"
		}
		probeConf := probeConfigFromStart(cfg)
		EnrichLiveCoreStatus(live, cfg.Network, probeConf)
		if cfg.HeaderSyncDiag != nil {
			if diag := cfg.HeaderSyncDiag(); diag != nil {
				rpc.CopyUtxoReplaySummary(live, diag)
			}
		}
		live["p2p_connectivity"] = p2pCfg
		live["maxoutbound"] = cfg.EffectiveFile.MaxOutbound
		live["maxinbound"] = cfg.EffectiveFile.MaxInbound
		live["firewall"] = strings.TrimSpace(cfg.EffectiveFile.Firewall)
		if live["firewall"] == "" {
			live["firewall"] = "auto"
		}
		live["upnp"] = strings.TrimSpace(cfg.EffectiveFile.Upnp)
		if live["upnp"] == "" {
			live["upnp"] = "auto"
		}
		EnrichRPCSummaryFields(live, cfg.RPCAddr, cfg.RPCSnapshot)
		zmqOn := strings.TrimSpace(cfg.EffectiveFile.ZmqPubHashBlock) != "" ||
			strings.TrimSpace(cfg.EffectiveFile.ZmqPubHashTx) != "" ||
			strings.TrimSpace(cfg.EffectiveFile.ZmqPubRawBlock) != "" ||
			strings.TrimSpace(cfg.EffectiveFile.ZmqPubRawTx) != ""
		live["zmq_enabled"] = zmqOn
		if cfg.P2PSnapshot != nil {
			if snap := p2PSnapshotWithTimeout(cfg.P2PSnapshot); snap != nil {
				for _, k := range []string{
					"health", "health_message", "connections_total", "connections_inbound",
					"connections_outbound", "listen_enabled", "multi_peer_enabled", "cgnat_mode",
					"upnp_mapped", "upnp_external", "upnp_method",
				} {
					if v, ok := snap[k]; ok {
						live[k] = v
					}
				}
			}
		}
		rp := RelayPolicyForAPI(cfg.EffectiveFile, cfg.Pool)
		live["relay_min_doge"] = rp["effective_minrelay_doge"]
		if pkg, ok := rp["package_policy"].(map[string]any); ok {
			live["package_ancestors"] = pkg["limitancestorcount"]
		}
		if std, ok := rp["standard_policy"].(map[string]any); ok {
			live["acceptdatacarrier"] = std["acceptdatacarrier"]
		}
		if cfg.DGRSnapshot != nil {
			if dgr := cfg.DGRSnapshot(); dgr != nil {
				if v, ok := dgr["enabled"]; ok {
					live["dgr_enabled"] = v
				}
				if v, ok := dgr["inbound_relay"]; ok {
					live["dgr_inbound"] = v
				}
				if v, ok := dgr["outbound_relay"]; ok {
					live["dgr_outbound"] = v
				}
			}
		} else if cfg.EffectiveFile.DogeGoRelayCGNAT.Enabled {
			live["dgr_enabled"] = true
		}
		EnrichCapabilitiesLive(&cap, live)
		EnrichParitySummaryFromProbeCache(&cap.ParitySummary)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(cap)
	})

	mux.HandleFunc("/api/explorer/header", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		m, code, msg := LookupHeaderForAPI(cfg.Journal, q.Get("height"), q.Get("hash"))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if code != 0 {
			status := http.StatusInternalServerError
			switch code {
			case 400:
				status = http.StatusBadRequest
			case 404:
				status = http.StatusNotFound
			case 503:
				status = http.StatusServiceUnavailable
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
			return
		}
		_ = json.NewEncoder(w).Encode(m)
	})

	mux.HandleFunc("/api/explorer/block", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		m, code, msg := LookupBlockForAPI(cfg.Journal, cfg.RawBlocks, cfg.PubkeyHashAddrID, q.Get("height"), q.Get("hash"), contiguousHeightForAPI(cfg))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if code != 0 {
			status := http.StatusInternalServerError
			switch code {
			case 400:
				status = http.StatusBadRequest
			case 404:
				status = http.StatusNotFound
			case 503:
				status = http.StatusServiceUnavailable
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
			return
		}
		_ = json.NewEncoder(w).Encode(m)
	})

	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if cfg.ActivityLog == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"lines": []any{}})
			return
		}
		limit := 800
		if s := r.URL.Query().Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 5000 {
				limit = n
			}
		}
		lines := cfg.ActivityLog.SnapshotTail(limit)
		_ = json.NewEncoder(w).Encode(map[string]any{"lines": lines})
	})

	mux.HandleFunc("/api/wallet", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if !requireWalletRead(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(walletAPIEnvelope(cfg))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/wallet/flags", func(w http.ResponseWriter, r *http.Request) {
		if webGate != nil && !webGate.RequireUnlocked(w, r) {
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if cfg.Wallet == nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "wallet disabled"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"avoid_reuse":            cfg.Wallet.AvoidReuse(),
				"pq_commitments_enabled": cfg.Wallet.PqCommitmentsEnabled(),
				"pq_carrier_enabled":     cfg.Wallet.PqCarrierEnabled(),
			})
		case http.MethodPost:
			var body struct {
				AvoidReuse           *bool `json:"avoid_reuse"`
				PqCommitmentsEnabled *bool `json:"pq_commitments_enabled"`
				PqCarrierEnabled     *bool `json:"pq_carrier_enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
			if body.AvoidReuse != nil {
				if err := cfg.Wallet.SetAvoidReuse(*body.AvoidReuse); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			if body.PqCommitmentsEnabled != nil {
				if err := cfg.Wallet.SetPqCommitmentsEnabled(*body.PqCommitmentsEnabled); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			if body.PqCarrierEnabled != nil {
				if err := cfg.Wallet.SetPqCarrierEnabled(*body.PqCarrierEnabled); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	registerWalletTxsRoutes(mux, cfg, webGate)
	registerExtensionsRoutes(mux, cfg, webGate)
	registerDIPsRoutes(mux)
	mux.HandleFunc("/api/explorer/tx", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		txid := strings.TrimSpace(r.URL.Query().Get("txid"))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if txid == "" {
			http.Error(w, "missing txid query parameter", http.StatusBadRequest)
			return
		}
		canChain := cfg.TxIndex != nil && cfg.RawBlocks != nil
		canPool := cfg.Pool != nil
		if !canChain && !canPool {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "tx lookup needs chain index + raw blocks and/or mempool (this run has neither)"})
			return
		}
		jm, rawTx, src, err := rpc.LookupTxExplorer(cfg.TxIndex, cfg.RawBlocks, cfg.Pool, txid)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		hx := ""
		if len(rawTx) > 0 {
			hx = hex.EncodeToString(rawTx)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tx": jm, "hex": hx, "source": src})
	})
	mux.HandleFunc("/api/explorer/decode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h := strings.TrimSpace(r.URL.Query().Get("hex"))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if h == "" {
			http.Error(w, "missing hex query parameter", http.StatusBadRequest)
			return
		}
		jm, err := rpc.DecodeTxHex(h, cfg.Network)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tx": jm})
	})
	mux.HandleFunc("/api/explorer/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		m, code, msg := ExplorerUniversalSearch(q, cfg.Network, cfg.Journal, cfg.RawBlocks, cfg.TxIndex, cfg.AddrIndex, cfg.Pool, cfg.PubkeyHashAddrID, cfg.RPCInvoke, cfg.UtxoCache, contiguousHeightForAPI(cfg))
		if code != 0 {
			status := http.StatusBadRequest
			switch code {
			case 404:
				status = http.StatusNotFound
			case 503:
				status = http.StatusServiceUnavailable
			}
			w.WriteHeader(status)
			if m == nil {
				_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
				return
			}
			_ = json.NewEncoder(w).Encode(m)
			return
		}
		_ = json.NewEncoder(w).Encode(m)
	})
	registerBlockStepRoutes(mux, cfg)
	mux.HandleFunc("/api/mempool", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !readAuth(w, r) {
			return
		}
		if strings.TrimSpace(r.URL.Query().Get("limit")) != "" && r.URL.Query().Get("limit") != "48" {
			lim := 120
			if s := strings.TrimSpace(r.URL.Query().Get("limit")); s != "" {
				if n, err := strconv.Atoi(s); err == nil && n > 0 {
					lim = n
				}
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(MempoolDetailForAPI(cfg.Pool, lim, cfg.EffectiveFile, cfg.OrphanCount))
			return
		}
		live.writeMempool(w)
	})
	registerWalletUnlockRoutes(mux, cfg, webGate)
	registerWalletEncryptRoutes(mux, cfg, webGate)
	registerWalletSendRoute(mux, cfg, webGate)
	registerWalletTxFlightRoutes(mux, cfg, webGate)
	registerWalletBackupRoutes(mux, cfg, webGate)
	registerWalletImportRoutes(mux, cfg, webGate)
	registerWalletKeypoolRoute(mux, cfg, webGate)
	registerWalletUtxosRoute(mux, cfg, webGate)
	registerWalletRescanRoute(mux, cfg, webGate)

	// /api/mining: read/update the "mine" flag in dogecoinconf.json (loopback only).
	mux.HandleFunc("/api/mining", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch r.Method {
		case http.MethodGet:
			inConf := false
			if cfg.ConfSavePath != "" {
				if b, err := os.ReadFile(cfg.ConfSavePath); err == nil {
					var disk config.File
					if json.Unmarshal(b, &disk) == nil {
						inConf = disk.Mine
					}
				}
			}
			miningActive := cfg.MiningActive != nil && cfg.MiningActive.Load()
			net := strings.ToLower(strings.TrimSpace(cfg.Network))
			disclaimer := "Reboot testnet: mining runs automatically to your wallet address (scrypt PoW). Disable with -mine=false or Settings → mining off."
			if net == "mainnet" || net == "main" {
				disclaimer = "Mainnet: CPU mining via generatetoaddress or pool merge-mining via createauxblock/submitauxblock (RPC). Background mine applies to reboot testnet only."
			}
			if miningActive {
				disclaimer = "Background miner is active (reboot testnet; coinbase pays the wallet address)."
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"mine_this_run":    cfg.MineRequested,
				"mine_in_config":   inConf,
				"mining_active":    miningActive,
				"config_path":      cfg.ConfSavePath,
				"disclaimer_short": disclaimer,
			})
		case http.MethodPost:
			var body struct {
				Mine bool `json:"mine"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid JSON (expected {\"mine\": true|false})", http.StatusBadRequest)
				return
			}
			if cfg.ConfSavePath == "" {
				http.Error(w, "no config save path", http.StatusInternalServerError)
				return
			}
			f := cfg.EffectiveFile
			if b, err := os.ReadFile(cfg.ConfSavePath); err == nil {
				var disk config.File
				if json.Unmarshal(b, &disk) == nil && strings.TrimSpace(disk.DataDir) != "" {
					f = disk
				}
			}
			f.Mine = body.Mine
			if err := config.ValidateAndNormalize(&f); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := config.Save(cfg.ConfSavePath, f); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":               true,
				"mine":             f.Mine,
				"saved_to":         cfg.ConfSavePath,
				"restart_required": true,
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	registerUACommentPreview(mux)

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			out := cfg.EffectiveFile
			if cfg.ConfSavePath != "" {
				if b, err := os.ReadFile(cfg.ConfSavePath); err == nil {
					var disk config.File
					if json.Unmarshal(b, &disk) == nil && strings.TrimSpace(disk.DataDir) != "" {
						out = disk
					}
				}
			}
			if nm := strings.TrimSpace(cfg.NodeMode); nm != "" {
				out.NodeMode = nm
			}
			fullNode := strings.ToLower(strings.TrimSpace(cfg.NodeMode)) != "spv"
			runtime := map[string]any{
				"node_mode":                  strings.TrimSpace(cfg.NodeMode),
				"full_node":                  fullNode,
				"embedded_analytics_sidecar": analyticsSidecarLive(cfg),
				"mine_requested":             cfg.MineRequested,
				"tx_index_enabled":           cfg.TxIndex != nil,
				"raw_blocks_enabled":         cfg.RawBlocks != nil,
				"block_storage_layout":       out.BlockStorageLayout,
				"block_zstd":                 out.BlockZstd,
				"tx_index_embed_tx":          out.EffectiveTxIndexEmbedTx(),
				"p2p_subversion":             chain.BuildSubVersion(out.EffectiveUAComment()),
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"path":    cfg.ConfSavePath,
				"config":  out,
				"runtime": runtime,
			})
		case http.MethodPost:
			var f config.File
			if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
			if err := config.ValidateAndNormalize(&f); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			existing := cfg.EffectiveFile
			if cfg.ConfSavePath != "" {
				if b, err := os.ReadFile(cfg.ConfSavePath); err == nil {
					var disk config.File
					if json.Unmarshal(b, &disk) == nil {
						existing = disk
					}
				}
			}
			tipWarn, err := resolveUACommentTipForConfig(&f, existing)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if config.IsRebootTestnetNetwork(f.Network) && !f.NoWallet {
				f.Mine = true
			}
			if cfg.ConfSavePath == "" {
				http.Error(w, "no config save path", http.StatusInternalServerError)
				return
			}
			if err := config.Save(cfg.ConfSavePath, f); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			autostartWarn := applyAutostart(f, cfg.ConfSavePath)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			out := map[string]any{"ok": true, "saved_to": cfg.ConfSavePath, "restart_required": true}
			if tipWarn != "" {
				out["uacomment_warning"] = tipWarn
			}
			if autostartWarn != "" {
				out["autostart_warning"] = autostartWarn
			}
			_ = json.NewEncoder(w).Encode(out)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/autostart", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		st := autostart.Current()
		want := false
		if cfg.ConfSavePath != "" {
			if b, err := os.ReadFile(cfg.ConfSavePath); err == nil {
				var disk config.File
				if json.Unmarshal(b, &disk) == nil {
					want = disk.AutostartOnLogin()
				}
			}
		}
		if !want {
			want = cfg.EffectiveFile.AutostartOnLogin()
		}
		vr := autostart.VerifyLogin(want)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":     st,
			"configured": want,
			"ok":         vr.OK,
			"verify":     vr,
		})
	})

	mux.HandleFunc("/api/services", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if cfg.Services == nil {
			http.Error(w, "service control not available", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]any{"services": cfg.Services.ServiceRows()})
		case http.MethodPost:
			var body struct {
				Service string `json:"service"`
				Action  string `json:"action"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
			if err := cfg.Services.ApplyServiceAction(strings.TrimSpace(body.Service), strings.TrimSpace(body.Action)); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"services": cfg.Services.ServiceRows(),
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/control/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if cfg.Stop == nil {
			http.Error(w, "shutdown not available", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		go func() {
			time.Sleep(80 * time.Millisecond)
			cfg.Stop()
		}()
	})

	mux.HandleFunc("/api/control/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if cfg.Restart == nil {
			http.Error(w, "restart not available", http.StatusServiceUnavailable)
			return
		}
		if err := cfg.Restart(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "note": "replacement process starts after a short delay"})
	})

	registerUpdateRoutes(mux, cfg)

	srv := &http.Server{
		Handler:           userAgentMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(ln)
	}()

	if cfg.OpenBrowser {
		time.AfterFunc(400*time.Millisecond, func() { OpenURLLog(baseURL) })
	}

	if cfg.RPCInvoke != nil {
		probeConf := probeConfigFromStart(cfg)
		go func() {
			time.Sleep(8 * time.Second)
			WarmCoreProbeCache(cfg.Network, cfg.RPCAddr, cfg.ChainDataDir, probeConf, cfg.RPCInvoke)
		}()
	}

	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
		wg.Wait()
	}()

	return baseURL, nil
}
