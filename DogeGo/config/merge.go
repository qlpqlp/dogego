// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"strings"

	"dogego/chain"
)

// MaxRawBlockBackfill is the upper bound for post-genesis tip-aligned full-block fetches per startup.
// Single-peer MVP: values in the tens of thousands overwhelm one outbound link; progressive catch-up
// fills the rest after startup (see node.rawsync_progress).
const MaxRawBlockBackfill = 4096

// Merged holds effective node settings after merging CLI flags and dogecoinconf.json.
// visited maps flag names the user set on the command line (flag.FlagSet.Visit).
type Merged struct {
	DataDir   string
	Peer      string
	Network   string
	RPCAddr   string
	WebUI     string
	NoWebUI   bool
	NoBrowser bool
	Tray      bool
	NoWallet  bool
	Mine            bool
	MiningAddress   string
	UAComment            string
	UACommentTipAddress  string
	UACommentUseNodeTip  *bool
	// RawBlockBackfill merges file + CLI; see EffectiveRawBlockBackfillCount.
	RawBlockBackfill        int
	RawBlockBackfillFromCLI bool
	// AllowUnverifiedMempool skips script/coinbase mempool checks (testing only).
	AllowUnverifiedMempool bool
	// MempoolFullRBF enables mempoolfullrbf-style replacement policy.
	MempoolFullRBF bool
	// MaxOrphanTx caps orphan pool size (0 = default).
	MaxOrphanTx int
	// MaxMempoolMB / MempoolExpiryHours are Core -maxmempool / -mempoolexpiry (0 = defaults).
	MaxMempoolMB       int
	MempoolExpiryHours int
	// PersistMempool enables dogego_mempool.json auto load/save (default true).
	PersistMempool bool
	// Relay standardness (Core policy.cpp defaults when unset).
	HardDustLimitKoinu int64
	AcceptDataCarrier   bool
	DatacarrierSize     int
	PermitBareMultisig  bool
	MaxTxFeeKoinu       int64
	// IncrementalRelayFeeKoinuPerKB is Core -incrementalrelayfee (0 = default).
	IncrementalRelayFeeKoinuPerKB uint64
	// MinRelayTxFeeKoinuPerKB is Core -minrelaytxfee (0 = default unless raised by incremental).
	MinRelayTxFeeKoinuPerKB uint64
	LimitAncestorCount    int
	LimitDescendantCount  int
	LimitAncestorSizeKB   int
	LimitDescendantSizeKB int
	BlockMaxWeight        int
	// NoTxIndex disables local tx id indexing under indexes/tx (smaller default tip raw batch).
	NoTxIndex bool
	// BlockStorageLayout / BlockZstd control raw block on-disk layout (see store.BlockStorageOpts).
	BlockStorageLayout string
	BlockZstd          bool
	// TxIndexEmbedTx stores serialized txs in the tx index when true (default).
	TxIndexEmbedTx bool
	// RpcUser / RpcPassword enable JSON-RPC HTTP Basic auth when RpcUser is non-empty (from dogecoinconf.json only).
	RpcUser     string
	RpcPassword string
	// RpcCookie enables Core-style .cookie file auth (from dogecoinconf.json only).
	RpcCookie bool
	// RpcAllowIP is extra JSON-RPC client allowlist (loopback always permitted).
	RpcAllowIP []string
	// RpcWhitelist restricts JSON-RPC methods when non-empty.
	RpcWhitelist []string
	// RpclimitPerMin / RpcAuthMaxFail are optional JSON-RPC rate limits (see rpc/rpclimit.go).
	RpclimitPerMin int
	RpcAuthMaxFail int
	// NodeMode is "full" or "spv" (see File.NodeMode).
	NodeMode string
	P2PConnectivity string
	// Firewall is auto|always|never (OS P2P firewall rules).
	Firewall string
	// Upnp is auto|enable|disable (UPnP/NAT-PMP port mapping).
	Upnp string
	ZmqPubHashBlock string
	ZmqPubHashTx    string
	ZmqPubRawBlock  string
	ZmqPubRawTx     string
	MaxOutbound       int
	MaxInbound        int
	BlockSyncWorkers  int
	AddedNodes        []string
	DNSSeeds          []string
	// DNSSeedLookup enables DNS seed host lookups (Core -dnsseed).
	DNSSeedLookup bool
	// AnalyticsSidecar when non-nil overrides default (true). nil = enabled.
	AnalyticsSidecar *bool
	// IBDOptimize when non-nil overrides default (true). nil = enabled.
	IBDOptimize *bool
	// DBCacheMB is Core -dbcache UTXO budget in MB (0 = auto from free RAM).
	DBCacheMB int
	// AlertNotify is a shell command run when chain warnings change (Core -alertnotify).
	AlertNotify string
	// AssumeValid is Core -assumevalid block hash hex (empty = default; "0" = verify all).
	AssumeValid string
	// Checkpoints enables Core mapCheckpoints hash checks during header sync (nil = default on).
	Checkpoints *bool
	// MaxTipAge is Core -maxtipage in seconds (0 = default 86400).
	MaxTipAge int
	RpcTLSCert   string
	RpcTLSKey    string
	WebUITLSCert string
	WebUITLSKey  string
	WebUITLSLocal bool
	RpcTLSLocal   bool
	LocalTLSTrustCA bool
	DogeGoRelayCGNAT DogeGoRelayCGNAT
	CoreRPCAddr     string
	CoreRPCUser     string
	CoreRPCPassword string
	SignerCmd       string
}

