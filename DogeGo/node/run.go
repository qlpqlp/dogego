// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dogego/analytics"
	"dogego/applog"
	"dogego/chain"
	"dogego/config"
	"dogego/consensus"
	"dogego/extensions"
	"dogego/httptls"
	"dogego/mempool"
	"dogego/node/dgr"
	"dogego/p2p"
	"dogego/pow"
	"dogego/rpc"
	"dogego/signer"
	"dogego/store"
	"dogego/ui"
	"dogego/version"
	"dogego/wallet"
	"dogego/wire"
	"dogego/zmqnotify"
)

func isNetTimeout(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return errors.Is(err, os.ErrDeadlineExceeded)
}

// Config controls the experimental node process.
type Config struct {
	Network chain.Network
	DataDir string
	// Peer is host:port to dial. If empty, discovers peers via Core DNS hostnames then pnSeed6_* fixed seeds.
	Peer    string
	RPCAddr string // e.g. 127.0.0.1:18556 - empty disables RPC
	// UAComment is optional metadata appended like Core's -uacomment; full sub-version is always /DogeGo:1.14.9(...)/.
	UAComment string

	// WebUIAddr is the dashboard listen address (e.g. 127.0.0.1:2013). Empty disables the web UI.
	WebUIAddr string
	// WebUINoBrowser skips opening the default system browser when the web UI starts.
	WebUINoBrowser bool

	// EnableWallet loads wallet.json under the per-network chain directory.
	EnableWallet bool
	// Mine enables background block generation on reboot testnet when no peers are reachable (solo founder mode).
	Mine bool
	// MiningAddress is an optional P2PKH payout for generate RPC (overridden by testnet wallet when enabled).
	MiningAddress string
	// ConfSavePath is where dogecoinconf.json is read/written from the web Settings page.
	ConfSavePath string
	// EffectiveFile is the merged config shown in Settings (GET /api/config).
	EffectiveFile config.File
	// Stop requests graceful shutdown (same as Ctrl+C).
	Stop func()
	// Restart spawns a replacement node after a short delay then stops this process.
	Restart func() error
	// ApplyUpdate launches a verified update binary, replaces the install path, then stops this process.
	ApplyUpdate func(newExePath string) error
	// WaitParentPID waits for a prior process to exit before acquiring the datadir lock (web restart).
	WaitParentPID int
	// RawBlockBackfill is resolved post-genesis full-block fetch depth (0 = genesis only). Ignored when FullNode is false.
	RawBlockBackfill int
	// FullNode enables downloading and storing raw block payloads (false = SPV / headers-only mode).
	FullNode bool
	// NodeMode is "full" or "spv" for the web dashboard (must match FullNode semantics).
	NodeMode string
	// AllowUnverifiedMempool skips coinbase/script mempool admission (local testing only; not full-node safe).
	AllowUnverifiedMempool bool
	// FullRBF enables mempoolfullrbf-style replacement of non-signaling conflicts (default false).
	FullRBF bool
	// Standard is mempool relay standardness (dust, OP_RETURN, bare multisig).
	Standard consensus.StandardPolicy
	// MempoolLimits configures maxtxfee and package count/size limits.
	MempoolLimits consensus.MempoolRelayLimits
	// BlockMaxWeight is the GBT / mining weight limit (0 = consensus default).
	BlockMaxWeight int
	// NoTxIndex disables indexes/tx and a smaller default tip raw-block batch (full node only).
	NoTxIndex bool
	// BlockStorageOpts selects bundled blk*.dat and/or zstd compression for raw blocks.
	BlockStorageOpts store.BlockStorageOpts
	// TxIndexEmbedTx when false uses offset-only tx index entries (smaller disk).
	TxIndexEmbedTx bool
	// RpcUser / RpcPassword enable HTTP Basic auth on JSON-RPC when RpcUser is non-empty (from config file).
	RpcUser     string
	RpcPassword string
	// RpcCookie writes chaindatadir/.cookie on each start and enables HTTP Basic with those credentials (Core-style).
	RpcCookie bool
	// RpcAllowIP is extra JSON-RPC client allowlist (Core -rpcallowip); loopback always allowed.
	RpcAllowIP []string
	// RpcWhitelist restricts JSON-RPC methods when non-empty.
	RpcWhitelist []string
	// RpclimitPerMin / RpcAuthMaxFail optional JSON-RPC rate limits (see rpc/rpclimit.go).
	RpclimitPerMin int
	RpcAuthMaxFail int
	// P2PConnectivity is classic, cgnat, or both (see ParseP2PMode).
	P2PConnectivity string
	// Firewall is auto|always|never (OS firewall rules for P2P; see netfw package).
	Firewall string
	// Upnp is auto|enable|disable (UPnP/NAT-PMP port mapping; Core -upnp).
	Upnp string
	// ZmqPub* are Core -zmqpub* bind addresses (empty = disabled).
	ZmqPubHashBlock string
	ZmqPubHashTx    string
	ZmqPubRawTx     string
	ZmqPubRawBlock  string
	MaxOutbound     int
	MaxInbound      int
	// BlockSyncWorkers is parallel block-download TCP sessions (0 = derive from MaxOutbound).
	BlockSyncWorkers int
	// MaxOrphanTx caps the P2P orphan pool (0 = default 100).
	MaxOrphanTx int
	// MaxMempoolMB / MempoolExpiryHours configure mempool byte cap and max age (Core -maxmempool / -mempoolexpiry).
	MaxMempoolMB       int
	MempoolExpiryHours int
	// PersistMempool auto load/save dogego_mempool.json (Core -persistmempool).
	PersistMempool bool
	// AlertNotify runs a shell command when chain warnings change after sync (Core -alertnotify).
	AlertNotify string
	// AssumeValid is Core -assumevalid block hash hex (empty = network default; "0" = verify all scripts).
	AssumeValid string
	// CheckpointsEnabled enforces Core mapCheckpoints header hashes during sync (Core -checkpoints).
	CheckpointsEnabled bool
	// MaxTipAge is Core -maxtipage in seconds (0 = default 86400).
	MaxTipAge int
	// RpcTLS / WebUITLS optional PEM paths for native TLS (both cert and key required per listener).
	RpcTLS   httptls.Pair
	WebUITLS httptls.Pair
	localTLSMaterial *httptls.LocalMaterial
	// DogeGoRelayCGNAT configures integrated QUIC reachability relay (NODE_DOGEGO_RELAY_CGNAT).
	DogeGoRelayCGNAT config.DogeGoRelayCGNAT
	// CoreRPCAddr / CoreRPCUser / CoreRPCPassword for Core dumpwallet fallback (dogego_importwalletdat).
	CoreRPCAddr     string
	CoreRPCUser     string
	CoreRPCPassword string
	// SignerCmd runs an HWI-compatible external signer for PSBT (enumeratesigners, walletprocesspsbt).
	SignerCmd string
	// UpdateChecker optional shared release checker (tray + web UI). When nil, one is created for the datadir.
	UpdateChecker *version.UpdateChecker
	// OnWebUIReady runs once after the dashboard HTTP listener is up (dual-peer spawn, etc.).
	OnWebUIReady func()
}