// EffectiveRawBlockBackfillCount returns how many tip-aligned full blocks to fetch after genesis (0 = skip tip batch).
func (m Merged) EffectiveRawBlockBackfillCount() int {
	if strings.ToLower(strings.TrimSpace(m.NodeMode)) == "spv" {
		return 0
	}
	if m.RawBlockBackfillFromCLI {
		if m.RawBlockBackfill < 0 {
			return 0
		}
		if m.RawBlockBackfill == 0 {
			return 0
		}
		if m.RawBlockBackfill > MaxRawBlockBackfill {
			return MaxRawBlockBackfill
		}
		return m.RawBlockBackfill
	}
	if m.RawBlockBackfill == -1 {
		return 0
	}
	if m.RawBlockBackfill == 0 {
		if !m.NoTxIndex {
			return MaxRawBlockBackfill
		}
		return 5
	}
	if m.RawBlockBackfill > MaxRawBlockBackfill {
		return MaxRawBlockBackfill
	}
	if m.RawBlockBackfill < -1 {
		return 0
	}
	return m.RawBlockBackfill
}

// EffectiveMaxOrphanTx returns the orphan pool capacity (Core -maxorphantx; 0 = DogeGo default).
func (m Merged) EffectiveMaxOrphanTx() int {
	if m.MaxOrphanTx <= 0 {
		return 100
	}
	if m.MaxOrphanTx > 1000 {
		return 1000
	}
	return m.MaxOrphanTx
}

// CheckpointsEnabled reports whether Core header checkpoint hashes are enforced (default true).
func (m Merged) CheckpointsEnabled() bool {
	if m.Checkpoints == nil {
		return true
	}
	return *m.Checkpoints
}

// EffectiveMaxTipAge returns Core -maxtipage seconds for IBD (0 = 86400).
func (m Merged) EffectiveMaxTipAge() int64 {
	return chain.EffectiveMaxTipAge(m.MaxTipAge)
}

// FileFromMerged maps merged runtime settings back to a persistable File.
func FileFromMerged(m Merged) File {
	f := File{
		DataDir:                m.DataDir,
		Peer:                   m.Peer,
		Network:                m.Network,
		RPCAddr:                m.RPCAddr,
		WebUI:                  m.WebUI,
		NoWebUI:                m.NoWebUI,
		NoBrowser:              m.NoBrowser,
		Tray:                   TrayPtr(m.Tray),
		NoWallet:               m.NoWallet,
		Mine:                   m.Mine,
		UAComment:              m.UAComment,
		UACommentTipAddress:    m.UACommentTipAddress,
		UACommentUseNodeTip:    m.UACommentUseNodeTip,
		AllowUnverifiedMempool: m.AllowUnverifiedMempool,
		MempoolFullRBF:         m.MempoolFullRBF,
		MaxOrphanTx:            m.MaxOrphanTx,
		MaxMempoolMB:           m.MaxMempoolMB,
		MempoolExpiryHours: m.MempoolExpiryHours,
		HardDustLimitKoinu: m.HardDustLimitKoinu,
		MaxTxFeeKoinu:                  m.MaxTxFeeKoinu,
		IncrementalRelayFeeKoinuPerKB: m.IncrementalRelayFeeKoinuPerKB,
		MinRelayTxFeeKoinuPerKB:       m.MinRelayTxFeeKoinuPerKB,
		LimitAncestorCount:            m.LimitAncestorCount,
		LimitDescendantCount:   m.LimitDescendantCount,
		LimitAncestorSizeKB:    m.LimitAncestorSizeKB,
		LimitDescendantSizeKB:  m.LimitDescendantSizeKB,
		BlockMaxWeight:         m.BlockMaxWeight,
		DatacarrierSize:        m.DatacarrierSize,
		NodeMode:               m.NodeMode,
		P2PConnectivity:        m.P2PConnectivity,
		Firewall:               m.Firewall,
		Upnp:                   m.Upnp,
		ZmqPubHashBlock:        m.ZmqPubHashBlock,
		ZmqPubHashTx:           m.ZmqPubHashTx,
		ZmqPubRawBlock:         m.ZmqPubRawBlock,
		ZmqPubRawTx:            m.ZmqPubRawTx,
		MaxOutbound:            m.MaxOutbound,
		MaxInbound:             m.MaxInbound,
		BlockSyncWorkers:       m.BlockSyncWorkers,
		AddedNodes:             append([]string(nil), m.AddedNodes...),
		DNSSeeds:               append([]string(nil), m.DNSSeeds...),
		NoTxIndex:              m.NoTxIndex,
		BlockStorageLayout:     m.BlockStorageLayout,
		BlockZstd:              m.BlockZstd,
		RpcUser:                m.RpcUser,
		RpcPassword:            m.RpcPassword,
		RpcCookie:              m.RpcCookie,
		RpcAllowIP:             append([]string(nil), m.RpcAllowIP...),
		AlertNotify:    m.AlertNotify,
		AssumeValid: m.AssumeValid,
		Checkpoints: m.Checkpoints,
		MaxTipAge:   m.MaxTipAge,
		RpcTLSCert:     m.RpcTLSCert,
		RpcTLSKey:      m.RpcTLSKey,
		WebUITLSCert:   m.WebUITLSCert,
		WebUITLSKey:    m.WebUITLSKey,
		WebUITLSLocal:  m.WebUITLSLocal,
		RpcTLSLocal:    m.RpcTLSLocal,
		LocalTLSTrustCA: m.LocalTLSTrustCA,
		DogeGoRelayCGNAT: m.DogeGoRelayCGNAT,
		CoreRPCAddr:     m.CoreRPCAddr,
		CoreRPCUser:     m.CoreRPCUser,
		CoreRPCPassword: m.CoreRPCPassword,
		SignerCmd:       m.SignerCmd,
	}
	if m.RawBlockBackfillFromCLI && m.RawBlockBackfill == 0 {
		f.RawBlockBackfill = -1
	} else if !m.RawBlockBackfillFromCLI && m.RawBlockBackfill == 0 {
		f.RawBlockBackfill = 0
	} else {
		f.RawBlockBackfill = m.RawBlockBackfill
	}
	if !m.PersistMempool {
		off := false
		f.PersistMempool = &off
	}
	if !m.DNSSeedLookup {
		off := false
		f.DNSSeedLookup = &off
	}
	if !m.TxIndexEmbedTx {
		off := false
		f.TxIndexEmbedTx = &off
	}
	if m.AnalyticsSidecar != nil {
		v := *m.AnalyticsSidecar
		f.AnalyticsSidecar = &v
	}
	if m.IBDOptimize != nil {
		v := *m.IBDOptimize
		f.IBDOptimize = &v
	}
	f.DBCacheMB = m.DBCacheMB
	return f
}