// Run uses one outbound TCP peer (MVP). Dogecoin Core maintains several outbound (+ inbound)
// connections for redundancy and parallel block download; adding that requires a peer manager and
// one read loop per socket. Header sync and tip-window full blocks can overlap on this single link.
func Run(ctx context.Context, cfg Config) error {
	fmt.Fprintln(os.Stderr, version.Banner())
	nodeStart := time.Now()
	if cfg.DataDir == "" {
		return fmt.Errorf("node: -datadir is required")
	}
	p, err := chain.ParamsFor(cfg.Network)
	if err != nil {
		return err
	}
	if !cfg.EffectiveFile.DNSSeedLookupEnabled() {
		p = chain.WithoutDNSSeeds(p)
	} else if len(cfg.EffectiveFile.DNSSeeds) > 0 {
		p = chain.WithDNSSeeds(p, cfg.EffectiveFile.DNSSeeds)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return err
	}
	baseDataAbs := cfg.DataDir
	if a, err := filepath.Abs(cfg.DataDir); err == nil {
		baseDataAbs = a
	}
	if err := resolveNodeTLS(&cfg, baseDataAbs); err != nil {
		return fmt.Errorf("node: %w", err)
	}
	if err := cfg.RpcTLS.Validate(); err != nil {
		return fmt.Errorf("node: %w", err)
	}
	if err := cfg.WebUITLS.Validate(); err != nil {
		return fmt.Errorf("node: %w", err)
	}
	analyticsOn := cfg.FullNode && cfg.EffectiveFile.EmbeddedAnalyticsEnabled()
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		return err
	}
	chainRoot, migrated, err := PrepareChainDataDir(cfg.DataDir, cfg.Network, g80)
	if err != nil {
		return err
	}
	if migrated {
		fmt.Fprintf(os.Stderr, "DogeGo: chain data now under %s (per-network layout, like Core)\n", chainRoot)
	}
	if baseDataAbs == "" {
		baseDataAbs = cfg.DataDir
		if a, err := filepath.Abs(cfg.DataDir); err == nil {
			baseDataAbs = a
		}
	}
	chainDataAbs := chainRoot
	if a, err := filepath.Abs(chainRoot); err == nil {
		chainDataAbs = a
	}
	procLock, err := store.AcquireProcessLock(chainDataAbs)
	if err != nil {
		// Web UI restart: parent may still be releasing the lock for a moment.
		if cfg.WaitParentPID > 0 {
			deadline := time.Now().Add(15 * time.Second)
			for time.Now().Before(deadline) {
				time.Sleep(200 * time.Millisecond)
				procLock, err = store.AcquireProcessLock(chainDataAbs)
				if err == nil {
					break
				}
			}
		}
	}
	if err != nil {
		return fmt.Errorf("%w - only one dogego node per network datadir; stop the other process first", err)
	}
	defer procLock.Release()
	updateChecker := cfg.UpdateChecker
	if updateChecker == nil {
		updateChecker = version.NewUpdateChecker(baseDataAbs)
	}
	updateChecker.Start(ctx)
	updateChecker.PrintNotice(os.Stderr)
	learnedAddrsPath := filepath.Join(chainRoot, "learned_addrs.json")
	bootstrapAddrBook, _ := LoadAddrBook(learnedAddrsPath)
	netSlug := "testnet"
	if cfg.Network == chain.MainnetDogecoin {
		netSlug = "mainnet"
	}
	jpath := filepath.Join(chainRoot, "headers.bin")
	needsUncleanRepair := HasUncleanShutdown(chainRoot)
	clearUncleanOnExit := false
	// Registered first so it runs last (after timed flushes). Only clears when this run marked unclean at ready.
	defer func() {
		if clearUncleanOnExit {
			ClearUncleanShutdown(chainRoot)
		}
	}()
	j, err := store.OpenHeaderChain(chainRoot, g80[:])
	if err != nil {
		return err
	}
	h0, err := j.ReadHeaderAt(0)
	if err != nil {
		return fmt.Errorf("headers journal: read genesis: %w", err)
	}
	if !bytes.Equal(h0, g80[:]) {
		return fmt.Errorf("%s: headers.bin genesis does not match this build's %s genesis (height 0 bytes differ). "+
			"Peers will send headers that fail with 'bad prev'. Delete %q and restart, or use a -datadir that matches -network",
			netSlug, netSlug, jpath)
	}
	pool := mempool.New(5000)
	pool.SetPolicy(cfg.MaxMempoolMB, cfg.MempoolExpiryHours)
	pool.SetIncrementalRelayFeePerKB(consensus.IncrementalRelayFeePerKB())
	pool.SetTipHeightFn(func() int64 {
		if j == nil {
			return -1
		}
		h, err := j.TipHeight()
		if err != nil {
			return -1
		}
		return h
	})
	maxOrphan := cfg.MaxOrphanTx
	if maxOrphan <= 0 {
		maxOrphan = mempool.DefaultMaxOrphans
	}
	orphans := mempool.NewOrphanPool(maxOrphan)
	go func() {
		poolTick := time.NewTicker(10 * time.Minute)
		orphanTick := time.NewTicker(mempool.OrphanTxExpireInterval * time.Second)
		defer poolTick.Stop()
		defer orphanTick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-poolTick.C:
				if n := pool.PruneExpired(); n > 0 {
					applog.Line("mempool", fmt.Sprintf("expired %d mempool transaction(s)", n))
				}
			case <-orphanTick.C:
				if n := orphans.PruneExpired(); n > 0 {
					applog.Line("mempool", fmt.Sprintf("expired %d orphan transaction(s)", n))
				}
			}
		}
	}()
	banMgr := rpc.LoadFileBanManager(filepath.Join(chainRoot, "banlist.json"))
	misbehavior := NewMisbehaviorTracker(banMgr)
	misbehaviorPath := filepath.Join(chainRoot, "misbehavior_scores.json")
	LoadMisbehaviorScores(misbehavior, misbehaviorPath)
	misbehavior.SetPersistPath(misbehaviorPath)
	defer func() { _ = SaveMisbehaviorScores(misbehavior, misbehaviorPath) }()
	activity := applog.New(2500)
	applog.Register(activity)
	defer applog.Register(nil)
	var rbStore *store.RawBlockStore
	var txIx *store.TxIndex
	var addrIx *store.AddrIndex
	var filterIx *store.BlockFilterIndex
	if cfg.FullNode {
		blockOpts := cfg.BlockStorageOpts
		if blockOpts.Layout == "" {
			blockOpts = store.DefaultBlockStorageOpts()
		}
		if rb, err := store.OpenRawBlockStoreWithOpts(chainRoot, blockOpts); err != nil {
			fmt.Fprintf(os.Stderr, "raw blocks dir: %v\n", err)
		} else {
			rbStore = rb
			rbStore.ReconcileCountCacheFromDisk()
		}
		if !cfg.NoTxIndex {
			if ix, err := store.OpenTxIndexWithOpts(chainRoot, cfg.TxIndexEmbedTx); err != nil {
				fmt.Fprintf(os.Stderr, "tx index dir: %v\n", err)
			} else {
				txIx = ix
			}
		}
		var addrIxLocal *store.AddrIndex
		if txIx != nil {
			if ax, err := store.OpenAddrIndex(chainRoot); err != nil {
				fmt.Fprintf(os.Stderr, "addr index dir: %v\n", err)
			} else {
				addrIxLocal = ax
				addrIx = ax
			}
			if fx, err := store.OpenBlockFilterIndex(chainRoot); err != nil {
				fmt.Fprintf(os.Stderr, "block filter index: %v\n", err)
			} else {
				filterIx = fx
			}
		}
		if rbStore != nil {
			rbStore.EnableTxIndexing(txIx, txIx != nil)
			if addrIxLocal != nil && txIx != nil {
				addrIxLocal.SetResolver(txIx, rbStore)
				rbStore.EnableAddrIndexing(addrIxLocal, true)
			}
		}
	}
	localServices := chain.EffectiveP2PServices(p, filterIx != nil, cfg.DogeGoRelayCGNAT.AdvertiseServiceBit(), cfg.FullNode)
	var dgrMgr *dgr.Manager
	var dgrAdvertiseP2P string
	var disk *wallet.Disk
	if cfg.EnableWallet {
		wpath := filepath.Join(chainRoot, "wallet.json")
		disk, err = wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID)
		if err != nil {
			return fmt.Errorf("wallet: %w", err)
		}
		if cfg.Network == chain.MainnetDogecoin && !disk.IsEncrypted() {
			applog.Line("wallet", "SECURITY: mainnet wallet.json is not encrypted; run encryptwallet before storing funds")
		}
		if cfg.EffectiveFile.UACommentUseNodeTipEnabled() && !disk.NodeTipEnabled() {
			if _, err := disk.EnableNodeTip(); err != nil {
				applog.Line("wallet", "node tip: "+err.Error())
			}
		}
		runWalletAutoLock(ctx, disk)
	}
	var spvBloom *SPVBloomClient
	if !cfg.FullNode && disk != nil {
		spvBloom = NewSPVBloomClient(disk, p, j)
		if spvBloom.Active() {
			applog.Line("spv", "BIP37 wallet bloom filter ready (filtered-block sync against NODE_BLOOM peers)")
		}
	}
	peerSlot := "Starting"
	if cfg.Peer != "" {
		peerSlot = "Connecting"
	}
	subVer := chain.BuildSubVersion(cfg.EffectiveFile.EffectiveUAComment())
	p2pSettings, err := ParseP2PMode(cfg.P2PConnectivity, cfg.MaxOutbound, cfg.MaxInbound)
	if err != nil {
		return err
	}
	ensureOSFirewall(cfg.Firewall, p2pSettings.Listen, int(p.Port))
	var analyticsStore *analytics.SharedStore
	if analyticsOn {
		analyticsStore, err = analytics.OpenShared(filepath.Join(chainDataAbs, "dogego_analytics.db"))
		if err != nil {
			applog.Line("indexer", "analytics shared store: "+err.Error())
		} else {
			defer func() { _ = analyticsStore.Close() }()
		}
	}
	runtimeSvc := NewRuntimeServices(RuntimeServicesConfig{
		Parent:   ctx,
		Pool:     pool,
		FullNode: cfg.FullNode,
		AnalyticsCfg: func() analytics.SidecarConfig {
			var rawCounter analytics.RawBlockBinCounter
			if rbStore != nil {
				rawCounter = rbStore
			}
			var sharedDB *analytics.DB
			if analyticsStore != nil {
				sharedDB = analyticsStore.Writer()
			}
			return analytics.SidecarConfig{
				ChainRoot:      chainDataAbs,
				NetworkSlug:    netSlug,
				GenesisHashHex: pow.BlockHashHex(g80[:]),
				Journal:        j,
				RawBlocks:      rawCounter,
				DB:             sharedDB,
				SampleMetrics: func() analytics.LiveMetrics {
					m := analytics.LiveMetrics{}
					if pool != nil {
						m.MempoolTxs = pool.Count()
						m.MempoolBytes = int64(pool.TotalBytes())
					}
					hB, rB, tB, total := analytics.ChainStoreBytes(chainDataAbs)
					m.HeadersBytes, m.RawBlocksBytes, m.TxIndexBytes, m.ChainDataBytes = hB, rB, tB, total
					if rbStore != nil && j != nil {
						if tip, err := j.TipHeight(); err == nil && tip >= 0 {
							if h80, err := j.ReadHeaderAt(tip); err == nil {
								if payload, err := rbStore.Get(pow.BlockHashLE(h80)); err == nil {
									m.MaxRecentBlockBytes = int64(len(payload))
								}
							}
						}
					}
					return m
				},
				Log:  applog.Line,
				Tick: 25 * time.Second,
			}
		},
	})
	runtimeSvc.SetRPCConfigured(cfg.RPCAddr != "")
	var earlyRPC *rpc.EarlyServer
	var nodeRPCAuth *rpc.RPCAuth
	if cfg.RPCAddr != "" {
		nodeRPCAuth = buildNodeRPCAuth(cfg, chainDataAbs)
		es, err := rpc.StartEarlyListen(cfg.RPCAddr, cfg.RpcTLS, nodeRPCAuth, func(err error) {
			runtimeSvc.SetRPCListening(false)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				applog.Line("rpc", "JSON-RPC stopped: "+err.Error())
			}
		})
		if err != nil {
			applog.Line("rpc", "JSON-RPC listen: "+err.Error())
		} else {
			earlyRPC = es
			runtimeSvc.SetRPCListening(true)
			if cfg.RpcTLS.Enabled() {
				applog.Line("rpc", "JSON-RPC listening with TLS on "+cfg.RPCAddr+" (warming up)")
			} else {
				applog.Line("rpc", "JSON-RPC listening on "+cfg.RPCAddr+" (warming up)")
			}
		}
	}
	var conn net.Conn
	var connectedAddr string
	var mw *MsgWriter
	var soloMode bool
	var headerCatchUpPending atomic.Bool
	headerAttachCh := make(chan headerSyncPeer, 1)
	headerRecoverKickCh := make(chan struct{}, 1)
	var soloMiningOn atomic.Bool
	soloMineKick := make(chan struct{}, 1)
	var p2pNetCtr *netByteCounter
	var peerFromHandshake *wire.DecodedVersion
	var peerMgr *PeerMgr
	var walletSendBridge *ui.WalletSendBridge
	var walletTxsBridge *ui.WalletTxsBridge
	var rawFill progressiveRawState
	var bodyReplay bodyReplayTracker
	var blockStore *BlockStoreCtx
	var assistRegistry *AssistPeerRegistry
	var blockPeerScorer *BlockPeerScorer
	var assistCandidates *BlockAssistCandidates
	var discoveryFeed *PeerDiscoveryFeed
	var onChainTruncatedAfter func(int64)
	var uiContiguous atomic.Int64
	uiContiguous.Store(-1)
	var utxoCache *store.UtxoCache
	contiguousForUI := func() int64 {
		if blockStore != nil {
			return blockStore.ContiguousRawHeight()
		}
		if v := uiContiguous.Load(); v >= 0 {
			return v
		}
		return -1
	}
	chainWorkCache := NewChainWorkCache()
	chainWorkCache.Warm(j)
	var primaryCmpctHBFrom, primaryCmpctHBTo bool
	chainRPC := "test"
	if cfg.Network == chain.MainnetDogecoin {
		chainRPC = "main"
	}
	var chainRPCPaths *rpc.DataPaths
	var extMgr *extensions.Manager
	medianPeerOffset := func() int32 {
		if peerMgr != nil {
			return peerMgr.MedianTimeOffset()
		}
		if peerFromHandshake != nil {
			return wire.TimeOffsetSeconds(peerFromHandshake, time.Now().Unix())
		}
		return 0
	}
	ibdSnapshotPaths := func() *rpc.DataPaths {
		return mergeChainWorkIntoIBDPaths(
			mergeContiguousHeightIntoIBDPaths(
				mergeUtxoIntoIBDPaths(
					snapshotIBDPaths(chainRPCPaths, chain.EffectiveMaxTipAge(cfg.MaxTipAge), medianPeerOffset),
					utxoCache,
				),
				contiguousForUI,
			),
			j,
			chainWorkCache,
		)
	}
	var cachedIBD rpc.ChainIBDSnapshot
	var cachedIBDAt int64
	var cachedIBDMu sync.Mutex
	coreIBDSnap := func() rpc.ChainIBDSnapshot {
		now := time.Now().UnixNano()
		cachedIBDMu.Lock()
		defer cachedIBDMu.Unlock()
		if cachedIBDAt > 0 && now-cachedIBDAt < int64(500*time.Millisecond) {
			return cachedIBD
		}
		cachedIBD = ChainIBDSnapshot(j, chainRPC, rbStore, ibdSnapshotPaths())
		cachedIBDAt = now
		return cachedIBD
	}
	headerSyncDiagFn := func() map[string]interface{} {
		tip, ok := j.DiskTip()
		if !ok {
			return nil
		}
		paths := ibdSnapshotPaths()
		chainActive := int64(-1)
		if blockStore != nil {
			chainActive = ChainActiveHeight(j, rbStore, utxoCache, blockStore.ContiguousRawHeight)
		}
		out := rpc.HeaderSyncDiagnostics(j, tip, contiguousForUI(), paths)
		if out == nil {
			out = map[string]interface{}{}
		}
		rpc.MergeUtxoOperatorSummary(out, paths, chainActive)
		return out
	}
	var recoverHeaderJournalUI func() (HeaderJournalRecoveryResult, error)
	var afterHeaderJournalRewind func(HeaderJournalRecoveryResult)
	var uiRPCInvoke func(string, []json.RawMessage) map[string]interface{}
	if cfg.WebUIAddr != "" {
		walletSendBridge = &ui.WalletSendBridge{}
		walletTxsBridge = &ui.WalletTxsBridge{}
		chainDisplay := "testnet"
		if cfg.Network == chain.MainnetDogecoin {
			chainDisplay = "mainnet"
		}
		p2pSnap := func() map[string]any {
			tipH, _ := j.DiskTip()
			if tipH < 0 {
				tipH, _, _ = j.SyncTipFromDisk()
			}
			cont := contiguousForUI()
			chainActive := int64(-1)
			if blockStore != nil {
				chainActive = ChainActiveHeight(j, rbStore, utxoCache, blockStore.ContiguousRawHeight)
			}
			extras := P2PExtrasFromNode(assistRegistry, blockPeerScorer, chainActive, cont, rawFill.syncWorkerCount(), dedicatedHeaderRunning(), DedicatedHeaderPeerAddr())
			ibdProg := rawFill.snapshot()
			enrichIBDProgressSnapshot(ibdProg, j, blockStore)
			extras.IBDProgress = IBDProgressWithDiscoveryFeed(ibdProg, assistCandidates, discoveryFeed)
			ibdSnap := coreIBDSnap()
			mergeCoreIBDIntoProgress(extras.IBDProgress, ibdSnap)
			out := BuildP2PUISnapshot(p2pSettings, peerMgr, connectedAddr, peerSlot, extras)
			dialing := false
			if v, ok := out["peer_dialing"].(bool); ok {
				dialing = v
			}
			out["chain_active_height"] = ibdSnap.Blocks
			out["initialblockdownload"] = ibdSnap.IBD
			out["verification_progress"] = ibdSnap.VerificationProgress
			out["peer_addr"] = connectedAddr
			out["local_protocol_version"] = p.ProtocolVersion
			out["local_user_agent"] = subVer
			out["local_services_hex"] = rpc.FormatServicesHex(localServices)
			if dgrMgr != nil {
				dgrSnap := dgrMgr.MetricsSnapshot()
				if cfg.DogeGoRelayCGNAT.RoleInbound() {
					if ext, ok := out["upnp_external"].(string); ok && ext != "" {
						dgrMgr.SetAdvertiseHost(ext, cfg.DogeGoRelayCGNAT.EffectiveRelayPort())
						if host, _, err := net.SplitHostPort(ext); err == nil && host != "" {
							dgrAdvertiseP2P = net.JoinHostPort(host, fmt.Sprintf("%d", p.Port))
						} else {
							dgrAdvertiseP2P = net.JoinHostPort(strings.TrimSpace(ext), fmt.Sprintf("%d", p.Port))
						}
						dgrSnap = dgrMgr.MetricsSnapshot()
					}
				}
				out["dogego_relay_cgnat"] = dgrSnap
				if v, ok := dgrSnap["using_relay"].(bool); ok {
					out["using_relay"] = v
					if v {
						out["active_relay"] = dgrSnap["active_relay"]
					}
				}
			} else if cfg.DogeGoRelayCGNAT.Enabled {
				out["dogego_relay_cgnat"] = map[string]any{"enabled": true, "starting": true}
			}
			if p2pNetCtr != nil {
				out["tcp_bytes_recv"] = p2pNetCtr.Recv()
				out["tcp_bytes_sent"] = p2pNetCtr.Sent()
			}
			if peerFromHandshake != nil {
				out["peer_protocol_version"] = peerFromHandshake.ProtocolVersion
				out["peer_user_agent"] = peerFromHandshake.UserAgent
				out["peer_services_hex"] = rpc.FormatServicesHex(peerFromHandshake.Services)
				out["peer_start_height"] = peerFromHandshake.StartHeight
			}
			headerCatchUp := headerCatchUpPending.Load()
			bodyIBDPaused := blockStore != nil && ShouldPauseHeaderCatchUpForBodyIBD(blockStore, 0)
			if bodyIBDPaused {
				headerCatchUp = false
			}
			out["header_catch_up_pending"] = headerCatchUp
			out["dogego_body_ibd_header_paused"] = bodyIBDPaused
			out["block_assist_active"] = BlockAssistWorkersActive()
			if hint := headerSyncRecoveryHintStr(); hint != "" {
				out["dogego_header_sync_recovery"] = hint
			}
			var peerStart int32
			if peerFromHandshake != nil {
				peerStart = peerFromHandshake.StartHeight
			}
			if peerMgr != nil {
				if m := peerMgr.MaxPeerStartHeight(); m > peerStart {
					peerStart = m
				}
			}
			connTotal, connOut := 0, 0
			if v, ok := out["connections_total"].(int); ok {
				connTotal = v
			}
			if v, ok := out["connections_outbound"].(int); ok {
				connOut = v
			}
			assistN := 0
			if v, ok := out["block_assist_connections"].(int); ok {
				assistN = v
			}
			lowestMissing := int64(-1)
			inFlightBatches := 0
			bpm := 0.0
			if prog := extras.IBDProgress; prog != nil {
				if v, ok := prog["lowest_missing_height"].(int64); ok {
					lowestMissing = v
				}
				if v, ok := prog["in_flight_batches"].(int); ok {
					inFlightBatches = v
				}
				if v, ok := prog["blocks_per_minute"].(float64); ok {
					bpm = v
				}
			}
			recHint := ""
			if h, ok := out["dogego_header_sync_recovery"].(string); ok {
				recHint = h
			}
			connectBPM := 0.0
			connectLag := int64(0)
			if blockStore != nil && utxoCache != nil {
				connectLag = ConnectCatchUpLag(blockStore, utxoCache)
				connectBPM = IBDConnectBlocksPerMinute()
			}
			out["dogego_sync_activity"] = BuildSyncActivitySnapshot(SyncActivityInput{
				HeaderTip:              tipH,
				PeerStartHeight:        peerStart,
				HeaderCatchUpPending:   headerCatchUp,
				BodyIBDHeaderPaused:    bodyIBDPaused,
				HeaderRecoveryRunning:  headerRecoveryRunning(),
				DedicatedHeaderRunning: dedicatedHeaderRunning(),
				PrimaryPeer:            connectedAddr,
				PeerDialing:            dialing,
				ConnectionsTotal:       connTotal,
				ConnectionsOutbound:    connOut,
				BlockAssistActive:      BlockAssistWorkersActive(),
				BlockAssistConnections: assistN,
				ContiguousBodies:       cont,
				ChainActiveHeight:      chainActive,
				ConnectLag:             connectLag,
				ConnectBlocksPerMinute: connectBPM,
				LowestMissing:          lowestMissing,
				InFlightBatches:        inFlightBatches,
				BlocksPerMinute:        bpm,
				HeaderRecoveryHint:     recHint,
			})
			annotateCmpctHBCounts(out, peerMgr, primaryCmpctHBTo, primaryCmpctHBFrom)
			return out
		}
		var (
			p2pUICacheMu  sync.Mutex
			p2pUICached   map[string]any
			p2pUICachedAt time.Time
		)
		p2pSnapForDashboard := func() map[string]any {
			p2pUICacheMu.Lock()
			if time.Since(p2pUICachedAt) < 750*time.Millisecond && p2pUICached != nil {
				s := cloneStringAnyMap(p2pUICached)
				p2pUICacheMu.Unlock()
				return s
			}
			p2pUICacheMu.Unlock()
			s := p2pSnap()
			p2pUICacheMu.Lock()
			p2pUICached = cloneStringAnyMap(s)
			p2pUICachedAt = time.Now()
			p2pUICacheMu.Unlock()
			return s
		}
		runtimeSvc.SetP2PStatus(p2pSnapForDashboard)
		if peerSlot == "Starting" || peerSlot == "Connecting" {
			if cfg.Peer == "" {
				peerSlot = "Connecting"
			} else {
				peerSlot = cfg.Peer
			}
		}
		u, err := ui.Start(ctx, ui.StartConfig{
			ListenAddr:   cfg.WebUIAddr,
			TLS:          cfg.WebUITLS,
			GenesisHash:  pow.BlockHashHex(g80[:]),
			ChainDisplay: chainDisplay,
			Network:      netSlug,
			NodeMode:     cfg.NodeMode,
			PeerLabel:    &peerSlot,
			RPCAddr:      cfg.RPCAddr,
			RPCSnapshot: func() (listening, dispatchReady bool) {
				return runtimeSvc.RPCListening(), runtimeSvc.RPCDispatchReady()
			},
			Journal:                  j,
			RawBlocks:                rbStore,
			StorageSummary:           nativeStorageSummary(chainRoot, rbStore, txIx, contiguousForUI),
			TxIndex:                  txIx,
			AddrIndex:                addrIx,
			Pool:                     pool,
			OpenBrowser:              !cfg.WebUINoBrowser,
			Wallet:                   disk,
			MineRequested:            cfg.Mine,
			MiningActive:             &soloMiningOn,
			ConfSavePath:             cfg.ConfSavePath,
			EffectiveFile:            cfg.EffectiveFile,
			Stop:                     cfg.Stop,
			Restart:                  cfg.Restart,
			ApplyUpdate:              cfg.ApplyUpdate,
			BaseDataDir:              baseDataAbs,
			ChainDataDir:             chainDataAbs,
			ActivityLog:              activity,
			PubkeyHashAddrID:         p.PubkeyHashAddrID,
			EmbeddedAnalyticsSidecar: analyticsOn,
			AnalyticsRead: func() (*analytics.SideDetail, error) {
				if analyticsStore != nil {
					return analyticsStore.ReadDetail()
				}
				return analytics.ReadSideDetail(filepath.Join(chainDataAbs, "dogego_analytics.db"))
			},
			Services:                 runtimeSvc,
			WalletSend:               walletSendBridge,
			WalletTxs:                walletTxsBridge,
			P2PSnapshot:              p2pSnapForDashboard,
			DGRSnapshot: func() map[string]any {
				if dgrMgr != nil {
					return dgrMgr.MetricsSnapshot()
				}
				if cfg.DogeGoRelayCGNAT.Enabled {
					return map[string]any{"enabled": true, "starting": true}
				}
				return map[string]any{"enabled": false}
			},
			ContiguousRawHeight: contiguousForUI,
			ChainIBDSync:        coreIBDSnap,
			HeaderSyncDiag:      headerSyncDiagFn,
			RecoverHeaderJournal: func() (int64, int64, bool, error) {
				if recoverHeaderJournalUI == nil {
					return 0, 0, false, fmt.Errorf("header recovery not ready yet")
				}
				res, err := recoverHeaderJournalUI()
				return res.TipBefore, res.TipAfter, res.Rewound, err
			},
			RPCInvoke: func(method string, params []json.RawMessage) map[string]interface{} {
				if uiRPCInvoke == nil {
					return map[string]interface{}{
						"jsonrpc": "1.0",
						"id":      1,
						"error":   map[string]interface{}{"code": -1, "message": "RPC not ready yet"},
					}
				}
				return uiRPCInvoke(method, params)
			},
			Extensions:         extMgr,
			ExtensionManager:   func() *extensions.Manager { return extMgr },
			UtxoCache: func() *store.UtxoCache { return utxoCache },
			OrphanCount: func() int {
				if orphans != nil {
					return orphans.Count()
				}
				return 0
			},
			UpdateChecker: updateChecker,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "web UI: %v\n", err)
		} else if u != "" {
			fmt.Fprintf(os.Stderr, "web UI: %s (opens in browser; sync continues in background)\n", u)
			if cfg.OnWebUIReady != nil {
				go cfg.OnWebUIReady()
			}
		}
	}
	var auxJ *store.HeaderAuxJournal
	auxPath := filepath.Join(chainRoot, "headers_aux.bin")
	if hcount, err := j.Count(); err == nil {
		if auxJ, err = store.OpenHeaderAuxJournal(auxPath, hcount); err != nil {
			fmt.Fprintf(os.Stderr, "header aux journal: %v - rebuilding aligned index (backup: headers_aux.bin.corrupt)\n", err)
			fmt.Fprintf(os.Stderr, "rebuilding headers_aux.bin for %d header(s)…\n", hcount)
			if auxJ, err = store.RecoverHeaderAuxJournal(auxPath, hcount); err != nil {
				fmt.Fprintf(os.Stderr, "header aux rebuild: %v\n", err)
			} else {
				applog.Line("headers", fmt.Sprintf("rebuilt headers_aux.bin for %d header(s); will backfill auxpow from raw blocks when available", hcount))
			}
		}
		if auxJ != nil {
			if tip, err := j.TipHeight(); err == nil {
				if act := consensus.AuxpowActivationHeight(p.Net); act > 0 && tip >= act {
					if err := consensus.ValidateStoredHeaders(j, auxJ, p, act, act, time.Now().Unix()); err != nil {
						applog.Line("headers", fmt.Sprintf("headers_aux.bin invalid at activation height %d (%v) - rebuilding aux index", act, err))
						if auxJ, err = store.RecoverHeaderAuxJournal(auxPath, hcount); err != nil {
							fmt.Fprintf(os.Stderr, "header aux rebuild: %v\n", err)
						} else {
							applog.Line("headers", "rebuilt headers_aux.bin after activation-height auxpow check failed")
						}
					}
				}
			}
		}
	}
	if cfg.Mine && p.IsRebootTestnet() {
		applog.Line("mining", "reboot testnet mining active (scrypt PoW; coinbase → wallet address; background loop every 15s)")
	}
	peerFeeFilters := NewFeeFilterSet()
	addedNodes := NewAddedNodeStore()
	LoadAddedNodesFromConfig(addedNodes, cfg.EffectiveFile.AddedNodes, int(p.Port))
	if n := len(addedNodes.List()); n > 0 {
		applog.Line("net", fmt.Sprintf("addnode: %d persistent peer(s) from config", n))
	}
	dgrMgr = bootDGR(ctx, cfg, p2pSettings.Mode, netSlug, int(p.Port), p, addedNodes, &dgrAdvertiseP2P, func() []dgr.P2PRelayPeer {
		if peerMgr == nil {
			return nil
		}
		return peerMgr.RelayCGNATPeers()
	}, &peerMgr, chainRoot, &cfg.EffectiveFile)
	if dgrMgr != nil {
		defer dgrMgr.Close()
		defer ClearDGRTunnelDial()
	}
	var primaryDGRTunneled bool
	var lastSentFeeFilter uint64
	lastFeeFilterPoll := time.Now()
	lastSoloAttachTry := time.Now()
	lastAlertNotifyPoll := time.Now()
	var alertNotifySt alertNotifyState
	var primaryCmpctPending *cmpctPending
	var primaryLastSend, primaryLastRecv, primaryLastBlock, primaryLastTx time.Time
	var primaryPing peerPingTracker
	var primaryExclude PrimaryExclude
	var lastPrimaryRedial time.Time
	var primaryRedialStreak int
	lastTxIndexRepairPoll := time.Now()
	lastHeaderTopUp := time.Now()
	lastIBDProgressLog := time.Now()
	lastAssistCandRefresh := time.Now()
	lastGetAddrPoll := time.Now()
	lastIBDStallRecover := time.Time{}
	lastBodyIBDPump := time.Time{}
	lastHeaderCatchUpResumeKick := time.Time{}
	var bodyIBDHeaderWasPaused bool
	lastHeaderDiscoveryPoll := time.Time{}
	lastAuxBackfillPoll := time.Now()
	lastAutoRecoverPoll := time.Now()
	lastConnectCatchUpPoll := time.Time{}
	var utxoQuarantinedOnStartup bool
	if cfg.FullNode && rbStore != nil && txIx != nil {
		snapPath := store.UtxoSnapshotPath(chainRoot)
		var err error
		utxoCache, utxoQuarantinedOnStartup, err = LoadUtxoSnapshotAtStartup(snapPath, chainRoot, j, rbStore, p.Net)
		if err != nil {
			fmt.Fprintf(os.Stderr, "utxo snapshot load: %v\n", err)
			utxoCache = store.NewUtxoCache()
		} else if utxoCache != nil && utxoCache.TipHeight() >= 0 {
			InitIBDUtxoSnapshotFromTip(utxoCache.TipHeight())
			applog.Line("utxo", fmt.Sprintf("loaded snapshot through height %d (%d outputs)", utxoCache.TipHeight(), utxoCache.Count()))
			if m, err := store.LoadChainActiveManifest(chainRoot); err == nil && m != nil && m.UtxoTipHeight == utxoCache.TipHeight() {
				applog.Line("utxo", fmt.Sprintf("chain connect checkpoint restored (bodies through %d)", m.ContiguousRawHeight))
			}
		} else if utxoQuarantinedOnStartup {
			applog.Line("utxo", "starting with empty UTXO cache; replay connect from stored bodies")
		}
	}
	tipWait := rpc.NewTipWaiter()
	var saveUtxoSnapshotOnShutdown func()
	if utxoCache != nil {
		saveUtxoSnapshotOnShutdown = func() {
			if utxoCache.TipHeight() < 0 {
				return
			}
			ok := RunWithTimeout(ShutdownFlushBudget, func() {
				if err := PersistUtxoSnapshotIfAligned(blockStore, utxoCache, store.UtxoSnapshotPath(chainRoot), "shutdown"); err != nil {
					applog.Line("utxo", "shutdown snapshot save: "+err.Error())
				}
			})
			if !ok {
				applog.Line("utxo", "shutdown snapshot save timed out after "+ShutdownFlushBudget.String()+"; will repair on next start if needed")
			}
		}
		defer saveUtxoSnapshotOnShutdown()
	}
	feeHistoryPath := filepath.Join(chainDataAbs, "fee_history.json")
	feeEstimatesDatPath := filepath.Join(chainDataAbs, "fee_estimates.dat")
	feeHistory := consensus.NewFeeHistory(0)
	if loaded, err := consensus.LoadFeeHistoryFile(feeHistoryPath, 0); err != nil {
		applog.Line("mempool", "fee_history load: "+err.Error())
	} else if loaded != nil {
		feeHistory = loaded
		applog.Line("mempool", fmt.Sprintf("loaded fee history (%d blocks)", feeHistory.BlockCount()))
	}
	if best, stats, err := consensus.ReadCoreFeeEstimatesDat(feeEstimatesDatPath); err != nil {
		applog.Line("mempool", "fee_estimates.dat load: "+err.Error())
	} else if stats != nil {
		feeHistory.ApplyCoreConfirmStats(best, stats)
		applog.Line("mempool", "loaded fee_estimates.dat (Core TxConfirmStats)")
	}
	if tip, err := j.TipHeight(); err == nil && tip >= 0 {
		feeHistory.CatchUpBlockHeights(tip)
		if n := feeHistory.ApplyLoadedPendingTracks(tip); n > 0 {
			applog.Line("mempool", fmt.Sprintf("restored %d fee-estimator pending track(s) from fee_history.json", n))
		}
	}
	saveFeeHistory := func() {
		if err := feeHistory.SaveFile(feeHistoryPath); err != nil {
			applog.Line("mempool", "fee_history save: "+err.Error())
		}
		if err := feeHistory.SaveCoreFeeEstimatesDat(feeEstimatesDatPath); err != nil {
			applog.Line("mempool", "fee_estimates.dat save: "+err.Error())
		}
	}
	defer func() {
		if !RunWithTimeout(ShutdownFlushBudget, saveFeeHistory) {
			applog.Line("mempool", "fee_history save timed out after "+ShutdownFlushBudget.String())
		}
	}()
	blockStore = NewBlockStoreCtx(j, auxJ, p, rbStore, txIx, utxoCache)
	blockStore.ChainWork = chainWorkCache
	blockStore.NetworkSlug = netSlug
	if analyticsStore != nil {
		blockStore.Analytics = analyticsStore.Writer()
	}
	if rbStore != nil {
		rbStore.SetDeferIndexing(func() bool {
			return ShouldDeferTxIndexOnPut(blockStore)
		})
	}
	earlyChainName := "testnet"
	if cfg.Network == chain.MainnetDogecoin {
		earlyChainName = "main"
	}
	ensureExtensionManager(cfg, &extMgr, chainDataAbs, j, rbStore, txIx, utxoCache)
	if cfg.FullNode && rbStore != nil {
		if err := EnsureLocalGenesis(blockStore); err != nil {
			applog.Line("block", "local genesis (chainparams): "+err.Error())
		} else if !NeedsGenesisBlock(blockStore) {
			applog.Line("block", "genesis raw block ready (Core-style chainparams)")
		}
	}
	if rbStore != nil && blockStore != nil {
		diskContig := int64(-1)
		if tip, err := rbStore.ProbeBundledContiguousTip(); err == nil {
			diskContig = tip
		} else {
			applog.Line("block", "bundled contiguous probe: "+err.Error())
		}
		if diskContig >= 0 {
			if fixed, err := store.ReconcileRawBlockSyncCheckpoint(chainRoot, diskContig); err != nil {
				applog.Line("block", "reconcile rawblocks_sync: "+err.Error())
			} else if fixed {
				applog.Line("block", fmt.Sprintf("reconciled rawblocks_sync.json to bundled disk tip %d", diskContig))
			}
			blockStore.maybeClampBundledContiguousFromDisk()
		}
		if cp, err := store.LoadRawBlockSyncCheckpoint(chainRoot); err == nil && cp.ContiguousRawHeight >= 0 {
			seed := cp.ContiguousRawHeight
			if diskContig >= 0 && seed > diskContig {
				seed = diskContig
			}
			if blockStore.TrySeedContiguousFromCheckpoint(seed) {
				applog.Line("block", fmt.Sprintf("resuming contiguous coverage from checkpoint height %d", seed))
			}
		}
	}
	deferStartupRawPurge := cfg.FullNode && rbStore != nil && blockStore != nil && BodiesBehindHeaders(blockStore)
	runStartupRawBodyPurge := func() {
		if n, err := blockStore.PurgeInadequateRawBodies(); err != nil {
			applog.Line("block", "purge inadequate raw blocks: "+err.Error())
		} else if n > 0 {
			applog.Line("block", fmt.Sprintf("removed %d unreadable raw block file(s) (stub/undersized/corrupt); re-downloading", n))
		}
	}
	if utxoCache != nil && utxoCache.TipHeight() >= 0 {
		tip := utxoCache.TipHeight()
		cont := blockStore.ContiguousRawHeight()
		if tip > cont+1 {
			purgeReplay := func() {
				if n, err := blockStore.PurgeInadequateRawBodiesThroughHeight(tip); err != nil {
					applog.Line("block", "purge snapshot replay bodies: "+err.Error())
				} else if n > 0 {
					applog.Line("block", fmt.Sprintf("removed %d unreadable body file(s) through snapshot height %d", n, tip))
				}
				applog.Line("utxo", fmt.Sprintf("snapshot at height %d; re-fetching stored bodies through %d before connect advances past %d", tip, tip, tip))
			}
			// Large replay windows scan thousands of heights; defer so pre-P2P RPC activates promptly.
			if tip-cont > 512 {
				go purgeReplay()
			} else {
				purgeReplay()
			}
		} else if deferStartupRawPurge {
			go runStartupRawBodyPurge()
		} else {
			runStartupRawBodyPurge()
		}
	} else if deferStartupRawPurge {
		go runStartupRawBodyPurge()
	} else {
		runStartupRawBodyPurge()
	}
	if cfg.FullNode && rbStore != nil {
		if err := EnsureLocalGenesis(blockStore); err != nil {
			applog.Line("block", "local genesis after purge: "+err.Error())
		}
		ReconcileGenesisWithContiguous(blockStore)
	}
	if rbStore != nil {
		var refreshed int64
		if blockStore.utxoAheadOfStoredBodies() {
			refreshed = blockStore.RampReplayContiguousFromDisk()
		} else {
			refreshed = blockStore.RefreshContiguousTip()
		}
		if refreshed >= 0 {
			applog.Line("block", fmt.Sprintf("contiguous raw bodies through height %d", refreshed))
		}
		if utxoCache != nil && blockStore.utxoAheadOfStoredBodies() {
			applog.Line("utxo", fmt.Sprintf("body replay active: UTXO through %d, contiguous %d - keep node running until bodies catch up",
				utxoCache.TipHeight(), blockStore.ContiguousRawHeight()))
			bodyReplay.seedWasAhead()
		}
	}
	if rewound, err := maybeRewindCompressedHeaderPeriod(j, auxJ, p, blockStore); err != nil {
		if rewound {
			applog.Line("headers", err.Error())
		} else {
			applog.Line("headers", "compressed-period rewind: "+err.Error())
		}
	}
	if reset, err := MaybeResetCriticallyStaleHeadersAtStartup(j, auxJ, p, blockStore); err != nil {
		applog.Line("headers", "critically stale header reset: "+err.Error())
	} else if reset {
		rawFill.ResetAfterChainTruncate(blockStore)
		headerCatchUpPending.Store(true)
	}
	if reset, err := MaybeResetStuckAncientHeaderChain(j, auxJ, p, blockStore, 0); err != nil {
		applog.Line("headers", "stuck ancient header reset: "+err.Error())
	} else if reset {
		rawFill.ResetAfterChainTruncate(blockStore)
		headerCatchUpPending.Store(true)
	}
	if tip, err := j.TipHeight(); err == nil && isPostAuxEraStallTipMainnet(p.Net, tip) && shouldContinueHeaderCatchUpDuringIBD(j, 0) {
		if !ShouldPauseHeaderCatchUpForBodyIBD(blockStore, 0) {
			maybeNotePostAuxEraHeaderStall(p.Net, tip)
			headerCatchUpPending.Store(true)
		}
	}
	reconcileHeaderCatchUpPending(blockStore, &headerCatchUpPending, &rawFill)
	bodyIBDHeaderWasPaused = blockStore != nil && ShouldPauseHeaderCatchUpForBodyIBD(blockStore, 0)
	if cfg.FullNode && rbStore != nil && blockStore != nil {
		if fixed, err := store.ReconcileRawBlockSyncCheckpoint(chainRoot, blockStore.ContiguousRawHeight()); err != nil {
			applog.Line("block", "rawblocks_sync startup reconcile: "+err.Error())
		} else if fixed {
			applog.Line("block", "rawblocks_sync startup reconcile: clamped stale checkpoint to contiguous frontier")
		}
		rawFill.initProgressiveRawAtStartup(chainRoot, blockStore, EffectiveBlockSyncWorkersOpt(cfg.MaxOutbound, cfg.BlockSyncWorkers, cfg.EffectiveFile.IBDOptimizeEnabled())+1)
		if cont := blockStore.ContiguousRawHeight(); cont >= 0 {
			rawFill.SyncCheckpointToContiguous(cont)
		}
		blockStore.SetBodyDownloadRealign(func(missing int64) {
			rawFill.realignProbeToConnectFrontier(blockStore, missing)
		})
		if gap := ConnectBodyGapHeight(blockStore); gap >= 0 {
			rawFill.realignProbeToConnectFrontier(blockStore, gap)
		}
	}
	activateEarlyChainRPC(earlyChainRPCEnv{
		Cfg: cfg, RuntimeSvc: runtimeSvc, EarlyRPC: earlyRPC, ChainRPCPaths: &chainRPCPaths,
		UIRPCInvoke: &uiRPCInvoke, ExtMgr: extMgr, ChainName: earlyChainName, J: j, AuxJ: auxJ, Pool: pool,
		RbStore: rbStore, TxIx: txIx, FilterIx: filterIx, UtxoCache: utxoCache, BlockStore: blockStore,
		TipWait: tipWait, RawFill: &rawFill, FeeHistory: feeHistory, ChainWorkCache: chainWorkCache,
		ChainRoot: chainRoot, BaseDataAbs: baseDataAbs, ChainDataAbs: chainDataAbs, AnalyticsOn: analyticsOn,
		BanMgr: banMgr, Orphans: orphans, PeerFeeFilters: peerFeeFilters, ContiguousForUI: contiguousForUI,
		HeaderCatchUpPending: func() bool { return headerCatchUpPending.Load() },
		SaveUtxoShutdown:     saveUtxoSnapshotOnShutdown,
		Disk: disk, WIFVer: p.PrivKeyWIFVersion, PKHVer: p.PubkeyHashAddrID, SHVer: p.ScriptHashAddrID,
	})
	startIBDConnectWorkers(ctx, blockStore, utxoCache, utxoQuarantinedOnStartup)
	autoFilterRepair := autoRecoverFilterRepairFn(j, chainRoot, filterIx, txIx, rbStore)
	if needsUncleanRepair {
		applog.Line("recovery", "previous run did not shut down cleanly; repairing in background (node stays usable)")
	}
	go func() {
		if rewound, err := autoRecoverSweep(chainRoot, j, auxJ, p, blockStore, autoFilterRepair); err != nil {
			applog.Line("recovery", "startup repair: "+err.Error())
		} else if rewound {
			rawFill.ResetAfterChainTruncate(blockStore)
			headerCatchUpPending.Store(true)
		}
	}()
	// Mark while running so a crash or force-kill leaves a marker for next-start repair.
	MarkUncleanShutdown(chainRoot)
	RegisterIntentionalStopClearDir(chainRoot)
	clearUncleanOnExit = true
	blockStore.FeeHistory = feeHistory
	blockStore.FeeHistoryPath = feeHistoryPath
	blockStore.FeeEstimatesDatPath = feeEstimatesDatPath
	assumeNet := "mainnet"
	if cfg.Network == chain.RebootTestnet {
		assumeNet = "testnet"
	}
	consensus.SetHeaderCheckpointsEnabled(cfg.CheckpointsEnabled)
	assumeValid := consensus.NewAssumeValid(assumeNet, cfg.AssumeValid)
	if err := assumeValid.Resolve(j); err != nil {
		if th, _ := j.TipHeight(); th >= 0 && assumeValid.HashHex() != "" {
			applog.Line("consensus", fmt.Sprintf("assumevalid: target not in header chain yet (local tip %d); verifying all scripts until headers reach assume-valid height", th))
		} else {
			applog.Line("consensus", "assumevalid: "+err.Error())
		}
	} else if assumeValid.Resolved() {
		applog.Line("consensus", fmt.Sprintf("assumevalid through height %d (%s…) - skipping script checks on buried blocks (Core -assumevalid)", assumeValid.Height(), assumeValid.HashHex()[:12]))
	} else if h := assumeValid.HashHex(); h != "" {
		applog.Line("consensus", "assumevalid block not in header chain yet; verifying all scripts until it appears")
	}
	if th, _ := j.TipHeight(); th >= 0 {
		assumeValid.SetHeaderTip(th)
	}
	consensus.SetGlobalAssumeValid(assumeValid)
	if blockStore != nil {
		blockStore.AssumeValid = assumeValid
		blockStore.IBDOptimize = cfg.EffectiveFile.IBDOptimizeEnabled()
		blockStore.DbCacheMB = cfg.EffectiveFile.EffectiveDBCacheMB(systemFreeMemoryMB())
		if blockStore.IBDOptimize {
			applog.Line("consensus", fmt.Sprintf("ibd_optimize: on (headers-first body priority, assumevalid script skip when resolved, dbcache≈%d MB)", blockStore.DbCacheMB))
		} else if blockStore.DbCacheMB > 0 {
			applog.Line("consensus", fmt.Sprintf("dbcache: %d MB (UTXO flush budget)", blockStore.DbCacheMB))
		}
	}
	if cfg.PersistMempool {
		reloadPersistedMempool(pool, chainDataAbs, utxoCache, txIx, rbStore, j, cfg.Network, cfg.Standard, cfg.MempoolLimits, cfg.FullRBF, feeHistory)
	}
	defer func() {
		if !cfg.PersistMempool {
			return
		}
		ok := RunWithTimeout(ShutdownFlushBudget, func() {
			if err := mempool.SavePersisted(mempool.PersistPath(chainDataAbs), pool.RawBlobs(), pool.ExportFeeDeltas()); err != nil {
				applog.Line("mempool", "persist save: "+err.Error())
			}
		})
		if !ok {
			applog.Line("mempool", "persist save timed out after "+ShutdownFlushBudget.String())
		}
	}()
	pool.SetOnRemove(func(displayTxid string) {
		tip := int64(-1)
		if j != nil {
			if h, err := j.TipHeight(); err == nil {
				tip = h
			}
		}
		if feeHistory.RecordMempoolLeftWithoutConfirm(displayTxid, tip) {
			saveFeeHistory()
		}
	})
	trackInboundMempoolFee := func(raw []byte) {
		if feeHistory == nil || j == nil {
			return
		}
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			return
		}
		adm := consensus.NewMempoolAdmissionWithUtxo(pool, pool, utxoCache, txIx, rbStore, j, cfg.Network)
		tip, err := j.TipHeight()
		if err != nil || tip < 0 {
			return
		}
		consensus.TrackMempoolTxFee(feeHistory, tx, raw, adm.View, tip)
	}
	var chainPolicy *store.ChainPolicy
	if policy, err := store.LoadChainPolicy(chainDataAbs); err != nil {
		applog.Line("chain", "chain_policy load: "+err.Error())
	} else {
		chainPolicy = policy
		blockStore.Policy = chainPolicy
	}
	NotifyRPCTip(j, rbStore, utxoCache, tipWait)
	var zmqHub *zmqnotify.Hub
	zmqCfg := zmqnotify.Config{
		PubHashBlock: cfg.ZmqPubHashBlock,
		PubHashTx:    cfg.ZmqPubHashTx,
		PubRawBlock:  cfg.ZmqPubRawBlock,
		PubRawTx:     cfg.ZmqPubRawTx,
	}
	if zmqCfg.Enabled() {
		hub, err := zmqnotify.Start(ctx, zmqCfg)
		if err != nil {
			applog.Line("zmq", "startup: "+err.Error())
		} else {
			zmqHub = hub
			defer zmqHub.Stop()
			pool.SetOnAdd(func(raw []byte) {
				rpc.NotifyGBTWake()
				zmqHub.NotifyTx(raw)
			})
		}
	} else {
		pool.SetOnAdd(func(raw []byte) { rpc.NotifyGBTWake() })
	}
	var tipBFCoord *tipBackfillCoordinator
	var mpSync mempoolSyncState
	var lastInvTxQuietLog time.Time
	var utxoIBD *utxoIBDSync
	var lastFilterContiguous int64 = -1
	blockPeerScorePath := filepath.Join(chainRoot, "block_peer_scores.json")
	if cfg.FullNode {
		assistRegistry = NewAssistPeerRegistry()
		blockPeerScorer = NewBlockPeerScorer()
		if err := blockPeerScorer.LoadFromFile(blockPeerScorePath); err != nil {
			applog.Line("block", "block_peer_scores load: "+err.Error())
		}
		go blockPeerScorer.RunSaveLoop(ctx, blockPeerScorePath)
		SetIBDAssistDiagnostics(func() *BlockAssistCandidates { return assistCandidates }, assistRegistry)
		tipH, _ := j.TipHeight()
		if filterIx != nil && blockStore != nil {
			if cont := blockStore.ContiguousRawHeight(); cont >= 0 && BodiesBehindHeaders(blockStore) {
				lastFilterContiguous = cont
				SetFilterIndexThrough(cont)
			}
		}
		if auxJ != nil && rbStore != nil {
			go func() {
				through := tipH
				if cont := blockStore.ContiguousRawHeight(); cont >= 0 && tipH-cont > 2048 {
					through = cont + 2048
				}
				n, err := store.BackfillAuxThroughHeight(j, auxJ, rbStore, through)
				if err != nil {
					applog.Line("headers", "aux backfill: "+err.Error())
				} else if n > 0 {
					applog.Line("headers", fmt.Sprintf("aux backfill: %d header auxpow record(s) from block store (through %d)", n, through))
				}
			}()
		}
		if cfg.RawBlockBackfill > 0 {
			cont0 := blockStore.ContiguousRawHeight()
			tipBFCoord = newTipBackfillCoordinator(cfg.RawBlockBackfill, ShouldDeferTipBackfill(tipH, cont0))
		}
		if utxoCache != nil {
			utxoIBD = newUtxoIBDSync(chainRoot)
		}
		blockStore.SetOnTipChanged(func(h int64) {
			rawFill.OnTipChanged(h)
			if assumeValid != nil {
				assumeValid.SetHeaderTip(h)
				if assumeValid.TryResolve(j) {
					applog.Line("consensus", fmt.Sprintf("assumevalid through height %d (%s…) - script skip enabled on buried blocks", assumeValid.Height(), assumeValid.HashHex()[:12]))
				}
			}
			NotifyRPCTip(j, rbStore, utxoCache, tipWait)
		})
		blockStore.SetOnChainTruncating(func(keepThrough int64) {
			_ = keepThrough
			rawFill.ResetInFlightForHeaderRewind()
		})
		blockStore.SetOnChainTruncated(func(keepThrough int64) {
			consensus.ClearMedianTimePastCache()
			rpc.ResetIBDExitLatch()
			mpSync.reset()
			uiContiguous.Store(-1)
			rawFill.ResetAfterChainTruncate(blockStore)
			if onChainTruncatedAfter != nil {
				onChainTruncatedAfter(keepThrough)
			}
			if assumeValid != nil {
				avH := assumeValid.Height()
				if avH < 0 || avH > keepThrough {
					assumeValid.ClearResolution()
					if assumeValid.TryResolve(j) {
						applog.Line("consensus", fmt.Sprintf("assumevalid re-resolved through height %d after chain rewind", assumeValid.Height()))
					}
				}
			}
			if ch := blockStore.ContiguousRawHeight(); ch >= 0 {
				uiContiguous.Store(ch)
			}
		})
		blockStore.SetOnChainActiveAdvance(func(h int64) {
			NotifyRPCTip(j, rbStore, utxoCache, tipWait)
			if zmqHub != nil {
				zmqHub.NotifyBlockAt(j, rbStore, h)
			}
			if extMgr != nil {
				extMgr.NotifyBlockConnected(h)
			}
		})
		blockStore.SetOnContiguousAdvance(func(cont int64) {
			uiContiguous.Store(cont)
			if assumeValid != nil && assumeValid.TryResolve(j) {
				applog.Line("consensus", fmt.Sprintf("assumevalid through height %d (%s…) - script skip enabled on buried blocks", assumeValid.Height(), assumeValid.HashHex()[:12]))
			}
			if shouldPersistSyncCheckpoint(cont, blockStore) {
				rawFill.SyncCheckpointToContiguous(cont)
			}
			bodyReplay.noteContiguousAdvance(blockStore, utxoCache, cont, chainRoot, rawFill.SyncCheckpointToContiguous, func(bs *BlockStoreCtx, utxo *store.UtxoCache, c int64) {
				onBodyReplayComplete(bs, utxo, c, &rawFill)
			})
			if utxoIBD != nil {
				utxoIBD.onContiguousAdvance(blockStore, utxoCache)
			}
			if filterIx != nil && txIx != nil {
				onContiguousAdvanceIndexFilters(blockStore, &lastFilterContiguous, cont, j, rbStore, filterIx, txIx)
			}
			NotifyRPCTip(j, rbStore, utxoCache, tipWait)
		})
	}
	go func() {
		if uiContiguous.Load() >= 0 {
			return
		}
		uiContiguous.Store(blockStore.ContiguousRawHeight())
	}()
	if rbStore != nil {
		rbStore.SetBlockPutSideband(&store.BlockPutSideband{
			Journal: j,
			Aux:     auxJ,
			Network: cfg.Network,
			ContiguousHeight: func() int64 {
				if blockStore == nil {
					return -1
				}
				return blockStore.ContiguousRawHeight()
			},
			Pool: pool,
			CollectMempoolConfirmed: func(blockRaw []byte, blockHeight int64) []store.MempoolConfirmFeeSample {
				if len(blockRaw) < 80 {
					return nil
				}
				adm := consensus.NewMempoolAdmissionWithUtxo(pool, pool, utxoCache, txIx, rbStore, j, cfg.Network)
				raw := consensus.CollectMempoolConfirmedSamplesRaw(blockRaw, pool, adm.View, blockHeight)
				out := make([]store.MempoolConfirmFeeSample, len(raw))
				for i, s := range raw {
					out[i] = store.MempoolConfirmFeeSample{
						TxID: s.TxID, FeeratePerKB: s.FeeratePerKB, BlocksWaited: s.BlocksWaited,
					}
				}
				return out
			},
			RecordMempoolConfirmed: func(blockHeight int64, samples []store.MempoolConfirmFeeSample) {
				if blockHeight >= 0 {
					feeHistory.NotifyBlockHeight(blockHeight)
				}
				if len(samples) > 0 {
					conv := make([]consensus.MempoolConfirmSample, len(samples))
					for i, s := range samples {
						conv[i] = consensus.MempoolConfirmSample{
							TxID: s.TxID, FeeratePerKB: s.FeeratePerKB, BlocksWaited: s.BlocksWaited,
						}
					}
					feeHistory.RecordMempoolConfirmedSamples(conv)
				}
				saveFeeHistory()
			},
			AfterBlockStored: func(blockRaw []byte) {
				if len(blockRaw) < 80 {
					return
				}
				adm := consensus.NewMempoolAdmissionWithUtxo(pool, pool, utxoCache, txIx, rbStore, j, cfg.Network)
				adm.MinRelayFeePerKB = consensus.EffectiveMinRelayFeePerKB(peerFeeFilters.Max(), pool.MinRelayFeePerKB())
				adm.FullRBF = cfg.FullRBF
				adm.Standard = cfg.Standard
				cfg.MempoolLimits.Apply(&adm)
				consensus.PromoteOrphansForBlockRaw(blockRaw, pool, orphans, adm)
			},
			IndexBlockFilter: func(hashLE [32]byte, blockRaw []byte) error {
				return blockFilterIndexOnPut(blockStore, j, filterIx, rbStore, txIx, hashLE, blockRaw)
			},
		})
	}
	if !cfg.FullNode {
		if spvBloom != nil && spvBloom.Active() {
			applog.Line("ui", "SPV mode: headers + BIP37 filtered blocks (wallet bloom); raw block store disabled")
		} else {
			applog.Line("ui", "SPV (headers-only) mode: raw block store disabled; enable wallet for BIP37 bloom sync")
		}
	}
	if p.Net == chain.MainnetDogecoin {
		if err := consensus.VerifyLegacySubsidyRNG(); err != nil {
			fmt.Fprintf(os.Stderr, "consensus: %v (binary does not match Core legacy subsidy; rebuild from current DogeGo source)\n", err)
			os.Exit(1)
		}
	}
	applog.Line("ui", "DogeGo node process up; initializing P2P and dashboard")
	ibdOptimize := cfg.EffectiveFile.IBDOptimizeEnabled()
	blockSyncWorkers := func() int {
		return EffectiveBlockSyncWorkersOpt(cfg.MaxOutbound, cfg.BlockSyncWorkers, ibdOptimize)
	}
	deferAnalyticsForIBD := analyticsOn && analyticsStore != nil && ibdOptimize &&
		blockStore != nil && BodiesBehindHeaders(blockStore)
	if !analyticsOn {
		applog.Line("indexer", "embedded analytics sidecar disabled in config (use dogego indexer for manual scans)")
	} else if analyticsStore == nil {
		applog.Line("indexer", "embedded analytics sidecar skipped (could not open dogego_analytics.db)")
	} else if deferAnalyticsForIBD {
		applog.Line("indexer", "embedded analytics sidecar deferred until IBD completes (ibd_optimize)")
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if blockStore == nil || !BodiesBehindHeaders(blockStore) {
						if err := runtimeSvc.StartAnalytics(); err != nil {
							applog.Line("indexer", "embedded analytics sidecar (post-IBD): "+err.Error())
						} else {
							applog.Line("indexer", "embedded analytics sidecar started after IBD")
						}
						return
					}
				}
			}
		}()
	} else if err := runtimeSvc.StartAnalytics(); err != nil {
		applog.Line("indexer", "embedded analytics sidecar: "+err.Error())
	}
	// Live-updating label for the dashboard (shown before and after P2P connect).
	peerLabel := cfg.Peer
	if peerLabel == "" {
		peerLabel = "Connecting"
	}
	peerSlot = peerLabel
	d := net.Dialer{Timeout: 15 * time.Second}
	blockAssistLaunch := func() BlockAssistLaunchParams {
		return BlockAssistLaunchParams{
			Ctx: ctx, Dialer: d, Candidates: assistCandidates, PrimaryExcl: &primaryExclude,
			Params: p, UserAgent: subVer, LocalServices: localServices,
			BlockStore: blockStore, Raw: &rawFill,
			Workers: blockSyncWorkers(),
			Scorer:  blockPeerScorer, AssistReg: assistRegistry,
			AddrBook: activeAddrBook(peerMgr, bootstrapAddrBook),
		}
	}

	var discoveredPeers []string
	discoveryFeed = NewPeerDiscoveryFeed(nil)
	refreshPeerDiscovery := func() []string {
		applog.Line("net", "refreshing peer discovery (DNS seeds + fixed seeds)")
		discoveredPeers = p2p.DiscoverAddresses(ctx, p, func(msg string) { applog.Line("net", msg) })
		if discoveryFeed == nil {
			discoveryFeed = NewPeerDiscoveryFeed(discoveredPeers)
		} else {
			for _, a := range discoveredPeers {
				discoveryFeed.Note(a)
			}
		}
		return discoveredPeers
	}
	ensureAssistPool := func() {
		if assistCandidates != nil {
			return
		}
		assistCandidates = seedBlockAssistCandidates(ctx, p, blockStore, blockPeerScorer, discoveryFeed, discoveredPeers)
	}
	armPreP2PBlockAssist := func() {
		if !cfg.FullNode || rbStore == nil || blockStore == nil || !BodiesBehindHeaders(blockStore) {
			return
		}
		ensureAssistPool()
		MaybeEnsureBlockAssistWorkers(blockAssistLaunch())
		go func() {
			refreshPeerDiscovery()
			if assistCandidates == nil {
				ensureAssistPool()
			} else {
				RefreshBlockAssistPool(assistCandidates, DiscoverySnapshot(discoveryFeed, discoveredPeers), peerMgr, blockPeerScorer, blockStore, addedNodes.List())
			}
			MaybeEnsureBlockAssistWorkers(blockAssistLaunch())
		}()
		applog.Line("block", "pre-P2P block-assist armed (forward body IBD while headers connect)")
	}
	armPreP2PBlockAssist()
	headerSyncViaDiscovery := cfg.Peer == ""
	if cfg.Peer != "" {
		discoveryFeed.Note(cfg.Peer)
		connectedAddr = cfg.Peer
		applog.Line("net", "dialing configured peer "+cfg.Peer)
		RecordOutboundDialTry(bootstrapAddrBook, cfg.Peer)
		c, viaDGR, err := DialP2POutbound(ctx, d, cfg.Peer)
		if err != nil {
			RecordOutboundHandshakeResult(bootstrapAddrBook, cfg.Peer, err)
			applog.Line("net", fmt.Sprintf("configured peer %s unreachable (%v); falling back to DNS/fixed seeds", cfg.Peer, err))
			headerSyncViaDiscovery = true
			connectedAddr = ""
			peerSlot = "Connecting"
		} else {
			primaryDGRTunneled = viaDGR
			if viaDGR {
				applog.Line("dgr", "primary sync peer "+cfg.Peer+" via DGR tunnel")
			}
			p2pNetCtr = newNetByteCounter()
			conn = &countingConn{Conn: c, ctr: p2pNetCtr}
			applog.Line("net", "TCP connected to "+cfg.Peer+", completing handshake")
			peerSlot = cfg.Peer + " · handshake"
			dv, err := Handshake(ctx, conn, p, subVer, localServices)
			if err != nil {
				RecordOutboundHandshakeResult(bootstrapAddrBook, cfg.Peer, err)
				_ = conn.Close()
				conn = nil
				applog.Line("net", fmt.Sprintf("configured peer %s handshake failed (%v); falling back to DNS/fixed seeds", cfg.Peer, err))
				headerSyncViaDiscovery = true
				connectedAddr = ""
				peerSlot = "Connecting"
			} else {
				RecordOutboundPeerHandshake(bootstrapAddrBook, blockPeerScorer, cfg.Peer, dv, nil)
				MaybeSaveAddrBookIfDirty(learnedAddrsPath, bootstrapAddrBook)
				peerFromHandshake = dv
				blockStore.SetNetworkTimeSource(nil, peerFromHandshake)
				applog.Line("net", "handshake complete with "+connectedAddr)
				peerSlot = connectedAddr
				mw = NewMsgWriter(conn, p.Magic)
				mw.PeerAddr = connectedAddr
				AttachWriterMsgStats(mw)
				maybeNegotiateExtensions(conn, p.Magic, connectedAddr, extMgr, mw)
				if peerMgr != nil {
					if peerMgr.OfferPrimaryCmpctHB(mw, &primaryCmpctHBTo) && blockStore != nil {
						blockStore.SetPrimaryCmpctHBTo(primaryCmpctHBTo)
					}
				}
				if peerMgr == nil && blockStore != nil {
					WireSoloPrimaryBlockPeerStats(blockStore, connectedAddr, func() { primaryLastBlock = time.Now() })
				}
				MaybePushSPVBloom(spvBloom, mw, peerFromHandshake)
				if cfg.FullNode && rbStore != nil {
					primaryExclude.Set(connectedAddr)
					if assistCandidates == nil {
						assistCandidates = seedBlockAssistCandidates(ctx, p, blockStore, blockPeerScorer, discoveryFeed, discoveredPeers)
					}
					MaybeEnsureBlockAssistWorkers(blockAssistLaunch())
				}
				applog.Line("headers", "starting headers-first sync (getheaders → headers); block bodies fetch in parallel on assist peers")
				peerStart := int32(0)
				if peerFromHandshake != nil {
					peerStart = peerFromHandshake.StartHeight
				}
				if err := DownloadHeaders(ctx, mw, p, j, auxJ, peerFeeFilters, blockStore, cfg.RawBlockBackfill, &rawFill, peerStart, discoveryFeed, false, blockPeerScorer, activeAddrBook(peerMgr, bootstrapAddrBook)); err != nil {
					if IsHeaderRewindRetryErr(err) {
						applog.Line("headers", err.Error())
						headerSyncViaDiscovery = true
						if conn != nil {
							_ = conn.Close()
							conn = nil
						}
						mw = nil
						connectedAddr = ""
						peerFromHandshake = nil
						peerSlot = "Connecting"
					} else if recoverableHeaderPeerErr(err) {
						noteHeaderSyncPeerFailure(blockPeerScorer, activeAddrBook(peerMgr, bootstrapAddrBook), connectedAddr, err)
						applog.Line("headers", fmt.Sprintf("configured peer %s dropped during header sync (%v); falling back to DNS/fixed seeds", cfg.Peer, err))
						if conn != nil {
							_ = conn.Close()
							conn = nil
						}
						mw = nil
						connectedAddr = ""
						peerFromHandshake = nil
						peerSlot = "Connecting"
						headerSyncViaDiscovery = true
					} else {
						return fmt.Errorf("headers: %w", err)
					}
				}
			}
		}
	}
	if headerSyncViaDiscovery {
		applog.Line("net", "peer discovery (DNS seeds like Core, then fixed seeds from chainparamsseeds.h)")
		discoveredPeers = p2p.DiscoverAddresses(ctx, p, func(msg string) { applog.Line("net", msg) })
		discoveryFeed = NewPeerDiscoveryFeed(discoveredPeers)
		peers := HeaderSyncProbeCandidates(discoveredPeers, blockPeerScorer, addedNodes.List())
		if len(peers) == 0 {
			if canEnterSoloMode(p, j) {
				soloMode = true
				logSoloMode("no peer candidates (empty DNS/fixed seed list for this network)", nil)
			} else if HasLocalHeaderChain(j) {
				headerCatchUpPending.Store(true)
				setHeaderSyncRecoveryHint("Peer discovery returned no candidates; retrying header sync in background while block download continues.")
				applog.Line("headers", "no peer candidates from DNS/fixed seeds - local headers on disk; node stays up and retries in background")
			} else {
				return fmt.Errorf("peer discovery: no candidates (DNS returned nothing and fixed seed list empty - check network name and connectivity)")
			}
		} else if !soloMode {
			probed, probeErr := probeHeaderSyncPeers(ctx, d, peers, p, subVer, localServices, headerSyncPeerProbeMax, blockPeerScorer, bootstrapAddrBook)
			MaybeSaveAddrBookIfDirty(learnedAddrsPath, bootstrapAddrBook)
			if probeErr != nil {
				if canEnterSoloMode(p, j) {
					soloMode = true
					logSoloMode("no peer handshakes succeeded", probeErr)
				} else if HasLocalHeaderChain(j) && shouldAutoRecoverHeaderSync(probeErr) {
					headerCatchUpPending.Store(true)
					noteHeaderSyncFailure(probeErr)
					applog.Line("headers", fmt.Sprintf("peer probe failed (%v); local headers on disk - node stays up and retries in background", probeErr))
				} else {
					return fmt.Errorf("peer discovery: %w", probeErr)
				}
			}
			if !soloMode {
				if cfg.FullNode && rbStore != nil {
					rawFill.PrepareAtStartup(blockStore)
				}
				if headerCatchUpPending.Load() {
					applog.Line("headers", "skipping inline header sync - background recovery will retry")
				} else if len(probed) > 0 {
					var lastHeaderErr error
					if j != nil {
						j.ReconcileCountCacheFromDisk()
					}
					headerPeer := probed[0]
					if cfg.FullNode && rbStore != nil {
						primaryExclude.Set(headerPeer.addr)
						if assistCandidates == nil {
							assistCandidates = seedBlockAssistCandidates(ctx, p, blockStore, blockPeerScorer, discoveryFeed, discoveredPeers)
							StartBlockAssistWorkers(ctx, d, assistCandidates, &primaryExclude, p, subVer, localServices, blockStore, &rawFill, blockSyncWorkers(), blockPeerScorer, assistRegistry, activeAddrBook(peerMgr, bootstrapAddrBook))
						}
					}
					if ShouldPauseHeaderCatchUpForBodyIBD(blockStore, headerPeer.startHeight()) {
						reconcileHeaderCatchUpPending(blockStore, &headerCatchUpPending, &rawFill)
						applog.Line("headers", "forward block IBD owns pipeline - dedicated header sync deferred until bodies catch up")
					} else {
						dedicatedEnv := DedicatedHeaderSyncEnv{
							Ctx: ctx, Params: p, Journal: j, Aux: auxJ, FeeFilters: peerFeeFilters,
							BlockStore: blockStore, RawBackfill: cfg.RawBlockBackfill, RawFill: &rawFill,
							DiscoveryFeed: discoveryFeed, Scorer: blockPeerScorer, AddrBook: activeAddrBook(peerMgr, bootstrapAddrBook),
							OnYieldOrDone: func() {
								if ShouldPauseHeaderCatchUpForBodyIBD(blockStore, 0) {
									reconcileHeaderCatchUpPending(blockStore, &headerCatchUpPending, &rawFill)
									return
								}
								headerCatchUpPending.Store(true)
								select {
								case headerRecoverKickCh <- struct{}{}:
								default:
								}
							},
							OnCaughtUp: func() {
								headerCatchUpPending.Store(false)
								clearHeaderSyncRecoveryHint()
							},
						}
						if StartDedicatedHeaderSync(dedicatedEnv, headerPeer) {
							headerCatchUpPending.Store(true)
							setHeaderSyncRecoveryHint("Headers download on a dedicated peer; primary link used for block bodies (Core-style IBD).")
						} else {
							headerCatchUpPending.Store(true)
							applog.Line("headers", "dedicated header sync already running on another peer")
						}
					}
					var primaryPeer headerSyncPeer
					hasPrimary := false
					wantBlocks := blockFetchWantHeight(blockStore)
					primaryPeer, hasPrimary = pickBlockPrimaryPeer(probed, headerPeer.addr, wantBlocks)
					if !hasPrimary {
						dialPeers := HeaderSyncProbeCandidates(DiscoverySnapshot(discoveryFeed, discoveredPeers), blockPeerScorer, addedNodes.List())
						extra, err := dialExtraHeaderSyncPeer(ctx, d, dialPeers, p, subVer, localServices, headerPeer.addr, blockPeerScorer, bootstrapAddrBook, wantBlocks)
						MaybeSaveAddrBookIfDirty(learnedAddrsPath, bootstrapAddrBook)
						if err == nil {
							primaryPeer = extra
							hasPrimary = true
						} else {
							lastHeaderErr = err
							applog.Line("headers", fmt.Sprintf("no separate primary peer yet (%v); block-assist continues headers on %s", err, headerPeer.addr))
						}
					}
					if hasPrimary {
						conn = primaryPeer.conn
						connectedAddr = primaryPeer.addr
						mw = primaryPeer.mw
						p2pNetCtr = primaryPeer.ctr
						peerFromHandshake = primaryPeer.dv
						peerSlot = primaryPeer.addr
						blockStore.SetNetworkTimeSource(nil, primaryPeer.dv)
						if cfg.FullNode && rbStore != nil && NeedsGenesisBlock(blockStore) {
							if err := SyncGenesisRawBlock(ctx, mw, p, blockStore); err != nil {
								applog.Line("block", "genesis fetch on primary: "+err.Error())
							}
						}
						clearHeaderSyncRecoveryHint()
						for _, p := range probed {
							if p.addr != headerPeer.addr && p.addr != primaryPeer.addr {
								closeHeaderSyncPeer(p)
							}
						}
						applog.Line("net", fmt.Sprintf("using outbound peer %s for primary block fetch (headers on %s; want height %d)", primaryPeer.addr, headerPeer.addr, wantBlocks))
						MaybePushSPVBloom(spvBloom, mw, peerFromHandshake)
					} else {
						for i := 1; i < len(probed); i++ {
							closeHeaderSyncPeer(probed[i])
						}
						applog.Line("headers", "entering main loop without primary - block-assist active; headers on dedicated peer")
					}
					if lastHeaderErr != nil && shouldAutoRecoverHeaderSync(lastHeaderErr) {
						noteHeaderSyncFailure(lastHeaderErr)
					}
				} // len(probed) > 0
			}
		}
	}
	if soloMode {
		peerSlot = "solo (reboot testnet founder - no peers connected)"
		connectedAddr = ""
		if cfg.FullNode && rbStore != nil {
			rawFill.PrepareAtStartup(blockStore)
		}
	}
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	if soloMode {
		applog.Line("net", "solo founder mode: mining locally; P2P listen/outbound active when configured (reboot testnet)")
	} else {
		applog.Line("net", fmt.Sprintf("P2P mode %q: %s (max %d outbound, max %d inbound listen)", p2pSettings.Mode, p2pSettings.Description, p2pSettings.MaxOutbound, p2pSettings.MaxInbound))
	}

	if p2pSettings.MaxOutbound > 1 || p2pSettings.Listen {
		peerMgr = NewPeerMgr(p2pSettings, p, subVer, d)
		peerMgr.SetAddrStorePath(learnedAddrsPath)
		if bootstrapAddrBook != nil {
			peerMgr.SetAddrBook(bootstrapAddrBook)
		}
		peerMgr.SetBanChecker(banMgr.IsBanned)
		if blockPeerScorer != nil {
			peerMgr.SetBlockPeerScorer(blockPeerScorer)
		}
		if discoveryFeed != nil {
			peerMgr.SetDiscoveryFeed(discoveryFeed)
		}
		peerMgr.SetPreferredPeers(addedNodes.List())
		addedNodes.SeedPeerMgr(peerMgr)
		defer func() {
			_ = RunWithTimeout(ShutdownFlushBudget, peerMgr.SaveLearnedAddrsNow)
		}()
		if !soloMode && conn != nil && mw != nil {
			peerMgr.RegisterPrimary(connectedAddr, conn, mw, p2pNetCtr, peerFromHandshake)
		}
		misbehavior.SetPeerMgr(peerMgr)
		blockStore.SetNetworkTimeSource(peerMgr, peerFromHandshake)
		if blockStore != nil {
			ann := BlockAnnounceEnv{PeerMgr: peerMgr, PrimaryCmpctHBTo: primaryCmpctHBTo}
			if !soloMode && mw != nil {
				ann.Primary = mw
			}
			blockStore.SetBlockAnnounce(ann)
			WirePeerMgrBlockCallbacks(blockStore, peerMgr, j, p)
		}
		peerMgr.SetLocalServices(localServices)
		peerMgr.SetRelayEnv(func() RelayEnv {
			env := PeerMgrRelayEnv(cfg.Network, cfg, j, auxJ, chainPolicy, blockStore, pool, orphans, txIx, rbStore, filterIx, peerFeeFilters, tipWait, &rawFill, misbehavior)
			if dgrMgr != nil {
				env.DGRFanIn = func(cmd string, pl []byte) {
					FanInViaDGR(dgrMgr, cmd, pl)
				}
			}
			return env
		}())
		addedNodes.SeedPeerMgr(peerMgr)
		exclude := connectedAddr
		if soloMode {
			exclude = ""
		}
		peerMgr.Start(ctx, discoveredPeers, exclude)
		stopPortMap := StartPortMapping(ctx, cfg.Upnp, p2pSettings.Listen, int(p.Port), peerMgr)
		defer stopPortMap()
		if soloMode {
			applog.Line("net", fmt.Sprintf("solo: multi-peer P2P started (listen=%v, max outbound %d) - peers can connect on port %d", p2pSettings.Listen, p2pSettings.MaxOutbound, p.Port))
		}
	}
	go func() {
		<-ctx.Done()
		applog.Line("net", "shutdown: closing P2P connections to unblock graceful stop")
		if peerMgr != nil {
			peerMgr.CloseAllConnections()
		}
		if conn != nil {
			_ = conn.Close()
		}
	}()
	var startHeaderBackgroundRecovery func() bool
	startHeaderBackgroundRecovery = func() bool {
		return StartHeaderSyncBackgroundRecoveryOnce(HeaderSyncRecoveryEnv{
			Ctx: ctx, Dialer: d, Params: p, SubVer: subVer, LocalServices: localServices,
			Journal: j, Aux: auxJ, BlockStore: blockStore, FeeFilters: peerFeeFilters,
			RawBackfill: cfg.RawBlockBackfill, RawFill: &rawFill, DiscoveryFeed: discoveryFeed,
			Discovered: discoveredPeers, Scorer: blockPeerScorer, AddrBook: activeAddrBook(peerMgr, bootstrapAddrBook),
			AddedNodes: addedNodes.List(), RefreshDiscovery: refreshPeerDiscovery,
			OnSuccess: func(peer headerSyncPeer) {
				select {
				case headerAttachCh <- peer:
				default:
					closeHeaderSyncPeer(peer)
				}
			},
			OnExhausted: func(lastErr error) {
				if lastErr == nil || !headerCatchUpPending.Load() || !shouldAutoRecoverHeaderSync(lastErr) {
					return
				}
				applog.Line("headers", "background header sync scheduling retry in 30s")
				time.AfterFunc(30*time.Second, func() {
					if headerCatchUpPending.Load() {
						startHeaderBackgroundRecovery()
					}
				})
			},
		})
	}
	kickHeaderSyncRecovery := func(force bool) bool {
		if blockStore != nil && ShouldPauseHeaderCatchUpForBodyIBD(blockStore, 0) {
			reconcileHeaderCatchUpPending(blockStore, &headerCatchUpPending, &rawFill)
			return false
		}
		if !force && ShouldDeferBackgroundHeaderSync() {
			return false
		}
		if !force {
			last := headerSyncLastFailure()
			if !headerCatchUpPending.Load() && (last == nil || !shouldAutoRecoverHeaderSync(last)) {
				return false
			}
		}
		headerCatchUpPending.Store(true)
		if force {
			setHeaderSyncRecoveryHint("Header journal rewound; header sync retrying in background while block download continues.")
		} else if last := headerSyncLastFailure(); last != nil {
			noteHeaderSyncFailure(last)
		}
		if !startHeaderBackgroundRecovery() {
			NoteHeaderRecoveryKickSuppressed()
			syncActivity.mu.Lock()
			shouldLog := time.Since(syncActivity.lastRecoveryKickLog) >= 2*time.Minute
			shouldForceRestart := syncActivity.recoveryKickSuppressed >= 2 &&
				time.Since(syncActivity.lastRecoveryForceLog) >= 2*time.Minute
			if shouldLog {
				syncActivity.lastRecoveryKickLog = time.Now()
			}
			if shouldForceRestart {
				syncActivity.lastRecoveryForceLog = time.Now()
			}
			syncActivity.mu.Unlock()
			if shouldLog {
				applog.Line("headers", "background header sync already running (watchdog will retry when the current pass finishes)")
			}
			if shouldForceRestart && ForceRestartHeaderSyncBackgroundRecovery() {
				applog.Line("headers", "background header sync appears stuck; forcing restart and peer reprobe (Core-style rotation)")
				ClearHeaderRecoveryRuntime()
				RestartHeaderSyncBackgroundRecoverySoon(func() bool {
					if !headerCatchUpPending.Load() {
						return true
					}
					return startHeaderBackgroundRecovery()
				})
			}
		}
		select {
		case headerRecoverKickCh <- struct{}{}:
		default:
		}
		return true
	}
	afterHeaderJournalRewind = func(res HeaderJournalRecoveryResult) {
		if !res.Rewound {
			return
		}
		if kickHeaderSyncRecovery(true) {
			applog.Line("headers", fmt.Sprintf("header journal rewound %d → %d; background header sync and block-assist restarted", res.TipBefore, res.TipAfter))
		}
	}
	onChainTruncatedAfter = func(keepThrough int64) {
		_ = keepThrough
		if kickHeaderSyncRecovery(true) {
			applog.Line("headers", "chain truncated - background header sync and block-assist restarted")
		}
		if cfg.FullNode && rbStore != nil && blockStore != nil {
			if assistCandidates == nil {
				assistCandidates = seedBlockAssistCandidates(ctx, p, blockStore, blockPeerScorer, discoveryFeed, discoveredPeers)
			}
			EnsureBlockAssistWorkers(blockAssistLaunch())
		}
	}
	maybeKickPostAuxEraHeaderRecovery(p.Net, j, kickHeaderSyncRecovery)
	StartHeaderAdvanceWatchdog(ctx, p.Net, j, func() int32 {
		return NetworkPeerStartHeight(peerFromHandshake, peerMgr)
	}, func(j *store.HeaderJournal, peerH int32) bool {
		return ShouldRunHeaderAdvanceWatchdog(j, blockStore, peerH)
	}, func() {
		peerH := NetworkPeerStartHeight(peerFromHandshake, peerMgr)
		tipBefore, _ := j.TipHeight()
		if reset, _ := MaybeResetStuckAncientHeaderChain(j, auxJ, p, blockStore, peerH); reset {
			tipAfter, _ := j.TipHeight()
			afterHeaderJournalRewind(HeaderJournalRecoveryResult{
				Rewound: true, TipBefore: tipBefore, TipAfter: tipAfter,
			})
			return
		}
		kickHeaderSyncRecovery(false)
	})
	if headerCatchUpPending.Load() {
		if cfg.FullNode && rbStore != nil && blockStore != nil {
			rawFill.PrepareAtStartup(blockStore)
			if assistCandidates == nil {
				assistCandidates = seedBlockAssistCandidates(ctx, p, blockStore, blockPeerScorer, discoveryFeed, discoveredPeers)
			}
			EnsureBlockAssistWorkers(blockAssistLaunch())
			if dedicatedHeaderRunning() {
				applog.Line("block", "header catch-up on dedicated peer - block-assist downloading bodies in parallel (Core-style IBD)")
			} else {
				kickHeaderSyncRecovery(false)
				applog.Line("block", "header catch-up pending - block-assist active; background header sync when dedicated peer unavailable")
			}
		} else if !dedicatedHeaderRunning() {
			kickHeaderSyncRecovery(false)
		}
	} else if !soloMode && cfg.FullNode && blockStore != nil && mw != nil {
		blockStore.SetBlockAnnounce(BlockAnnounceEnv{Primary: mw})
		if peerMgr == nil {
			WireSoloPrimaryBlockPeerStats(blockStore, connectedAddr, func() { primaryLastBlock = time.Now() })
		}
	}
	if !soloMode && !headerCatchUpPending.Load() && cfg.FullNode && BodiesBehindHeaders(blockStore) && mw != nil {
		RequestGetAddrFromPeers(mw, peerMgr)
	}

	tipH, _ := j.TipHeight()
	NotifyRPCTip(j, rbStore, utxoCache, tipWait)
	if headerCatchUpPending.Load() {
		if dedicatedHeaderRunning() {
			applog.Line("headers", fmt.Sprintf("header IBD on dedicated peer; local tip height %d", tipH))
		} else if headerRecoveryRunning() {
			applog.Line("headers", fmt.Sprintf("header catch-up pending; local tip height %d (background header sync active)", tipH))
		} else {
			applog.Line("headers", fmt.Sprintf("header catch-up pending; local tip height %d", tipH))
		}
	} else {
		applog.Line("headers", fmt.Sprintf("header sync finished; tip height %d", tipH))
	}
	p2pStableSince := time.Now()
	chainName := "testnet"
	if cfg.Network == chain.MainnetDogecoin {
		chainName = "main"
	}
	if dgrMgr != nil {
		dgrMgr.SetP2PBridge(NewDGRBridge(&DGRBridgeEnv{
			Ctx:                    ctx,
			Params:                 p,
			Network:                cfg.Network,
			MW:                     &mw,
			PeerMgr:                &peerMgr,
			Pool:                   pool,
			Orphans:                orphans,
			TxIndex:                txIx,
			RawBlocks:              rbStore,
			Journal:                j,
			Aux:                    auxJ,
			BlockStore:             blockStore,
			Misbehavior:            misbehavior,
			AllowUnverifiedMempool: cfg.AllowUnverifiedMempool,
			FullRBF:                cfg.FullRBF,
			Standard:               cfg.Standard,
			MempoolLimits:          cfg.MempoolLimits,
			PeerFeeFilters:         peerFeeFilters,
			ConnectedAddr:          &connectedAddr,
		}))
	}
	makeWalletRelay := func() func([]byte) error {
		return func(raw []byte) error {
			if mw == nil {
				return nil
			}
			err := RelayTxToPeer(mw, raw, peerFeeFilters.For(connectedAddr), pool, txIx, rbStore)
			if dgrMgr != nil && dgrMgr.UsingRelay() {
				PublishTxViaDGR(dgrMgr, raw)
			}
			if peerMgr != nil {
				peerMgr.BroadcastTx(raw, "", pool, txIx, rbStore)
			}
			if soloMiningOn.Load() {
				select {
				case soloMineKick <- struct{}{}:
				default:
				}
			}
			return err
		}
	}
	wireWalletAPIBridge := func(paths *rpc.DataPaths) {
		if disk == nil || paths == nil {
			return
		}
		cn := chainName
		if walletTxsBridge != nil {
			walletTxsBridge.Set(func() []interface{} {
				return rpc.WalletListTransactions(cn, paths, j, rbStore, pool, txIx)
			})
			walletTxsBridge.SetPage(func(offset, limit int, q, kind string) ui.WalletTxPageResult {
				page := rpc.WalletListTransactionsPage(cn, paths, j, rbStore, pool, txIx, offset, limit, q, kind)
				return ui.WalletTxPageResult{
					Total:  page.Total,
					Offset: page.Offset,
					Limit:  page.Limit,
					Items:  page.Items,
				}
			})
		}
		if walletSendBridge == nil {
			return
		}
		relay := makeWalletRelay()
		allow := cfg.AllowUnverifiedMempool
		net := cfg.Network
		walletSendBridge.Set(func(dest string, amount float64, fundOpts map[string]interface{}) (string, int, string) {
			return rpc.WalletSendDOGE(cn, paths, j, pool, txIx, rbStore, dest, amount, relay, allow, net, fundOpts)
		})
		walletSendBridge.SetDetailed(func(dest string, amount float64, fundOpts map[string]interface{}) (ui.WalletSendDetailed, int, string) {
			txid, hexStr, status, berr, code, msg := rpc.WalletSendDOGEDetailed(cn, paths, j, pool, txIx, rbStore, dest, amount, relay, allow, net, fundOpts)
			if code != 0 {
				return ui.WalletSendDetailed{}, code, msg
			}
			return ui.WalletSendDetailed{
				Txid:           txid,
				Hex:            hexStr,
				Status:         status,
				BroadcastError: berr,
			}, 0, ""
		})
	}
	// Full node always wires chain RPC paths (web Console, header recovery, wallet) even when HTTP RPC listen is disabled.
	if chainRPCPathsNeeded(cfg) {
		miningAddr := strings.TrimSpace(cfg.MiningAddress)
		if disk != nil {
			miningAddr = disk.Address()
		}
		ensureExtensionManager(cfg, &extMgr, chainDataAbs, j, rbStore, txIx, utxoCache)
		if extMgr != nil {
			extMgr.SetP2PBroadcast(func(cmd string, payload []byte, excludePeer, protocolID string) {
				if peerMgr == nil || extMgr == nil {
					return
				}
				peerMgr.BroadcastCmdFiltered(cmd, payload, excludePeer, func(addr string) bool {
					for _, p := range extMgr.PeerEnabledProtocols(addr) {
						if p == protocolID {
							return true
						}
					}
					return false
				})
			})
			if peerMgr != nil {
				peerMgr.SetExtensionManager(extMgr)
			}
		}
		paths := &rpc.DataPaths{
			BaseDataDir:  baseDataAbs,
			ChainDataDir: chainDataAbs,
			MaxTipAgeSec: chain.EffectiveMaxTipAge(cfg.MaxTipAge),
			HeaderAux:    auxJ,
			Utxo:         utxoCache,
			SyncUtxo: func() error {
				if blockStore != nil {
					return blockStore.SyncUtxoCacheBounded(rpcSyncUtxoMaxBlocks)
				}
				return nil
			},
			SyncUtxoBounded: func(maxBlocks int) error {
				if blockStore != nil {
					return blockStore.SyncUtxoCacheBounded(maxBlocks)
				}
				return nil
			},
			UtxoConnectInFlight:      UtxoConnectInFlight,
			FilterIndexThrough:       FilterIndexThrough,
			TipWaiter:                tipWait,
			MiningAddress:            miningAddr,
			EmbeddedAnalyticsSidecar: analyticsOn,
			FeeFilter:                peerFeeFilters.Load,
			OrphanCount: func() int {
				if orphans == nil {
					return 0
				}
				return orphans.Count()
			},
			MaxMempoolEntries:  pool.MaxMempoolLimitBytes(),
			MempoolExpiryHours: func() int { return pool.ExpiryHours() },
			FullRBF:            func() bool { return cfg.FullRBF },
			Standard: func() consensus.StandardPolicy {
				return cfg.Standard
			},
			MempoolLimits: func() consensus.MempoolRelayLimits {
				return cfg.MempoolLimits
			},
			MempoolMinRelayFee: func() uint64 {
				return pool.MinRelayFeePerKB()
			},
			MempoolFeeEstimate: func(nblocks int) uint64 {
				adm := consensus.NewMempoolAdmissionWithUtxo(pool, pool, utxoCache, txIx, rbStore, j, cfg.Network)
				return consensus.EstimateMempoolFeePerKB(pool.RawBlobs(), adm.View, nblocks)
			},
			MempoolFeeEstimateConservative: func(nblocks int) uint64 {
				_ = nblocks
				adm := consensus.NewMempoolAdmissionWithUtxo(pool, pool, utxoCache, txIx, rbStore, j, cfg.Network)
				return consensus.EstimateMempoolFeePerKBConservative(pool.RawBlobs(), adm.View)
			},
			MempoolFeeEstimateEconomical: func(nblocks int) uint64 {
				adm := consensus.NewMempoolAdmissionWithUtxo(pool, pool, utxoCache, txIx, rbStore, j, cfg.Network)
				return consensus.EstimateMempoolFeePerKBEconomical(pool.RawBlobs(), adm.View, nblocks)
			},
			ConfirmedFeeEstimate: func(nblocks int) uint64 {
				return feeHistory.EstimatePerKBEconomical(nblocks)
			},
			ConfirmedFeeEstimateConservative: func(nblocks int) uint64 {
				return feeHistory.EstimatePerKBConservative(nblocks)
			},
			FeeBucketEstimates: func() map[string]float64 {
				return feeHistory.BucketEstimatesDOGE()
			},
			FeeBucketMarketStats: func() map[string]map[string]interface{} {
				return feeHistory.BucketMarketStats()
			},
			MempoolConfirmBucketStats: func() map[string]map[string]interface{} {
				return feeHistory.MempoolConfirmBucketStats()
			},
			MempoolLeftBucketStats: func() map[string]map[string]interface{} {
				return feeHistory.MempoolLeftBucketStats()
			},
			FeeConfirmStatsBucketMarket: func() map[string]map[string]interface{} {
				return feeHistory.ConfirmStatsBucketMarket()
			},
			FeeHistory:          feeHistory,
			ContiguousRawHeight: contiguousForUI,
			UtxoBodiesAligned: func() bool {
				return BodiesAlignedForUtxoSnapshot(blockStore, utxoCache)
			},
			CumulativeChainWork: func(through int64) (*big.Int, bool) {
				return chainWorkCache.LookupThrough(j, through)
			},
			ChainWorkCacheReady:      chainWorkCache.Ready,
			HeaderSyncRecoveryHint:   headerSyncRecoveryHintStr,
			HeaderCatchUpPending:     func() bool { return headerCatchUpPending.Load() },
			BlockAssistWorkersActive: BlockAssistWorkersActive,
			HeaderTipHeight: func() int64 {
				return ChainActiveHeight(j, rbStore, utxoCache, contiguousForUI)
			},
			OrphanPool:       orphans,
			MaxOrphanEntries: orphans.MaxEntries(),
			RPCWhitelist:     rpc.ParseRPCWhitelist(cfg.RpcWhitelist),
			BlockMaxWeight:   cfg.BlockMaxWeight,
			Uptime:           func() int64 { return int64(time.Since(nodeStart).Seconds()) },
			BanManager:       banMgr,
			BlockFilterIndex: filterIx,
			StorageSummary: nativeStorageSummary(chainRoot, rbStore, txIx, func() int64 {
				if blockStore != nil {
					return blockStore.ContiguousRawHeight()
				}
				return -1
			}),
			ZmqNotifications: func() []map[string]interface{} {
				if zmqHub == nil {
					return nil
				}
				return zmqCfg.ActiveNotifications()
			},
			Extensions: extMgr,
		}
		chainRPCPaths = paths
		paths.CoreRPCAddr = strings.TrimSpace(cfg.CoreRPCAddr)
		paths.CoreRPCUser = strings.TrimSpace(cfg.CoreRPCUser)
		paths.CoreRPCPassword = cfg.CoreRPCPassword
		paths.SignerCommand = signer.ParseCommandLine(cfg.SignerCmd)
		if disk != nil {
			wifVer := p.PrivKeyWIFVersion
			wireWalletHD(paths, disk, wifVer, p.PubkeyHashAddrID, p.ScriptHashAddrID)
			wireWalletEncryption(paths, disk)
			wireWalletRescan(paths, disk, j, rbStore, p.PubkeyHashAddrID, p.ScriptHashAddrID)
			wireWalletLiveIndex(blockStore, paths, disk, j, rbStore, p.PubkeyHashAddrID, p.ScriptHashAddrID)
			wireWalletUtxoCacheOnAdvance(blockStore, paths, disk)
			wireWalletUtxoCacheWarm(paths, disk)
			StartWalletCatchUpRescan(ctx, paths, j, rbStore, disk)
			paths.WalletWIF = func() string {
				w, err := disk.WIFExport(wifVer)
				if err != nil {
					return ""
				}
				return w
			}
			paths.WalletWIFs = func() []string {
				w, err := disk.AllWIFs(wifVer)
				if err != nil {
					return nil
				}
				return w
			}
			paths.WalletP2PKHScript = func() []byte { return disk.P2PKHScript() }
			addrVer := p.PubkeyHashAddrID
			paths.WalletImportPrivKey = func(wif string) error {
				return disk.ImportPrivKey(wif, wifVer, addrVer)
			}
			paths.WalletImportSpendKey = func(wif string) error {
				return disk.ImportSpendPrivKey(wif, wifVer, addrVer)
			}
			paths.WalletImportWatch = func(script []byte) error { return disk.AddWatchScript(script) }
			paths.WalletSetWatchRedeem = func(spk, redeem []byte) error { return disk.SetWatchRedeem(spk, redeem) }
			paths.WalletWatchRedeemScript = func(spk []byte) []byte { return disk.WatchRedeemScript(spk) }
			paths.WalletWatchScripts = func() [][]byte { return disk.WatchScripts() }
			paths.WalletIsWatchAddress = func(addr string) bool {
				return disk.IsWatchAddress(addr, addrVer, p.ScriptHashAddrID)
			}
			paths.WalletPath = func() string { return disk.Path() }
			paths.WalletPayTxFee = func() float64 { return disk.PayTxFee() }
			paths.WalletSetPayTxFee = func(f float64) error { return disk.SetPayTxFee(f) }
			paths.WalletListLocked = func() []wallet.LockedOutpoint { return disk.ListLockedOutpoints() }
			paths.WalletSetLocked = func(unlock bool, outs []wallet.LockedOutpoint) error {
				return disk.SetLockedOutpoints(unlock, outs)
			}
			paths.WalletIsLockedOutpoint = func(txid string, vout uint32) bool {
				return disk.IsLockedOutpoint(txid, vout)
			}
			paths.WalletGetLabel = func(addr string) string { return disk.Label(addr) }
			paths.WalletSetLabel = func(addr, label string) error { return disk.SetLabel(addr, label) }
			paths.WalletListLabels = func() []string { return disk.ListLabels() }
			paths.WalletRecordTxReplacement = func(oldTxid, newTxid string) error {
				return disk.RecordTxReplacement(oldTxid, newTxid)
			}
			paths.WalletConflictsForTx = func(txid string) []string { return disk.ConflictsForTx(txid) }
			paths.WalletAbandonTx = func(txid, category, addr string, amountKoinu int64) error {
				return disk.AbandonTx(wallet.AbandonedTx{
					TxID: txid, Category: category, Address: addr, AmountKoinu: amountKoinu,
				})
			}
			paths.WalletListAbandoned = func() []wallet.AbandonedTx { return disk.ListAbandoned() }
			paths.WalletIsAbandoned = func(txid string) bool { return disk.IsAbandoned(txid) }
			paths.WalletRemoveAbandoned = func(txid string) bool { return disk.RemoveAbandoned(txid) }
			paths.WalletRemoveReplacementsForTx = func(txid string) error { return disk.RemoveReplacementsForTx(txid) }
			wireWalletPrunedImports(paths, disk)
		}
		if cfg.FullNode {
			paths.RawSyncProgress = func() map[string]interface{} {
				snap := rawFill.snapshot()
				enrichIBDProgressSnapshot(snap, j, blockStore)
				enrichAssistDiagnosticsAuto(snap)
				if tip, err := j.TipHeight(); err == nil && tip >= 0 {
					snap["headers_tip"] = tip
				}
				return snap
			}
			paths.RelayBlock = func(payload []byte) error {
				if len(payload) < 80 {
					return nil
				}
				if dgrMgr != nil && dgrMgr.UsingRelay() {
					PublishViaDGR(dgrMgr, "block", payload)
				}
				if blockStore != nil {
					blockStore.AnnounceConnectedBlock(payload)
					return nil
				}
				if mw != nil {
					return RelayBlockToPeer(mw, payload)
				}
				if peerMgr != nil {
					peerMgr.BroadcastCmd("block", payload, "")
				}
				return nil
			}
			if txIx != nil {
				paths.ConnectSubmittedBlock = func(payload []byte, height int64) error {
					return blockStore.tryConnectBlockPayloadRaw(payload, height)
				}
			}
		}
		paths.InvalidateBlock = func(hash string) error {
			if err := InvalidateBlock(j, auxJ, chainPolicy, blockStore, hash); err != nil {
				return err
			}
			NotifyRPCTip(j, rbStore, utxoCache, tipWait)
			return nil
		}
		paths.ReconsiderBlock = func(hash string) error {
			return ReconsiderBlock(chainPolicy, hash)
		}
		recoverHeaderJournalUI = func() (HeaderJournalRecoveryResult, error) {
			res, err := RecoverHeaderJournal(j, auxJ, p, blockStore)
			if err == nil {
				afterHeaderJournalRewind(res)
				return res, nil
			}
			if strings.Contains(err.Error(), "no header journal change") && kickHeaderSyncRecovery(false) {
				return res, nil
			}
			return res, err
		}
		paths.RestartHeaderSyncIfStuck = func() bool {
			return kickHeaderSyncRecovery(false)
		}
		paths.RecoverHeaderJournal = func() (int64, int64, bool, error) {
			res, err := recoverHeaderJournalUI()
			return res.TipBefore, res.TipAfter, res.Rewound, err
		}
		paths.TruncateToHeight = func(height int64) error {
			return TruncateChainToHeight(j, auxJ, blockStore, height)
		}
		paths.MarkPreciousBlock = func(hash string) error {
			return MarkPreciousBlock(j, chainPolicy, hash)
		}
		var p2pNetworkActive atomic.Bool
		p2pNetworkActive.Store(true)
		paths.NetworkActive = func() bool { return p2pNetworkActive.Load() }
		paths.SetNetworkActive = func(active bool) (bool, error) {
			p2pNetworkActive.Store(active)
			if !active && peerMgr != nil {
				peerMgr.DisconnectAllRelays()
			}
			return p2pNetworkActive.Load(), nil
		}
		paths.SetMaxConnections = func(max int) error {
			if peerMgr != nil {
				if err := peerMgr.SetMaxConnections(max); err != nil {
					return err
				}
				cfg.EffectiveFile.MaxOutbound = max
				if err := PersistMaxOutbound(cfg.ConfSavePath, &cfg.EffectiveFile, max); err != nil {
					applog.Line("config", "setmaxconnections save: "+err.Error())
				}
				return nil
			}
			if max < 8 {
				return fmt.Errorf("maxconnectioncount must be at least 8")
			}
			p2pSettings.MaxOutbound = max
			if max > 32 {
				p2pSettings.MaxOutbound = 32
			}
			cfg.EffectiveFile.MaxOutbound = p2pSettings.MaxOutbound
			if err := PersistMaxOutbound(cfg.ConfSavePath, &cfg.EffectiveFile, p2pSettings.MaxOutbound); err != nil {
				applog.Line("config", "setmaxconnections save: "+err.Error())
			}
			return nil
		}
		paths.P2PStats = func() map[string]any {
			cont := int64(-1)
			chainActive := int64(-1)
			if blockStore != nil {
				cont = blockStore.ContiguousRawHeight()
				chainActive = ChainActiveHeight(j, rbStore, utxoCache, blockStore.ContiguousRawHeight)
			}
			extras := P2PExtrasFromNode(assistRegistry, blockPeerScorer, chainActive, cont, rawFill.syncWorkerCount(), dedicatedHeaderRunning(), DedicatedHeaderPeerAddr())
			ibdProg := rawFill.snapshot()
			enrichIBDProgressSnapshot(ibdProg, j, blockStore)
			extras.IBDProgress = IBDProgressWithDiscoveryFeed(ibdProg, assistCandidates, discoveryFeed)
			ibdSnap := coreIBDSnap()
			mergeCoreIBDIntoProgress(extras.IBDProgress, ibdSnap)
			out := BuildP2PUISnapshot(p2pSettings, peerMgr, connectedAddr, peerSlot, extras)
			out["chain_active_height"] = ibdSnap.Blocks
			out["initialblockdownload"] = ibdSnap.IBD
			out["verification_progress"] = ibdSnap.VerificationProgress
			return out
		}
		paths.MedianPeerTimeOffset = func() int32 {
			if peerMgr != nil {
				return peerMgr.MedianTimeOffset()
			}
			if peerFromHandshake != nil {
				return wire.TimeOffsetSeconds(peerFromHandshake, time.Now().Unix())
			}
			return 0
		}
		paths.AddNode = func(node, command string) error {
			norm, err := NormalizeNodeAddr(node, int(p.Port))
			if err != nil {
				return err
			}
			switch command {
			case "add":
				addedNodes.Add(norm)
				if peerMgr != nil {
					peerMgr.NoteAddr(norm)
					peerMgr.SetPreferredPeers(addedNodes.List())
				}
				if err := PersistAddedNodes(cfg.ConfSavePath, &cfg.EffectiveFile, addedNodes); err != nil {
					applog.Line("config", "addnode save: "+err.Error())
				}
				// Apply on the fly: dial now when multi-peer P2P is up (Core addnode add is preferred for later slots).
				if peerMgr != nil {
					connected := connectedAddr != "" && addnodeMatchesSession(norm, connectedAddr)
					if !connected {
						connected = peerMgr.HasSession(norm) || len(peerMgr.ConnectedAddressesForAddedNode(norm)) > 0
					}
					if !connected {
						go func(addr string) {
							if err := peerMgr.DialOnce(ctx, addr); err != nil {
								applog.Line("net", "addnode dial "+addr+": "+err.Error())
							}
						}(norm)
					}
				}
				return nil
			case "remove":
				addedNodes.Remove(norm)
				if peerMgr != nil {
					peerMgr.SetPreferredPeers(addedNodes.List())
					// Drop matching non-primary sessions so remove applies without restart.
					for _, row := range peerMgr.ConnectedAddressesForAddedNode(norm) {
						m, ok := row.(map[string]interface{})
						if !ok {
							continue
						}
						addr, _ := m["address"].(string)
						if addr == "" {
							continue
						}
						if err := peerMgr.DisconnectPeer(addr); err != nil {
							applog.Line("net", "addnode remove disconnect "+addr+": "+err.Error())
						}
					}
				}
				if err := PersistAddedNodes(cfg.ConfSavePath, &cfg.EffectiveFile, addedNodes); err != nil {
					applog.Line("config", "addnode save: "+err.Error())
				}
				return nil
			case "onetry":
				if peerMgr != nil {
					return peerMgr.DialOnce(ctx, norm)
				}
				return fmt.Errorf("onetry requires multi-peer P2P (set p2p_connectivity to cgnat or both)")
			default:
				return fmt.Errorf("unknown addnode command")
			}
		}
		paths.BanDisconnect = func() {
			if peerMgr != nil {
				peerMgr.DisconnectBanned(banMgr.IsBanned)
			}
		}
		paths.AddedNodes = func() []string { return addedNodes.List() }
		paths.NodeAddresses = func(count int, network string) []map[string]interface{} {
			if peerMgr != nil {
				return peerMgr.NodeAddressRows(count, network)
			}
			return nil
		}
		paths.AddrManInfo = func() map[string]interface{} {
			if peerMgr != nil {
				return peerMgr.AddrManInfo()
			}
			return map[string]interface{}{
				"all": map[string]interface{}{"total": 0, "new": 0, "tried": 0},
			}
		}
		paths.IsPeerConnected = func(addr string) bool {
			if addr == "" {
				return false
			}
			if connectedAddr != "" && addnodeMatchesSession(addr, connectedAddr) {
				return true
			}
			if peerMgr != nil {
				return peerMgr.HasSession(addr) || len(peerMgr.ConnectedAddressesForAddedNode(addr)) > 0
			}
			return false
		}
		paths.PeerAddresses = func(added string) []interface{} {
			if added == "" {
				return nil
			}
			if connectedAddr != "" && addnodeMatchesSession(added, connectedAddr) {
				return []interface{}{
					map[string]interface{}{
						"address":   connectedAddr,
						"connected": "outbound",
					},
				}
			}
			if peerMgr != nil {
				return peerMgr.ConnectedAddressesForAddedNode(added)
			}
			return nil
		}
		paths.DisconnectNode = func(addr string) error {
			norm, err := NormalizeNodeAddr(addr, int(p.Port))
			if err != nil {
				return err
			}
			if peerMgr != nil {
				return peerMgr.DisconnectPeer(norm)
			}
			return fmt.Errorf("disconnectnode requires multi-peer P2P (set p2p_connectivity to cgnat or both)")
		}
		paths.NetRecv = func() int64 {
			var r int64
			if peerMgr != nil {
				r, _ = peerMgr.TotalNetBytes()
			} else if p2pNetCtr != nil {
				r = int64(p2pNetCtr.Recv())
			}
			if assistRegistry != nil {
				ar, _ := assistRegistry.NetBytes()
				r += ar
			}
			return r
		}
		paths.NetSent = func() int64 {
			var s int64
			if peerMgr != nil {
				_, s = peerMgr.TotalNetBytes()
			} else if p2pNetCtr != nil {
				s = int64(p2pNetCtr.Sent())
			}
			if assistRegistry != nil {
				_, as := assistRegistry.NetBytes()
				s += as
			}
			return s
		}
		if cfg.Stop != nil {
			stopFn := cfg.Stop
			paths.Shutdown = func() {
				// Flush happens in Run defers with timeouts; do not block cancel on disk I/O here.
				stopFn()
			}
		}
		paths.LocalP2P = func() (int32, string, uint64) {
			return p.ProtocolVersion, subVer, localServices
		}
		paths.ConnectionCount = func() int {
			if paths.NetworkActive != nil && !paths.NetworkActive() {
				return 0
			}
			if paths.P2PStats != nil {
				if snap := paths.P2PStats(); snap != nil {
					if v, ok := snap["connections_total"].(int); ok {
						return v
					}
				}
			}
			if peerMgr != nil {
				return peerMgr.SessionCount()
			}
			if mw != nil {
				return 1
			}
			return 0
		}
		paths.PingPeers = func() {
			if peerMgr != nil {
				peerMgr.PingAll()
				return
			}
			if mw != nil {
				primaryPing.forcePing(mw)
			}
		}
		paths.PeerInfo = func() []map[string]interface{} {
			pctx := &PeerInfoContext{
				Scorer:      blockPeerScorer,
				Assist:      assistRegistry,
				AddedNodes:  addedNodes,
				RawFill:     &rawFill,
				PrimaryAddr: connectedAddr,
				IBDLanes:    rawFill.syncWorkerCount(),
				Misbehavior: misbehavior,
			}
			if blockStore != nil {
				pctx.SyncedBlocks = ChainActiveHeight(j, rbStore, utxoCache, blockStore.ContiguousRawHeight)
			}
			if peerMgr != nil {
				return peerMgr.PeerInfoMaps(j, p, pctx)
			}
			tipH, _ := j.TipHeight()
			syncedBlocks := int64(tipH)
			if pctx.SyncedBlocks >= 0 {
				syncedBlocks = pctx.SyncedBlocks
			}
			now := time.Now().Unix()
			var recv, sent int64
			if p2pNetCtr != nil {
				recv = int64(p2pNetCtr.Recv())
				sent = int64(p2pNetCtr.Sent())
			}
			proto := p.ProtocolVersion
			sub := subVer
			svHex := rpc.FormatServicesHex(localServices)
			startH := int64(0)
			note := "single outbound TCP; set p2p_connectivity=both or cgnat for multi-peer relay"
			if peerFromHandshake != nil {
				proto = peerFromHandshake.ProtocolVersion
				sub = peerFromHandshake.UserAgent
				svHex = rpc.FormatServicesHex(peerFromHandshake.Services)
				startH = int64(peerFromHandshake.StartHeight)
			}
			timeOff := int32(0)
			if peerFromHandshake != nil {
				timeOff = wire.TimeOffsetSeconds(peerFromHandshake, now)
			}
			row := BuildSoloPrimaryPeerInfoRow(SoloPrimaryPeerInfoOpts{
				Addr: connectedAddr, Conn: conn, ConnTime: p2pStableSince,
				ProtocolVersion: proto, SubVer: sub, ServicesHex: svHex, StartHeight: startH,
				TipH: tipH, SyncedBlocks: syncedBlocks, Sent: sent, Recv: recv, TimeOffset: timeOff,
				Ping: &primaryPing, LastSend: primaryLastSend, LastRecv: primaryLastRecv,
				LastBlock: primaryLastBlock, LastTx: primaryLastTx,
				FeeFilterKoinu: peerFeeFilters.For(connectedAddr), CmpctHBFrom: primaryCmpctHBFrom, CmpctHBTo: primaryCmpctHBTo,
				RelayTxes: relayTxesFromVersion(peerFromHandshake), MsgWriter: mw,
				Misbehavior: misbehavior, Ctx: pctx, Note: note, DGRTunneled: primaryDGRTunneled,
			})
			rows := []map[string]interface{}{row}
			return append(rows, assistPeerInfoRows(pctx, p, j, rows)...)
		}
		wireWalletAPIBridge(paths)
		wireExtensionWalletRPC(extMgr, chainName, j, pool, paths, rbStore, txIx, makeWalletRelay(), cfg.AllowUnverifiedMempool)
		addr := strings.TrimSpace(miningAddr)
		miningRT := NewSoloMiningRuntime(SoloMiningRuntimeConfig{
			Parent:        ctx,
			Active:        &soloMiningOn,
			Kick:          soloMineKick,
			MineRequested: func() bool { return cfg.Mine },
			PayoutAddress: addr,
			RestartNote:   "Settings → Wallet mine checkbox persists after save+restart; start/stop here applies to this run only.",
			Eligible: func() (bool, string) {
				if cfg.NodeMode != "full" && !cfg.FullNode {
					return false, "full node required for background mining"
				}
				if !p.IsRebootTestnet() {
					return false, "background solo mining is reboot testnet only (mainnet: Console generatetoaddress or createauxblock)"
				}
				if strings.TrimSpace(miningAddr) == "" {
					return false, "enable wallet or set miningaddress in config"
				}
				return true, "reboot testnet solo mining available"
			},
		})
		miningRT.Configure(SoloMinerOpts{
			ChainName: chainName, Journal: j, Aux: auxJ, Raw: rbStore, Paths: paths,
			Pool: pool, TxIndex: txIx,
			MiningAddr: addr, BlockStore: blockStore, TipWait: tipWait,
		})
		runtimeSvc.SetMining(miningRT)
		if cfg.Mine && cfg.FullNode && p.IsRebootTestnet() {
			if addr == "" {
				fmt.Fprintln(os.Stderr, "DogeGo: reboot testnet mining: enable the wallet or set miningaddress in dogecoinconf.json")
			} else if err := miningRT.Start(); err != nil && !soloMiningOn.Load() {
				fmt.Fprintln(os.Stderr, "DogeGo: reboot testnet mining: "+err.Error())
			}
		}
		paths.RPCTLSEnabled = cfg.RpcTLS.Enabled()
		relayForUI := makeWalletRelay()
		uiRPCInvoke = func(method string, params []json.RawMessage) map[string]interface{} {
			return rpc.Dispatch(chainName, j, pool, paths, rbStore, txIx, relayForUI, cfg.AllowUnverifiedMempool, method, params, json.RawMessage(`1`))
		}
		if cfg.RPCAddr == "" {
			applog.Line("rpc", "JSON-RPC listen disabled; web Console and wallet RPC still available in-process")
		} else if earlyRPC != nil {
			relay := makeWalletRelay()
			earlyRPC.Activate(rpc.HandlerCore(chainName, j, pool, paths, rbStore, txIx, relay, cfg.AllowUnverifiedMempool))
			runtimeSvc.SetRPCDispatchReady(true)
			applog.Line("rpc", "JSON-RPC dispatch ready on "+cfg.RPCAddr)
		} else if nodeRPCAuth != nil {
			go func() {
				applog.Line("rpc", "starting JSON-RPC on "+cfg.RPCAddr)
				runtimeSvc.SetRPCListening(true)
				runtimeSvc.SetRPCDispatchReady(true)
				defer runtimeSvc.SetRPCListening(false)
				relay := makeWalletRelay()
				if cfg.RpcTLS.Enabled() {
					applog.Line("rpc", "JSON-RPC listening with TLS on "+cfg.RPCAddr)
				}
				if err := rpc.Serve(cfg.RPCAddr, cfg.RpcTLS, chainName, j, pool, paths, rbStore, txIx, relay, cfg.AllowUnverifiedMempool, nodeRPCAuth); err != nil && !errors.Is(err, http.ErrServerClosed) {
					applog.Line("rpc", "JSON-RPC stopped: "+err.Error())
				}
			}()
		}
	} else if disk != nil && (walletSendBridge != nil || walletTxsBridge != nil) {
		paths := &rpc.DataPaths{Utxo: utxoCache}
		wifVer := p.PrivKeyWIFVersion
		wireWalletHD(paths, disk, wifVer, p.PubkeyHashAddrID, p.ScriptHashAddrID)
		wireWalletEncryption(paths, disk)
		wireWalletRescan(paths, disk, j, rbStore, p.PubkeyHashAddrID, p.ScriptHashAddrID)
		StartWalletCatchUpRescan(ctx, paths, j, rbStore, disk)
		paths.WalletWIF = func() string {
			w, err := disk.WIFExport(wifVer)
			if err != nil {
				return ""
			}
			return w
		}
		paths.WalletWIFs = func() []string {
			w, err := disk.AllWIFs(wifVer)
			if err != nil {
				return nil
			}
			return w
		}
		paths.WalletP2PKHScript = func() []byte { return disk.P2PKHScript() }
		addrVer := p.PubkeyHashAddrID
		paths.WalletImportPrivKey = func(wif string) error {
			return disk.ImportPrivKey(wif, wifVer, addrVer)
		}
		paths.WalletImportSpendKey = func(wif string) error {
			return disk.ImportSpendPrivKey(wif, wifVer, addrVer)
		}
		paths.WalletImportWatch = func(script []byte) error { return disk.AddWatchScript(script) }
		paths.WalletSetWatchRedeem = func(spk, redeem []byte) error { return disk.SetWatchRedeem(spk, redeem) }
		paths.WalletWatchRedeemScript = func(spk []byte) []byte { return disk.WatchRedeemScript(spk) }
		paths.WalletWatchScripts = func() [][]byte { return disk.WatchScripts() }
		paths.WalletIsWatchAddress = func(addr string) bool {
			return disk.IsWatchAddress(addr, addrVer, p.ScriptHashAddrID)
		}
		paths.WalletPath = func() string { return disk.Path() }
		paths.WalletPayTxFee = func() float64 { return disk.PayTxFee() }
		paths.WalletSetPayTxFee = func(f float64) error { return disk.SetPayTxFee(f) }
		paths.WalletListLocked = func() []wallet.LockedOutpoint { return disk.ListLockedOutpoints() }
		paths.WalletSetLocked = func(unlock bool, outs []wallet.LockedOutpoint) error {
			return disk.SetLockedOutpoints(unlock, outs)
		}
		paths.WalletIsLockedOutpoint = func(txid string, vout uint32) bool {
			return disk.IsLockedOutpoint(txid, vout)
		}
		paths.WalletGetLabel = func(addr string) string { return disk.Label(addr) }
		paths.WalletSetLabel = func(addr, label string) error { return disk.SetLabel(addr, label) }
		paths.WalletListLabels = func() []string { return disk.ListLabels() }
		wireWalletPrunedImports(paths, disk)
		wireWalletAPIBridge(paths)
		wireExtensionWalletRPC(extMgr, chainName, j, pool, paths, rbStore, txIx, makeWalletRelay(), cfg.AllowUnverifiedMempool)
	}
	if rbStore != nil {
		// Core-style: do not block P2P/RPC wiring on a long startup body/UTXO catch-up pass.
		go func() {
			if err := EnsureLocalGenesis(blockStore); err != nil {
				applog.Line("block", "genesis (chainparams): "+err.Error())
			}
			if NeedsGenesisBlock(blockStore) && mw != nil {
				applog.Line("block", "genesis still missing after chainparams - trying getdata fallback")
				if err := SyncGenesisRawBlock(ctx, mw, p, blockStore); err != nil {
					applog.Line("block", "genesis getdata fallback: "+err.Error())
				}
			}
			if !NeedsGenesisBlock(blockStore) {
				applog.Line("block", "genesis raw block stored")
			}
			cont := blockStore.ContiguousRawHeight()
			tipBF := cfg.RawBlockBackfill
			if ShouldDeferTipBackfill(tipH, cont) {
				applog.Line("block", fmt.Sprintf("deferring tip backfill (headers tip %d, contiguous raw through %d); forward sync first", tipH, cont))
				tipBF = 0
			}
			if tipBF > 0 && mw != nil {
				applog.Line("block", fmt.Sprintf("tip-aligned raw block backfill (max %d heights)", tipBF))
				SyncRecentRawBlocks(ctx, mw, p, blockStore, tipBF)
				if tipBFCoord != nil {
					tipBFCoord.noteStartupRan()
				}
			} else if tipBFCoord != nil {
				applog.Line("block", "tip backfill will run when forward sync is within 512 blocks of header tip")
			}
			applog.Line("block", "startup raw block fetch pass finished")
			blockStore.FlushDeferredConnect()
			if utxoStartupConnectNeeded(blockStore, utxoCache) {
				if err := blockStore.SyncUtxoCache(); err != nil {
					applog.Line("utxo", "startup sync: "+err.Error())
				} else if utxoCache != nil && utxoCache.TipHeight() >= 0 {
					applog.Line("utxo", fmt.Sprintf("UTXO cache through height %d (%d outputs)", utxoCache.TipHeight(), utxoCache.Count()))
				}
			} else if utxoCache != nil && utxoCache.TipHeight() >= 0 {
				applog.Line("utxo", fmt.Sprintf("startup connect skipped: chainActive through %d matches contiguous bodies", utxoCache.TipHeight()))
			}
			if utxoCache != nil && utxoCache.TipHeight() >= 0 {
				if err := PersistUtxoSnapshotIfAligned(blockStore, utxoCache, store.UtxoSnapshotPath(chainRoot), "startup"); err != nil {
					applog.Line("utxo", "snapshot save: "+err.Error())
				}
			}
		}()
	}

	if !soloMode && mw != nil {
		mpSync.maybeRequest(ctx, mw, p, blockStore, &rawFill, peerMgr)
		if err := MaybeBroadcastFeeFilter(mw, pool, &lastSentFeeFilter); err != nil {
			applog.Line("net", "feefilter send: "+err.Error())
		}
		if peerMgr != nil {
			peerMgr.BroadcastFeeFilter(pool)
		}
	}

	pausePrimaryForRecovery := func(lastErr error) {
		headerCatchUpPending.Store(true)
		noteHeaderSyncFailure(lastErr)
		if peerMgr != nil && connectedAddr != "" {
			peerMgr.DropPrimary(connectedAddr)
		}
		if conn != nil {
			_ = conn.Close()
			conn = nil
		}
		mw = nil
		connectedAddr = ""
		peerSlot = "(header catch-up; block-assist active)"
		primaryRedialStreak = 0
		rawFill.ReleaseLaneInFlight(0)
		refreshPeerDiscovery()
		if cfg.FullNode && rbStore != nil && blockStore != nil {
			if assistCandidates == nil {
				assistCandidates = seedBlockAssistCandidates(ctx, p, blockStore, blockPeerScorer, discoveryFeed, discoveredPeers)
			} else {
				RefreshBlockAssistPool(assistCandidates, DiscoverySnapshot(discoveryFeed, discoveredPeers), peerMgr, blockPeerScorer, blockStore, addedNodes.List())
			}
		}
		startHeaderBackgroundRecovery()
		if cfg.FullNode && rbStore != nil && blockStore != nil {
			EnsureBlockAssistWorkers(blockAssistLaunch())
		}
		applog.Line("net", fmt.Sprintf("primary session paused (%v); node and block-assist keep running - headers retry in background", lastErr))
	}

	if cfg.FullNode && filterIx != nil && txIx != nil && rbStore != nil {
		if cont := blockStore.ContiguousRawHeight(); cont >= 0 && BodiesBehindHeaders(blockStore) {
			lastFilterContiguous = cont
			SetFilterIndexThrough(cont)
		}
		startBlockFilterCatchUpWorker(ctx, blockStore, j, rbStore, filterIx, txIx, &lastFilterContiguous)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		select {
		case peer := <-headerAttachCh:
			headerCatchUpPending.Store(false)
			clearHeaderSyncRecoveryHint()
			conn = peer.conn
			mw = peer.mw
			p2pNetCtr = peer.ctr
			connectedAddr = peer.addr
			peerFromHandshake = peer.dv
			peerSlot = peer.addr
			p2pStableSince = time.Now()
			if peerMgr != nil {
				peerMgr.RegisterPrimary(connectedAddr, conn, mw, p2pNetCtr, peerFromHandshake)
				if blockStore != nil {
					blockStore.SetBlockAnnounce(BlockAnnounceEnv{Primary: mw, PeerMgr: peerMgr, PrimaryCmpctHBTo: primaryCmpctHBTo})
				}
			} else if blockStore != nil {
				blockStore.SetBlockAnnounce(BlockAnnounceEnv{Primary: mw})
			}
			blockStore.SetNetworkTimeSource(peerMgr, peerFromHandshake)
			if cfg.FullNode && rbStore != nil {
				primaryExclude.Set(connectedAddr)
				if assistCandidates == nil {
					assistCandidates = seedBlockAssistCandidates(ctx, p, blockStore, blockPeerScorer, discoveryFeed, discoveredPeers)
					StartBlockAssistWorkers(ctx, d, assistCandidates, &primaryExclude, p, subVer, localServices, blockStore, &rawFill, blockSyncWorkers(), blockPeerScorer, assistRegistry, addrBookFromPeerMgr(peerMgr))
				}
				if NeedsGenesisBlock(blockStore) {
					_ = SyncGenesisRawBlock(ctx, mw, p, blockStore)
				}
			}
			NotifyRPCTip(j, rbStore, utxoCache, tipWait)
			MaybePushSPVBloom(spvBloom, mw, peerFromHandshake)
			applog.Line("headers", "background header sync completed - primary peer attached")
			if cfg.FullNode && rbStore != nil && blockStore != nil {
				if assistCandidates != nil {
					RefreshBlockAssistPool(assistCandidates, DiscoverySnapshot(discoveryFeed, discoveredPeers), peerMgr, blockPeerScorer, blockStore, addedNodes.List())
				}
				EnsureBlockAssistWorkers(blockAssistLaunch())
				if BodiesBehindHeaders(blockStore) {
					RequestGetAddrFromPeers(mw, peerMgr)
				}
			}
			if j != nil && !dedicatedHeaderRunning() && (blockStore == nil || !ShouldPauseHeaderCatchUpForBodyIBD(blockStore, 0)) {
				if n, herr := TopUpHeadersRound(ctx, mw, p, j, auxJ, blockStore, &rawFill, discoveryFeed); herr != nil {
					applog.Line("headers", "post-attach header top-up: "+herr.Error())
				} else if n > 0 {
					NotifyRPCTip(j, rbStore, utxoCache, tipWait)
				}
				if peerMgr != nil {
					if sent := peerMgr.RequestHeadersTopUpFromRelays(p, j, maxRelayHeaderTopUpPeers); sent > 0 {
						applog.Line("headers", fmt.Sprintf("post-attach relay header top-up from %d peer(s)", sent))
					}
				}
				blockStore.maybeBackfillAuxAfterHeaderAdvance()
			}
			mpSync.maybeRequest(ctx, mw, p, blockStore, &rawFill, peerMgr)
			_ = MaybeBroadcastFeeFilter(mw, pool, &lastSentFeeFilter)
			if peerMgr != nil {
				peerMgr.BroadcastFeeFilter(pool)
			}
		case <-headerRecoverKickCh:
			if headerCatchUpPending.Load() && (blockStore == nil || !ShouldPauseHeaderCatchUpForBodyIBD(blockStore, 0)) {
				kickHeaderSyncRecovery(false)
			}
			if cfg.FullNode && rbStore != nil && blockStore != nil {
				if assistCandidates == nil {
					assistCandidates = seedBlockAssistCandidates(ctx, p, blockStore, blockPeerScorer, discoveryFeed, discoveredPeers)
				}
				rawFill.PrepareAtStartup(blockStore)
				EnsureBlockAssistWorkers(blockAssistLaunch())
			}
		default:
		}
		if soloMode {
			if cfg.FullNode && blockStore != nil && time.Since(lastAutoRecoverPoll) >= 15*time.Minute {
				lastAutoRecoverPoll = time.Now()
				if rewound, _ := autoRecoverSweep(chainRoot, j, auxJ, p, blockStore, autoFilterRepair); rewound {
					rawFill.ResetAfterChainTruncate(blockStore)
					headerCatchUpPending.Store(true)
					kickHeaderSyncRecovery(true)
				}
			}
			if cfg.FullNode && blockStore != nil && time.Since(lastTxIndexRepairPoll) >= 15*time.Minute {
				lastTxIndexRepairPoll = time.Now()
				maybeRepairTxIndex(chainRoot, blockStore, txIndexRepairMinRawBlocks)
				maybeUpgradeLegacyTxIndex(chainRoot, txIndexLegacyUpgradeBatch)
				maybeRepairBlockFilters(j, chainRoot, txIndexRepairMinRawBlocks, func(hashLE [32]byte, blockRaw []byte) error {
					return rpc.IndexBasicBlockFilter(filterIx, hashLE, blockRaw, j, rbStore, txIx)
				})
			}
			if strings.TrimSpace(cfg.AlertNotify) != "" && j != nil && time.Since(lastAlertNotifyPoll) >= 5*time.Minute {
				lastAlertNotifyPoll = time.Now()
				pollChainAlertNotify(cfg.AlertNotify, false, j, cfg.Network, &alertNotifySt)
			}
			if time.Since(lastSoloAttachTry) >= soloPeerAttachInterval {
				lastSoloAttachTry = time.Now()
				peer, err := TryPromoteSoloPrimary(SoloAttachOpts{
					Ctx: ctx, Dialer: d, Params: p, UserAgent: subVer, LocalServices: localServices,
					Journal: j, Aux: auxJ, BlockStore: blockStore, FeeFilters: peerFeeFilters, RawFill: &rawFill,
					DiscoveryFeed: discoveryFeed, Discovered: discoveredPeers, Scorer: blockPeerScorer,
					AddedNodes: addedNodes, RawBackfill: cfg.RawBlockBackfill,
				})
				if err != nil {
					applog.Line("net", "solo peer attach: "+err.Error())
				} else if peer != nil {
					soloMode = false
					conn = peer.conn
					mw = peer.mw
					p2pNetCtr = peer.ctr
					connectedAddr = peer.addr
					peerFromHandshake = peer.dv
					peerSlot = peer.addr
					p2pStableSince = time.Now()
					if peerMgr != nil {
						peerMgr.RegisterPrimary(connectedAddr, conn, mw, p2pNetCtr, peerFromHandshake)
						if blockStore != nil {
							blockStore.SetBlockAnnounce(BlockAnnounceEnv{Primary: mw, PeerMgr: peerMgr, PrimaryCmpctHBTo: primaryCmpctHBTo})
						}
					}
					if rbStore != nil && mw != nil && NeedsGenesisBlock(blockStore) {
						_ = SyncGenesisRawBlock(ctx, mw, p, blockStore)
					}
					mpSync.maybeRequest(ctx, mw, p, blockStore, &rawFill, peerMgr)
					_ = MaybeBroadcastFeeFilter(mw, pool, &lastSentFeeFilter)
					if peerMgr != nil {
						peerMgr.BroadcastFeeFilter(pool)
					}
					NotifyRPCTip(j, rbStore, utxoCache, tipWait)
					MaybePushSPVBloom(spvBloom, mw, peerFromHandshake)
					applog.Line("net", "left solo founder mode: connected to "+connectedAddr)
				}
			}
			if cfg.FullNode && blockStore != nil && rawFill.bodiesDownloadActive(blockStore) {
				maybeEnsureBlockAssistDuringNoPrimary(ctx, p, blockStore, &rawFill, &assistCandidates, &lastAssistCandRefresh, discoveryFeed, discoveredPeers, peerMgr, blockPeerScorer, addedNodes.List(), blockAssistLaunch, refreshPeerDiscovery)
				MaybeRecoverIBDStall(nil, peerMgr, &rawFill, blockStore, assistCandidates, discoveryFeed, discoveredPeers, blockPeerScorer, addedNodes.List(), &lastIBDStallRecover, blockAssistLaunch, refreshPeerDiscovery, ensureAssistPool)
			}
			if cfg.FullNode && j != nil && blockStore != nil {
				soloPeerStart := NetworkPeerStartHeight(peerFromHandshake, peerMgr)
				MaybeResumeHeaderCatchUpAfterBodyIBD(j, blockStore, soloPeerStart, &bodyIBDHeaderWasPaused, &headerCatchUpPending, &lastHeaderCatchUpResumeKick, kickHeaderSyncRecovery)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}
		if blockStore != nil {
			reconcileHeaderCatchUpPending(blockStore, &headerCatchUpPending, &rawFill)
		}
		if mw == nil {
			if headerCatchUpPending.Load() && (blockStore == nil || !ShouldPauseHeaderCatchUpForBodyIBD(blockStore, 0)) && peerMgr != nil && j != nil && time.Since(lastHeaderTopUp) >= 10*time.Minute {
				lastHeaderTopUp = time.Now()
				if sent := peerMgr.RequestHeadersTopUpFromRelays(p, j, maxRelayHeaderTopUpPeers); sent > 0 {
					applog.Line("headers", fmt.Sprintf("header catch-up: relay header top-up from %d peer(s) (no primary)", sent))
				}
			}
			if headerCatchUpPending.Load() && (blockStore == nil || !ShouldPauseHeaderCatchUpForBodyIBD(blockStore, 0)) && time.Since(lastHeaderDiscoveryPoll) >= 5*time.Minute {
				lastHeaderDiscoveryPoll = time.Now()
				refreshPeerDiscovery()
				ensureAssistPool()
				if assistCandidates != nil {
					RefreshBlockAssistPool(assistCandidates, DiscoverySnapshot(discoveryFeed, discoveredPeers), peerMgr, blockPeerScorer, blockStore, addedNodes.List())
				}
				if cfg.FullNode && rbStore != nil && blockStore != nil {
					EnsureBlockAssistWorkers(blockAssistLaunch())
				}
				kickHeaderSyncRecovery(false)
			}
			if cfg.FullNode && rawFill.bodiesDownloadActive(blockStore) {
				maybeEnsureBlockAssistDuringNoPrimary(ctx, p, blockStore, &rawFill, &assistCandidates, &lastAssistCandRefresh, discoveryFeed, discoveredPeers, peerMgr, blockPeerScorer, addedNodes.List(), blockAssistLaunch, refreshPeerDiscovery)
			}
			if cfg.FullNode && blockStore != nil && rawFill.bodiesDownloadActive(blockStore) {
				MaybeRecoverIBDStall(nil, peerMgr, &rawFill, blockStore, assistCandidates, discoveryFeed, discoveredPeers, blockPeerScorer, addedNodes.List(), &lastIBDStallRecover, blockAssistLaunch, refreshPeerDiscovery, ensureAssistPool)
			}
			if cfg.FullNode && blockStore != nil && utxoCache != nil {
				MaybeSyncConnectCatchUp(blockStore, utxoCache, &lastConnectCatchUpPoll)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}
		if time.Since(lastFeeFilterPoll) >= 10*time.Minute {
			lastFeeFilterPoll = time.Now()
			if err := MaybeBroadcastFeeFilter(mw, pool, &lastSentFeeFilter); err != nil {
				applog.Line("net", "feefilter periodic: "+err.Error())
			}
			if peerMgr != nil {
				peerMgr.BroadcastFeeFilter(pool)
			}
		}
		if strings.TrimSpace(cfg.AlertNotify) != "" && j != nil && time.Since(lastAlertNotifyPoll) >= 5*time.Minute {
			lastAlertNotifyPoll = time.Now()
			skipIBD := cfg.FullNode && rawFill.useShortReadDeadline()
			pollChainAlertNotify(cfg.AlertNotify, skipIBD, j, cfg.Network, &alertNotifySt)
		}
		if cfg.FullNode && txIx != nil && time.Since(lastTxIndexRepairPoll) >= 15*time.Minute {
			lastTxIndexRepairPoll = time.Now()
			maybeRepairTxIndex(chainRoot, blockStore, txIndexRepairMinRawBlocks)
			maybeUpgradeLegacyTxIndex(chainRoot, txIndexLegacyUpgradeBatch)
		}
		if cfg.FullNode && blockStore != nil && time.Since(lastAutoRecoverPoll) >= 15*time.Minute {
			lastAutoRecoverPoll = time.Now()
			if rewound, _ := autoRecoverSweep(chainRoot, j, auxJ, p, blockStore, autoFilterRepair); rewound {
				rawFill.ResetAfterChainTruncate(blockStore)
				headerCatchUpPending.Store(true)
				kickHeaderSyncRecovery(true)
			}
		}
		if cfg.FullNode && rawFill.useShortReadDeadline() && time.Since(lastIBDProgressLog) >= 3*time.Minute {
			lastIBDProgressLog = time.Now()
			rawFill.MaybeLogIBDProgress(j, blockStore)
		}
		if cfg.FullNode && blockStore != nil && utxoCache != nil {
			MaybeSyncConnectCatchUp(blockStore, utxoCache, &lastConnectCatchUpPoll)
		}
		if cfg.FullNode && auxJ != nil && rbStore != nil && rawFill.useShortReadDeadline() && time.Since(lastAuxBackfillPoll) >= 15*time.Minute {
			lastAuxBackfillPoll = time.Now()
			tipH, _ := j.TipHeight()
			through := tipH
			if cont := blockStore.ContiguousRawHeight(); cont >= 0 && tipH-cont > 2048 {
				through = cont + 2048
				applog.Line("headers", fmt.Sprintf("IBD aux backfill: scanning heights 0..%d (header tip %d, bodies through %d)", through, tipH, cont))
			}
			if n, err := store.BackfillAuxThroughHeight(j, auxJ, rbStore, through); err != nil {
				applog.Line("headers", "IBD aux backfill: "+err.Error())
			} else if n > 0 {
				applog.Line("headers", fmt.Sprintf("IBD aux backfill: %d header auxpow record(s) from raw blocks (through %d)", n, through))
			}
		}
		if cfg.FullNode && blockStore != nil && rawFill.bodiesDownloadActive(blockStore) {
			MaybePumpBodyIBDDownload(ctx, mw, p, blockStore, &rawFill, blockPeerScorer, addrBookFromPeerMgr(peerMgr), &lastBodyIBDPump)
			MaybeEnsureBlockAssistWorkers(blockAssistLaunch())
		}
		if cfg.FullNode && blockStore != nil && rawFill.bodiesDownloadActive(blockStore) {
			MaybeRequestGetAddrDuringIBD(mw, peerMgr, true, &lastGetAddrPoll)
			MaybeRecoverIBDStall(mw, peerMgr, &rawFill, blockStore, assistCandidates, discoveryFeed, discoveredPeers, blockPeerScorer, addedNodes.List(), &lastIBDStallRecover, blockAssistLaunch, refreshPeerDiscovery, ensureAssistPool)
		}
		if cfg.FullNode && rawFill.useShortReadDeadline() && assistCandidates == nil {
			assistCandidates = seedBlockAssistCandidates(ctx, p, blockStore, blockPeerScorer, discoveryFeed, discoveredPeers)
		}
		if cfg.FullNode && assistCandidates != nil && rawFill.useShortReadDeadline() && time.Since(lastAssistCandRefresh) >= blockAssistCandidatesRefreshInterval {
			lastAssistCandRefresh = time.Now()
			before := assistCandidates.Len()
			RefreshBlockAssistPool(assistCandidates, DiscoverySnapshot(discoveryFeed, discoveredPeers), peerMgr, blockPeerScorer, blockStore, addedNodes.List())
			if after := assistCandidates.Len(); after != before {
				applog.Line("block", fmt.Sprintf("block-assist peer pool refreshed (%d → %d candidates)", before, after))
				if before == 0 && after > 0 {
					EnsureBlockAssistWorkers(blockAssistLaunch())
				}
			}
		}
		headerTopUpInterval := 10 * time.Minute
		if rawFill.useShortReadDeadline() {
			headerTopUpInterval = headerTopUpDuringIBD
		}
		peerStartH := NetworkPeerStartHeight(peerFromHandshake, peerMgr)
		if cfg.FullNode && j != nil && blockStore != nil {
			MaybeResumeHeaderCatchUpAfterBodyIBD(j, blockStore, peerStartH, &bodyIBDHeaderWasPaused, &headerCatchUpPending, &lastHeaderCatchUpResumeKick, kickHeaderSyncRecovery)
		}
		if cfg.FullNode && j != nil && mw != nil && time.Since(lastHeaderTopUp) >= headerTopUpInterval {
			headersStillBehind := shouldContinueHeaderCatchUpDuringIBD(j, peerStartH)
			tryTopUp := !rawFill.useShortReadDeadline() || headersStillBehind
			if blockStore != nil && ShouldPauseHeaderCatchUpForBodyIBD(blockStore, peerStartH) {
				tryTopUp = false
			}
			if tryTopUp && !dedicatedHeaderRunning() && (blockStore == nil || !ShouldDeferInboundHeaders(blockStore) || headersStillBehind) {
				lastHeaderTopUp = time.Now()
				if n, err := TopUpHeadersRound(ctx, mw, p, j, auxJ, blockStore, &rawFill, discoveryFeed); err != nil {
					applog.Line("headers", "header top-up: "+err.Error())
				} else if n > 0 {
					NotifyRPCTip(j, rbStore, utxoCache, tipWait)
				}
				if peerMgr != nil {
					if sent := peerMgr.RequestHeadersTopUpFromRelays(p, j, maxRelayHeaderTopUpPeers); sent > 0 {
						applog.Line("headers", fmt.Sprintf("header top-up also requested from %d relay peer(s)", sent))
					}
				}
			}
		}
		readTO := 120 * time.Second
		if rbStore != nil && cfg.FullNode && rawFill.bodiesDownloadActive(blockStore) {
			// Interleave block-body catch-up with inbound P2P (single reader on the conn).
			readTO = 4 * time.Second
			if peerMgr != nil {
				peerMgr.MaybePingPrimary(mw)
			} else {
				primaryPing.maybePing(mw)
			}
		}
		_ = mw.Conn().SetReadDeadline(time.Now().Add(readTO))
		cmd, pl, err := wire.ReadMessage(mw.Conn(), p.Magic)
		if err != nil {
			if isNetTimeout(err) && rbStore != nil && cfg.FullNode {
				wantH := blockFetchWantHeight(blockStore)
				stallDisconnect := false
				if NeedsGenesisBlock(blockStore) {
					if gerr := SyncGenesisRawBlock(ctx, mw, p, blockStore); gerr != nil && !IsBenignShutdownErr(gerr) {
						applog.Line("block", "genesis fetch on idle primary: "+gerr.Error())
						if shouldRedialPrimaryForAncientFetch(gerr, 0) && connectedAddr != "" {
							penalizeBlockPeer(blockPeerScorer, addrBookFromPeerMgr(peerMgr), connectedAddr, true)
							err = gerr
							stallDisconnect = true
							applog.Line("block", fmt.Sprintf("primary %s cannot serve genesis - rotating to archival peer", connectedAddr))
						}
					}
				}
				n, ferr := rawFill.tryFetchMissingBatches(ctx, mw, p, blockStore, 0, 3, blockPeerScorer, addrBookFromPeerMgr(peerMgr))
				if errors.Is(ferr, ErrBlockDownloadStall) || errors.Is(ferr, ErrBlockDownloadTimeout) {
					rawFill.ReleaseLaneInFlight(0)
					err = ferr
					stallDisconnect = true
				} else if ferr != nil {
					if !IsBenignShutdownErr(ferr) {
						applog.Line("block", "progressive raw fetch: "+ferr.Error())
						if connectedAddr != "" {
							if shouldRotatePeerForStubBlock(ferr) {
								penalizeStubBlockPeer(blockPeerScorer, addrBookFromPeerMgr(peerMgr), connectedAddr)
							} else {
								penalizeBlockPeer(blockPeerScorer, addrBookFromPeerMgr(peerMgr), connectedAddr, sessionFailureHardFromFetchErr(ferr))
							}
							if shouldRedialPrimaryForAncientFetch(ferr, wantH) || shouldRotatePeerForForwardIBDFetch(ferr, wantH) || shouldRotatePeerForStubBlock(ferr) {
								err = ferr
								stallDisconnect = true
								if shouldRotatePeerForStubBlock(ferr) {
									applog.Line("block", fmt.Sprintf("primary %s sent undersized block stub - rotating peer", connectedAddr))
								} else {
									applog.Line("block", fmt.Sprintf("primary %s cannot serve height %d - rotating to archival peer", connectedAddr, wantH))
								}
							}
						}
					}
				} else if n > 0 && blockPeerScorer != nil && connectedAddr != "" {
					blockPeerScorer.NoteBlocksDelivered(connectedAddr, n)
				}
				if !stallDisconnect {
					if tipBFCoord != nil {
						tipBFCoord.maybeRunDeferred(ctx, mw, p, blockStore)
					}
					if !rawFill.useShortReadDeadline() {
						blockStore.FlushDeferredConnect()
						if err := blockStore.SyncUtxoCache(); err != nil {
							applog.Line("utxo", "post-IBD sync: "+err.Error())
						}
						mpSync.maybeRequest(ctx, mw, p, blockStore, &rawFill, peerMgr)
					}
					continue
				}
			}
			if recoverablePrimarySessionErr(err) {
				if primaryRedialStreak >= maxPrimaryRedialStreak {
					applog.Line("net", fmt.Sprintf("primary redial streak %d; pausing primary and retrying headers in background", primaryRedialStreak))
					pausePrimaryForRecovery(err)
					continue
				}
				if gap := primaryRedialMinInterval - time.Since(lastPrimaryRedial); gap > 0 {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(gap):
					}
				}
				lastPrimaryRedial = time.Now()
				primaryRedialStreak++
				applog.Line("net", fmt.Sprintf("primary session lost (%v); attempting redial", err))
				disc := DiscoverySnapshot(discoveryFeed, discoveredPeers)
				if blockPeerScorer != nil && len(disc) > 0 {
					disc = blockPeerScorer.MergeDiscoveryCandidates(disc, blockFetchWantHeight(blockStore))
				}
				res, rerr := RedialPrimary(PrimaryRedialOpts{
					Ctx: ctx, Dialer: d, Params: p, UserAgent: subVer, LocalServices: localServices,
					FixedPeer: cfg.Peer, Discovered: disc, ExcludeAddr: connectedAddr,
					Scorer: blockPeerScorer, PeerMgr: peerMgr,
					AddrBook:        activeAddrBook(peerMgr, bootstrapAddrBook),
					WantBlockHeight: blockFetchWantHeight(blockStore),
				})
				if rerr == nil {
					primaryRedialStreak = 0
					rawFill.ReleaseLaneInFlight(0)
					if blockPeerScorer != nil && connectedAddr != "" {
						blockPeerScorer.NoteSessionFailure(connectedAddr, false)
					}
					oldPrimary := connectedAddr
					_ = conn.Close()
					conn = res.Conn
					mw = res.MW
					p2pNetCtr = res.Ctr
					peerFromHandshake = res.DV
					connectedAddr = res.Addr
					peerSlot = res.Addr
					primaryExclude.Set(res.Addr)
					maybeNegotiateExtensions(conn, p.Magic, connectedAddr, extMgr, mw)
					MaybePushSPVBloom(spvBloom, mw, peerFromHandshake)
					if assistCandidates != nil {
						RefreshBlockAssistPool(assistCandidates, disc, peerMgr, blockPeerScorer, blockStore, addedNodes.List())
					}
					if cfg.FullNode && rbStore != nil {
						EnsureBlockAssistWorkers(blockAssistLaunch())
						if NeedsGenesisBlock(blockStore) {
							_ = SyncGenesisRawBlock(ctx, mw, p, blockStore)
						}
					}
					primaryPing = peerPingTracker{}
					if peerMgr != nil {
						peerMgr.ReplacePrimary(oldPrimary, res.Addr, res.Conn, res.MW, res.Ctr, res.DV)
					}
					if blockStore != nil {
						blockStore.SetBlockAnnounce(BlockAnnounceEnv{Primary: mw, PeerMgr: peerMgr, PrimaryCmpctHBTo: primaryCmpctHBTo})
						if peerMgr == nil {
							WireSoloPrimaryBlockPeerStats(blockStore, connectedAddr, func() { primaryLastBlock = time.Now() })
						}
					}
					if err := MaybeBroadcastFeeFilter(mw, pool, &lastSentFeeFilter); err != nil {
						applog.Line("net", "feefilter after primary redial: "+err.Error())
					}
					if j != nil {
						if n, herr := TopUpHeadersRound(ctx, mw, p, j, auxJ, blockStore, &rawFill, discoveryFeed); herr != nil {
							applog.Line("headers", "post-redial header top-up: "+herr.Error())
						} else if n > 0 {
							NotifyRPCTip(j, rbStore, utxoCache, tipWait)
						}
					}
					continue
				}
				applog.Line("net", "primary redial failed: "+rerr.Error())
				if shouldAutoRecoverHeaderSync(err) || shouldAutoRecoverHeaderSync(rerr) {
					pausePrimaryForRecovery(err)
					continue
				}
			}
			if shouldAutoRecoverHeaderSync(err) {
				pausePrimaryForRecovery(err)
				continue
			}
			return err
		}
		if peerMgr != nil && connectedAddr != "" {
			peerMgr.NotePeerRecv(connectedAddr)
			peerMgr.NotePeerMsg(connectedAddr, cmd, len(pl))
		} else if connectedAddr != "" {
			primaryLastRecv = time.Now()
			noteWriterRecv(mw, cmd, len(pl))
		}
		switch cmd {
		default:
			if extMgr != nil && handleExtensionP2P(extMgr, connectedAddr, cmd, pl, extensionSendFunc(mw, p.Magic)) {
				continue
			}
		case "ping":
			if err := replyPing(mw, pl); err != nil {
				return err
			}
		case "pong":
			if peerMgr != nil && connectedAddr != "" {
				peerMgr.NotePeerPong(connectedAddr, pl)
			} else {
				primaryPing.notePong(pl)
			}
		case "tx":
			if spvBloom != nil && spvBloom.Active() {
				if err := spvBloom.HandleMatchedTx(pl); err != nil {
					applog.Line("spv", "matched tx ingest: "+err.Error())
				}
				if !cfg.FullNode {
					break
				}
			}
			if cfg.AllowUnverifiedMempool {
				if err := pool.Add(pl); err != nil {
					applog.Line("mempool", "P2P tx not stored: "+err.Error())
				} else {
					applog.Line("mempool", fmt.Sprintf("P2P tx accepted unverified (%d bytes)", len(pl)))
				}
				break
			}
			if err := AdmitInboundTx(pl, connectedAddr, mw, misbehavior, pool, orphans, txIx, rbStore, j, cfg.Network, peerFeeFilters.Max(), cfg.FullRBF, cfg.Standard, cfg.MempoolLimits); err != nil {
				if errors.Is(err, ErrWitnessTxRejected) {
					break
				}
				if errors.Is(err, consensus.ErrOrphanTx) {
					applog.Line("mempool", fmt.Sprintf("P2P tx orphan stored (%d bytes, pending parents)", len(pl)))
				} else {
					HandleInboundTxAdmissionFailure(pl, connectedAddr, mw, misbehavior, err)
					applog.Line("mempool", "P2P tx rejected: "+err.Error())
				}
				break
			}
			_ = MaybeBroadcastFeeFilter(mw, pool, &lastSentFeeFilter)
			trackInboundMempoolFee(pl)
			if peerMgr != nil {
				peerMgr.NotePeerTx(connectedAddr)
			} else {
				primaryLastTx = time.Now()
			}
			applog.Line("mempool", fmt.Sprintf("P2P tx accepted (%d bytes)", len(pl)))
			FanInViaDGR(dgrMgr, "tx", pl)
		case "inv":
			if !cfg.FullNode && spvBloom != nil && spvBloom.Active() {
				if entries, err := wire.DecodeInvPayload(pl); err == nil {
					if err := spvBloom.RequestFilteredBlocks(mw, entries); err != nil {
						applog.Line("spv", "filtered getdata: "+err.Error())
					}
				}
			} else if blockStore == nil || !BodiesBehindHeaders(blockStore) {
				HandleInvBlockFetch(ctx, mw, p, blockStore, pl)
			}
			if !cfg.AllowUnverifiedMempool {
				deepIBDQuiet := ShouldSuppressInvTxFetchDuringIBD(blockStore)
				if deepIBDQuiet {
					if time.Since(lastInvTxQuietLog) >= 2*time.Minute {
						lastInvTxQuietLog = time.Now()
						applog.Line("mempool", "deep IBD: suppressing inv tx fetch/getdata until block bodies catch up (Core-style priority)")
					}
				} else {
					HandleInvTxFetch(ctx, mw, p, pool, orphans, txIx, rbStore, j, peerFeeFilters.Max(), TxInvMempoolCtx{
						Network:                cfg.Network,
						AllowUnverifiedMempool: cfg.AllowUnverifiedMempool,
						FullRBF:                cfg.FullRBF,
						Standard:               cfg.Standard,
						MempoolLimits:          cfg.MempoolLimits,
					}, connectedAddr, misbehavior, peerMgr, pl)
				}
			}
			FanInViaDGR(dgrMgr, "inv", pl)
		case "block":
			if blockStore != nil {
				HandleBroadcastBlock(mw, blockStore, connectedAddr, misbehavior, pl)
				RelayStoredBlock(blockStore, pl, connectedAddr)
				FanInViaDGR(dgrMgr, "block", pl)
			}
		case "merkleblock":
			if spvBloom != nil && spvBloom.Active() {
				if _, err := spvBloom.HandleMerkleBlock(pl); err != nil {
					applog.Line("spv", "merkleblock: "+err.Error())
					if connectedAddr != "" {
						misbehavior.Note(connectedAddr, misbehaviorReject, "bad-merkleblock")
					}
				}
			}
		case "addr":
			addrs, err := wire.DecodeAddrPayload(pl)
			if err != nil {
				applog.Line("net", "addr decode: "+err.Error())
				break
			}
			if len(addrs) == 0 {
				break
			}
			if discoveryFeed != nil {
				discoveryFeed.NoteFromAddrPayload(pl)
			}
			if peerMgr != nil {
				peerMgr.NoteAddrsFromPeer(connectedAddr, addrs)
			}
			const sample = 4
			var parts []string
			for i := 0; i < len(addrs) && i < sample; i++ {
				hp := addrs[i].HostPort()
				if hp != "" {
					parts = append(parts, hp)
				}
			}
			msg := fmt.Sprintf("addr: %d peer(s)", len(addrs))
			if len(parts) > 0 {
				msg += " e.g. " + parts[0]
				for i := 1; i < len(parts); i++ {
					msg += ", " + parts[i]
				}
				if len(addrs) > sample {
					msg += ", …"
				}
			}
			applog.Line("net", msg)
		case "reject":
			rj, err := wire.DecodeRejectPayload(pl)
			if err != nil {
				applog.Line("net", "reject (malformed): "+err.Error())
				break
			}
			applog.Line("net", "reject: "+rj.String())
			if connectedAddr != "" {
				misbehavior.Note(connectedAddr, misbehaviorReject, rj.String())
			}
		case "feefilter":
			fee, err := wire.DecodeFeeFilterPayload(pl)
			if err != nil {
				applog.Line("net", "feefilter (malformed): "+err.Error())
				break
			}
			peerFeeFilters.Set(connectedAddr, fee)
			applog.Line("net", fmt.Sprintf("feefilter from %s: %d koinu/kB (aggregate max %d)", connectedAddr, fee, peerFeeFilters.Max()))
		case "headers":
			if cfg.FullNode && ShouldDeferInboundHeaders(blockStore) {
				applog.Line("headers", "inbound headers ignored during forward block catch-up (bodies lag header tip)")
				break
			}
			n, partial, err := ApplyHeadersMessage(j, auxJ, p, pl, blockStore.NetworkTimeUnix(), blockStore)
			if err != nil {
				if strings.Contains(err.Error(), "fork deferred (marginal chain work") {
					applog.Line("headers", "inbound headers: "+err.Error())
					break
				}
				applog.Line("headers", "inbound headers rejected: "+err.Error())
				retryTopUp, pausePrimary, misbehave := InboundHeadersErrorPolicy(err)
				if retryTopUp {
					applog.Line("headers", err.Error())
					if blockStore != nil {
						MaybeResetContiguousAfterHeaderRewind(blockStore)
					}
					if hn, herr := TopUpHeadersRound(ctx, mw, p, j, auxJ, blockStore, &rawFill, discoveryFeed); herr != nil {
						applog.Line("headers", "post-rewind header top-up: "+herr.Error())
					} else if hn > 0 {
						NotifyRPCTip(j, rbStore, utxoCache, tipWait)
					}
					break
				}
				if pausePrimary {
					pausePrimaryForRecovery(err)
					break
				}
				if misbehave && connectedAddr != "" {
					misbehavior.Note(connectedAddr, misbehaviorInvalidHeaders, err.Error())
				}
				break
			}
			if n == 0 {
				break
			}
			tip, _ := j.TipHeight()
			if peerMgr != nil && connectedAddr != "" {
				tipHash := ""
				if tip >= 0 {
					if h80, err := j.ReadHeaderAt(tip); err == nil {
						tipHash = pow.BlockHashHex(h80)
					}
				}
				peerMgr.NotePeerHeaders(connectedAddr, tip, tipHash)
			}
			applog.Line("headers", fmt.Sprintf("inbound %d header(s) (sendheaders path), tip height %d", n, tip))
			FanInViaDGR(dgrMgr, "headers", pl)
			rawFill.OnTipChanged(tip)
			NotifyRPCTip(j, rbStore, utxoCache, tipWait)
			if rbStore != nil && cfg.FullNode && partial && n > 0 {
				applog.Line("headers", "partial inbound batch - likely at chain tip")
			}
		case "getaddr":
			if peerMgr != nil {
				body, err := wire.EncodeAddrPayload(peerMgr.AddrSample(25))
				if err == nil && len(body) > 0 {
					_ = mw.Write("addr", body)
				}
			}
		case "sendcmpct":
			if announce, weAnnounce, err := NegotiateSendCmpct(peerMgr, nil, mw, pl, &primaryCmpctHBTo); err != nil {
				applog.Line("net", "sendcmpct: "+err.Error())
			} else {
				if announce {
					primaryCmpctHBFrom = true
				}
				if weAnnounce && blockStore != nil {
					blockStore.SetPrimaryCmpctHBTo(primaryCmpctHBTo)
				}
			}
		case "getdata":
			serve := GetDataServeEnv{Raw: rbStore, Pool: pool, TxIx: txIx}
			if peerMgr != nil {
				serve.Bloom = peerMgr.PeerBloom(connectedAddr)
			}
			if serve.Raw != nil || serve.Pool != nil || serve.TxIx != nil {
				if err := HandleInboundGetData(ctx, mw, serve, pl); err != nil {
					applog.Line("net", "getdata serve: "+err.Error())
				}
			}
		case "filterload":
			if err := HandleFilterLoad(peerMgr, connectedAddr, pl, misbehavior); err != nil {
				applog.Line("net", "filterload: "+err.Error())
			}
		case "filteradd":
			if err := HandleFilterAdd(peerMgr, connectedAddr, pl, misbehavior); err != nil {
				applog.Line("net", "filteradd: "+err.Error())
			}
		case "filterclear":
			HandleFilterClear(peerMgr, connectedAddr)
		case "getheaders":
			if j != nil {
				if err := HandleInboundGetHeaders(ctx, mw, GetHeadersServeEnv{Journal: j, Aux: auxJ}, pl); err != nil {
					applog.Line("headers", "getheaders serve: "+err.Error())
				}
			}
		case "sendheaders":
			_ = mw.Write("sendheaders", nil)
		case "getcfilters":
			if err := HandleInboundGetCFilters(mw, j, rbStore, txIx, filterIx, pl); err != nil {
				applog.Line("net", "getcfilters: "+err.Error())
			}
		case "getcfheaders":
			if err := HandleInboundGetCFHeaders(mw, j, rbStore, txIx, filterIx, pl); err != nil {
				applog.Line("net", "getcfheaders: "+err.Error())
			}
		case "getcfcheckpt":
			if err := HandleInboundGetCFCheckpt(mw, j, rbStore, txIx, filterIx, pl); err != nil {
				applog.Line("net", "getcfcheckpt: "+err.Error())
			}
		case "cmpctblock":
			if primaryCmpctHBFrom && blockStore != nil {
				plink := &peerLink{addr: connectedAddr, cmpctHBFrom: true, cmpctPending: primaryCmpctPending}
				cmpctEnv := CmpctServeEnv{Raw: rbStore, Pool: pool, Block: blockStore}
				HandleInboundCmpctBlock(mw, cmpctEnv, plink, pl)
				primaryCmpctPending = plink.cmpctPending
			}
		case "getblocktxn":
			if err := HandleInboundGetBlockTxn(mw, rbStore, pl); err != nil {
				applog.Line("net", "getblocktxn serve: "+err.Error())
			}
		case "blocktxn":
			if blockStore != nil {
				plink := &peerLink{addr: connectedAddr, cmpctPending: primaryCmpctPending}
				cmpctEnv := CmpctServeEnv{Raw: rbStore, Pool: pool, Block: blockStore}
				HandleInboundBlockTxn(mw, cmpctEnv, plink, pl)
				primaryCmpctPending = plink.cmpctPending
			}
		}
	}
}