// MergeNode combines on-disk config with parsed flags. CLI wins when a flag was explicitly passed.
// Node mode resolution: explicit -mode wins; otherwise defaultNodeMode from the subcommand (`node` → full,
// `spvnode` → spv). file.node_mode is not applied here so `dogego node` stays full even if the JSON still
// says spv from an earlier run (use -mode or spvnode to choose SPV).
func MergeNode(visited map[string]bool, file File, datadir, peer, netName, rpcAddr, webui string, nowebui, nobrowser, mine, nowallet bool, uaComment string, rawBlockBackfill int, allowUnverifiedMempool, mempoolFullRBF bool, noTxIndex bool, modeFlag string, visitedMode bool, defaultNodeMode string, alertNotify string) Merged {
	out := Merged{}
	pick := func(name, fileVal, flagVal string) string {
		if visited[name] {
			return flagVal
		}
		if fileVal != "" {
			return fileVal
		}
		return flagVal
	}
	out.DataDir = pick("datadir", file.DataDir, datadir)
	out.Peer = pick("peer", file.Peer, peer)
	out.Network = netName
	if !visited["network"] && file.Network != "" {
		out.Network = file.Network
	}
	out.RPCAddr = pick("rpc", file.RPCAddr, rpcAddr)
	out.WebUI = webui
	if !visited["webui"] && file.WebUI != "" {
		out.WebUI = file.WebUI
	}
	out.NoWebUI = file.NoWebUI
	if visited["nowebui"] {
		out.NoWebUI = nowebui
	}
	out.NoBrowser = file.NoBrowser
	if visited["nobrowser"] {
		out.NoBrowser = nobrowser
	}
	out.Tray = file.TrayEnabled(false)
	out.UAComment = pick("uacomment", file.UAComment, uaComment)
	out.UACommentTipAddress = file.UACommentTipAddress
	out.UACommentUseNodeTip = file.UACommentUseNodeTip
	out.Mine = file.Mine
	if visited["mine"] {
		out.Mine = mine
	}
	out.MiningAddress = strings.TrimSpace(file.MiningAddress)
	out.NoWallet = file.NoWallet
	if visited["nowallet"] {
		out.NoWallet = nowallet
	}
	out.RawBlockBackfill = file.RawBlockBackfill
	if visited["rawblock_backfill"] {
		out.RawBlockBackfill = rawBlockBackfill
		out.RawBlockBackfillFromCLI = true
	}
	out.AllowUnverifiedMempool = file.AllowUnverifiedMempool
	if visited["allowunverifiedmempool"] {
		out.AllowUnverifiedMempool = allowUnverifiedMempool
	}
	out.MempoolFullRBF = file.MempoolFullRBF
	if visited["mempoolfullrbf"] {
		out.MempoolFullRBF = mempoolFullRBF
	}
	out.MaxOrphanTx = file.MaxOrphanTx
	out.MaxMempoolMB = file.MaxMempoolMB
	out.MempoolExpiryHours = file.MempoolExpiryHours
	out.PersistMempool = EffectivePersistMempool(file)
	out.NoTxIndex = file.NoTxIndex
	if visited["no_tx_index"] {
		out.NoTxIndex = noTxIndex
	}
	out.BlockStorageLayout = file.BlockStorageLayout
	out.BlockZstd = file.BlockZstd
	out.TxIndexEmbedTx = file.EffectiveTxIndexEmbedTx()
	dm := strings.ToLower(strings.TrimSpace(defaultNodeMode))
	if dm != "spv" {
		dm = "full"
	}
	out.NodeMode = dm
	if visitedMode {
		nm := strings.ToLower(strings.TrimSpace(modeFlag))
		if nm == "full" || nm == "spv" {
			out.NodeMode = nm
		}
	}
	out.RpcUser = file.RpcUser
	out.RpcPassword = file.RpcPassword
	out.RpcCookie = file.RpcCookie
	out.RpcAllowIP = append([]string(nil), file.RpcAllowIP...)
	out.RpcWhitelist = append([]string(nil), file.RpcWhitelist...)
	out.RpclimitPerMin = file.RpclimitPerMin
	out.RpcAuthMaxFail = file.RpcAuthMaxFail
	out.HardDustLimitKoinu = file.HardDustLimitKoinu
	out.MaxTxFeeKoinu = file.MaxTxFeeKoinu
	out.IncrementalRelayFeeKoinuPerKB = file.IncrementalRelayFeeKoinuPerKB
	out.MinRelayTxFeeKoinuPerKB = file.MinRelayTxFeeKoinuPerKB
	out.LimitAncestorCount = file.LimitAncestorCount
	out.LimitDescendantCount = file.LimitDescendantCount
	out.LimitAncestorSizeKB = file.LimitAncestorSizeKB
	out.LimitDescendantSizeKB = file.LimitDescendantSizeKB
	out.BlockMaxWeight = file.BlockMaxWeight
	if file.AcceptDataCarrier != nil {
		out.AcceptDataCarrier = *file.AcceptDataCarrier
	} else {
		out.AcceptDataCarrier = true
	}
	out.DatacarrierSize = file.DatacarrierSize
	if file.PermitBareMultisig != nil {
		out.PermitBareMultisig = *file.PermitBareMultisig
	} else {
		out.PermitBareMultisig = true
	}
	out.P2PConnectivity = file.P2PConnectivity
	out.Firewall = pick("firewall", file.Firewall, "auto")
	if out.Firewall == "" {
		out.Firewall = "auto"
	}
	out.Upnp = pick("upnp", file.Upnp, "")
	if out.Upnp == "" {
		out.Upnp = "auto"
	}
	out.ZmqPubHashBlock = strings.TrimSpace(file.ZmqPubHashBlock)
	out.ZmqPubHashTx = strings.TrimSpace(file.ZmqPubHashTx)
	out.ZmqPubRawBlock = strings.TrimSpace(file.ZmqPubRawBlock)
	out.ZmqPubRawTx = strings.TrimSpace(file.ZmqPubRawTx)
	out.MaxOutbound = file.MaxOutbound
	out.MaxInbound = file.MaxInbound
	out.BlockSyncWorkers = file.BlockSyncWorkers
	out.AddedNodes = append([]string(nil), file.AddedNodes...)
	out.DNSSeeds = append([]string(nil), file.DNSSeeds...)
	out.DNSSeedLookup = file.DNSSeedLookupEnabled()
	out.AnalyticsSidecar = file.AnalyticsSidecar
	out.IBDOptimize = file.IBDOptimize
	out.DBCacheMB = file.DBCacheMB
	out.AlertNotify = pick("alertnotify", file.AlertNotify, alertNotify)
	out.AssumeValid = strings.TrimSpace(file.AssumeValid)
	out.Checkpoints = file.Checkpoints
	out.MaxTipAge = file.MaxTipAge
	out.RpcTLSCert = strings.TrimSpace(file.RpcTLSCert)
	out.RpcTLSKey = strings.TrimSpace(file.RpcTLSKey)
	out.WebUITLSCert = strings.TrimSpace(file.WebUITLSCert)
	out.WebUITLSKey = strings.TrimSpace(file.WebUITLSKey)
	out.WebUITLSLocal = file.WebUITLSLocal
	out.RpcTLSLocal = file.RpcTLSLocal
	out.LocalTLSTrustCA = file.LocalTLSTrustCA
	out.DogeGoRelayCGNAT = file.DogeGoRelayCGNAT
	out.CoreRPCAddr = file.CoreRPCAddr
	out.CoreRPCUser = file.CoreRPCUser
	out.CoreRPCPassword = file.CoreRPCPassword
	out.SignerCmd = file.SignerCmd
	if out.RPCAddr == "" && out.NodeMode == "full" {
		out.RPCAddr = DefaultRPCListenAddr(out.Network)
	}
	return out
}

// FromFile maps a saved config file into Merged defaults (for use after setup wizard).
func FromFile(f File) Merged {
	m := Merged{
		DataDir:                f.DataDir,
		Peer:                   f.Peer,
		Network:                f.Network,
		RPCAddr:                f.RPCAddr,
		WebUI:                  f.WebUI,
		NoWebUI:                f.NoWebUI,
		NoBrowser:              f.NoBrowser,
		Tray:                   f.TrayEnabled(false),
		NoWallet:               f.NoWallet,
		Mine:                   f.Mine,
		UAComment:             f.UAComment,
		UACommentTipAddress:   f.UACommentTipAddress,
		UACommentUseNodeTip:   f.UACommentUseNodeTip,
		RawBlockBackfill:       f.RawBlockBackfill,
		AllowUnverifiedMempool: f.AllowUnverifiedMempool,
		MempoolFullRBF:     f.MempoolFullRBF,
		MaxOrphanTx:        f.MaxOrphanTx,
		MaxMempoolMB:       f.MaxMempoolMB,
		MempoolExpiryHours: f.MempoolExpiryHours,
		HardDustLimitKoinu:    f.HardDustLimitKoinu,
		MaxTxFeeKoinu:                  f.MaxTxFeeKoinu,
		IncrementalRelayFeeKoinuPerKB: f.IncrementalRelayFeeKoinuPerKB,
		MinRelayTxFeeKoinuPerKB:       f.MinRelayTxFeeKoinuPerKB,
		LimitAncestorCount:            f.LimitAncestorCount,
		LimitDescendantCount:  f.LimitDescendantCount,
		LimitAncestorSizeKB:   f.LimitAncestorSizeKB,
		LimitDescendantSizeKB: f.LimitDescendantSizeKB,
		BlockMaxWeight:        f.BlockMaxWeight,
		NodeMode:              f.NodeMode,
		P2PConnectivity:       f.P2PConnectivity,
		Firewall:              f.Firewall,
		Upnp:                  f.Upnp,
		ZmqPubHashBlock:       f.ZmqPubHashBlock,
		ZmqPubHashTx:          f.ZmqPubHashTx,
		ZmqPubRawBlock:        f.ZmqPubRawBlock,
		ZmqPubRawTx:           f.ZmqPubRawTx,
		MaxOutbound:           f.MaxOutbound,
		MaxInbound:            f.MaxInbound,
		BlockSyncWorkers:      f.BlockSyncWorkers,
		AddedNodes:            append([]string(nil), f.AddedNodes...),
		DNSSeeds:              append([]string(nil), f.DNSSeeds...),
		DNSSeedLookup:         f.DNSSeedLookupEnabled(),
		AnalyticsSidecar:      f.AnalyticsSidecar,
		IBDOptimize:           f.IBDOptimize,
		DBCacheMB:             f.DBCacheMB,
		NoTxIndex:              f.NoTxIndex,
		RpcUser:                f.RpcUser,
		RpcPassword:            f.RpcPassword,
		RpcCookie:          f.RpcCookie,
		RpcAllowIP:         append([]string(nil), f.RpcAllowIP...),
		RpcWhitelist:       append([]string(nil), f.RpcWhitelist...),
		RpclimitPerMin:     f.RpclimitPerMin,
		RpcAuthMaxFail:     f.RpcAuthMaxFail,
		DatacarrierSize:    f.DatacarrierSize,
		AlertNotify:   f.AlertNotify,
		AssumeValid:   strings.TrimSpace(f.AssumeValid),
		Checkpoints:   f.Checkpoints,
		DogeGoRelayCGNAT: f.DogeGoRelayCGNAT,
		CoreRPCAddr:     f.CoreRPCAddr,
		CoreRPCUser:     f.CoreRPCUser,
		CoreRPCPassword: f.CoreRPCPassword,
		SignerCmd:       f.SignerCmd,
		RpcTLSCert:      strings.TrimSpace(f.RpcTLSCert),
		RpcTLSKey:       strings.TrimSpace(f.RpcTLSKey),
		WebUITLSCert:    strings.TrimSpace(f.WebUITLSCert),
		WebUITLSKey:     strings.TrimSpace(f.WebUITLSKey),
		WebUITLSLocal:   f.WebUITLSLocal,
		RpcTLSLocal:     f.RpcTLSLocal,
		LocalTLSTrustCA: f.LocalTLSTrustCA,
	}
	if f.AcceptDataCarrier != nil {
		m.AcceptDataCarrier = *f.AcceptDataCarrier
	} else {
		m.AcceptDataCarrier = true
	}
	if f.PermitBareMultisig != nil {
		m.PermitBareMultisig = *f.PermitBareMultisig
	} else {
		m.PermitBareMultisig = true
	}
	if m.Network == "" {
		m.Network = "testnet"
	}
	if m.WebUI == "" {
		m.WebUI = DefaultWebUIListen
	}
	if strings.TrimSpace(m.NodeMode) == "" {
		m.NodeMode = "full"
	}
	return m
}
