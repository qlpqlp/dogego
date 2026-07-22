// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// FileName is the on-disk configuration file name.
const FileName = "dogecoinconf.json"

// AppConfigDirName is the subdirectory under os.UserConfigDir() for dogecoinconf.json.
const AppConfigDirName = "DogeGo"

const legacyAppConfigDirName = "dogego"

// File holds persisted node settings (subset of CLI flags).
type File struct {
	DataDir   string `json:"datadir,omitempty"`
	Peer      string `json:"peer,omitempty"`
	Network   string `json:"network,omitempty"`
	RPCAddr   string `json:"rpc,omitempty"`
	WebUI     string `json:"webui,omitempty"`
	NoWebUI   bool   `json:"nowebui"`
	NoBrowser bool   `json:"nobrowser"`
	// Tray enables a system tray icon (Open Dashboard / Shutdown Node) while the node runs.
	// Omitted in JSON defaults to on for interactive desktop sessions.
	Tray *bool `json:"tray,omitempty"`
	// NoWallet disables the built-in web wallet (wallet.json under the network datadir).
	NoWallet bool `json:"nowallet"`
	// Mine enables background block generation on reboot testnet (defaults on; coinbase → wallet).
	Mine bool `json:"mine"`
	// MiningAddress is the P2PKH address for generate RPC (optional; testnet wallet also supplies one).
	MiningAddress string `json:"miningaddress,omitempty"`
	// UAComment is optional metadata (like Core -uacomment); the wire user agent is always /DogeGo:1.14.9(...)/.
	UAComment string `json:"uacomment,omitempty"`
	// UACommentTipAddress is appended to uacomment in the P2P user-agent (visible to all peers).
	UACommentTipAddress string `json:"uacomment_tip_address,omitempty"`
	// UACommentUseNodeTip derives a dedicated HD tip key at m/44'/3'/0'/2/0 (separate from Receive).
	UACommentUseNodeTip *bool `json:"uacomment_use_node_tip,omitempty"`
	// RawBlockBackfill is how many full blocks to fetch at the header tip (plus genesis via a separate fetch).
	// 0 omits file value: with tx indexing (default full node) uses MaxRawBlockBackfill; with no_tx_index uses 5.
	// -1 disables this tip batch (genesis only). Positive values cap at MaxRawBlockBackfill.
	RawBlockBackfill int `json:"rawblock_backfill,omitempty"`
	// AllowUnverifiedMempool skips coinbase/script mempool admission (local testing only).
	AllowUnverifiedMempool bool `json:"allow_unverified_mempool,omitempty"`
	// MempoolFullRBF enables replacement of non-BIP125-signaling mempool conflicts (Core -mempoolfullrbf).
	MempoolFullRBF bool `json:"mempoolfullrbf,omitempty"`
	// MaxOrphanTx caps the in-memory orphan transaction pool (Core -maxorphantx; 0 = DogeGo default 100).
	MaxOrphanTx int `json:"maxorphantx,omitempty"`
	// MaxMempoolMB caps total serialized tx bytes in the mempool (Core -maxmempool; 0 = default 300).
	MaxMempoolMB int `json:"maxmempool,omitempty"`
	// MempoolExpiryHours drops txs older than this (Core -mempoolexpiry; 0 = default 24).
	MempoolExpiryHours int `json:"mempoolexpiry,omitempty"`
	// PersistMempool when false skips auto load/save of dogego_mempool.json (Core -persistmempool; default true).
	PersistMempool *bool `json:"persistmempool,omitempty"`
	// HardDustLimitKoinu is the minimum non-OP_RETURN output value for relay (Core -harddustlimit; default 100000 = 0.001 DOGE).
	HardDustLimitKoinu int64 `json:"harddustlimit,omitempty"`
	// AcceptDataCarrier allows OP_RETURN outputs (Core -datacarrier).
	AcceptDataCarrier *bool `json:"acceptdatacarrier,omitempty"`
	// DatacarrierSize is max OP_RETURN scriptPubKey size in bytes (Core -datacarriersize; default 83).
	DatacarrierSize int `json:"datacarriersize,omitempty"`
	// PermitBareMultisig allows bare multisig outputs (Core -permitbaremultisig).
	PermitBareMultisig *bool `json:"permitbaremultisig,omitempty"`
	// MaxTxFeeKoinu caps per-tx relay fee (Core -maxtxfee); 0 uses DefaultMaxAbsurdTxFeeKoinu.
	MaxTxFeeKoinu int64 `json:"maxtxfee,omitempty"`
	// IncrementalRelayFeeKoinuPerKB is the BIP125 / mempool-limit incremental relay rate (Core -incrementalrelayfee; 0 = default).
	IncrementalRelayFeeKoinuPerKB uint64 `json:"incrementalrelayfee,omitempty"`
	// MinRelayTxFeeKoinuPerKB is the minimum relay feerate (Core -minrelaytxfee; 0 = default, or raised to incrementalrelayfee when unset).
	MinRelayTxFeeKoinuPerKB uint64 `json:"minrelaytxfee,omitempty"`
	// LimitAncestorCount / LimitDescendantCount cap in-mempool package counts (Core -limitancestorcount / -limitdescendantcount).
	LimitAncestorCount    int `json:"limitancestorcount,omitempty"`
	LimitDescendantCount  int `json:"limitdescendantcount,omitempty"`
	LimitAncestorSizeKB   int `json:"limitancestorsize,omitempty"`
	LimitDescendantSizeKB int `json:"limitdescendantsize,omitempty"`
	// BlockMaxWeight caps getblocktemplate weightlimit (Core -blockmaxweight; default 4000000).
	BlockMaxWeight int `json:"blockmaxweight,omitempty"`
	// NoTxIndex disables indexes/tx and uses a smaller default raw-block tip batch (full node only).
	NoTxIndex bool `json:"no_tx_index,omitempty"`
	// BlockStorageLayout is "perfile" (default) or "bundled" (blk*.dat under rawblocks/).
	BlockStorageLayout string `json:"block_storage_layout,omitempty"`
	// BlockZstd enables zstd compression of stored block payloads (transparent on read).
	BlockZstd bool `json:"block_zstd,omitempty"`
	// TxIndexEmbedTx when true stores full serialized txs in indexes/tx (v2); false uses offset-only entries.
	TxIndexEmbedTx *bool `json:"tx_index_embed_tx,omitempty"`
	// RpcUser / RpcPassword enable HTTP Basic auth on JSON-RPC when RpcUser is non-empty.
	RpcUser     string `json:"rpc_user,omitempty"`
	RpcPassword string `json:"rpc_password,omitempty"`
	// RpcCookie, when true, writes chaindatadir/.cookie on startup (random password) and enables HTTP Basic
	// auth with those credentials (Core-style). Takes precedence over rpc_user/rpc_password for this process.
	RpcCookie bool `json:"rpc_cookie"`
	// RpcAllowIP lists extra JSON-RPC client subnets (Core -rpcallowip); loopback is always allowed.
	RpcAllowIP []string `json:"rpcallowip,omitempty"`
	// RpcWhitelist limits JSON-RPC to these method names when non-empty (safe operator subset).
	RpcWhitelist []string `json:"rpcwhitelist,omitempty"`
	// RpclimitPerMin caps JSON-RPC POST requests per client IP per minute (0 = off).
	RpclimitPerMin int `json:"rpclimit,omitempty"`
	// RpcAuthMaxFail caps failed HTTP Basic attempts per IP per minute when auth is on (0 = default 30; -1 = off).
	RpcAuthMaxFail int `json:"rpcauthmaxfail,omitempty"`
	// NodeMode is "full" (headers + optional raw block payloads) or "spv" (headers-first only; no raw block store).
	NodeMode string `json:"node_mode,omitempty"`
	// P2PConnectivity selects classic (inbound listen + outbound), cgnat (outbound-only multi-peer), or both (default).
	P2PConnectivity string `json:"p2p_connectivity,omitempty"`
	// Firewall is auto|always|never for OS firewall rules on the P2P port and binary (default auto when omitted).
	Firewall string `json:"firewall,omitempty"`
	// Upnp is auto|enable|disable for UPnP/NAT-PMP port mapping when listening (Core -upnp; default auto).
	Upnp string `json:"upnp,omitempty"`
	// ZmqPub* are Core -zmqpub* PUB bind addresses (tcp://host:port or host:port).
	ZmqPubHashBlock string `json:"zmqpubhashblock,omitempty"`
	ZmqPubHashTx    string `json:"zmqpubhashtx,omitempty"`
	ZmqPubRawBlock  string `json:"zmqpubrawblock,omitempty"`
	ZmqPubRawTx     string `json:"zmqpubrawtx,omitempty"`
	// MaxOutbound caps total P2P sessions including the primary sync peer (0 = default 12).
	MaxOutbound int `json:"maxoutbound,omitempty"`
	// MaxInbound caps accepted inbound connections when listening (0 = default 16).
	MaxInbound int `json:"maxinbound,omitempty"`
	// BlockSyncWorkers is parallel TCP sessions for historical block download during IBD (0 = derive from maxoutbound).
	BlockSyncWorkers int `json:"block_sync_workers,omitempty"`
	// AddedNodes lists persistent addnode host:ports (Core addnode add).
	AddedNodes []string `json:"addnode,omitempty"`
	// DNSSeeds lists extra DNS seed hostnames merged with chain params at startup.
	DNSSeeds []string `json:"dnsseed,omitempty"`
	// DNSSeedLookup enables DNS seed queries (Core -dnsseed; default true when omitted).
	DNSSeedLookup *bool `json:"dnsseed_lookup,omitempty"`
	// AnalyticsSidecar when false disables the embedded Pebble analytics updater in dogego node (CLI indexer remains available).
	// Omit or true = enabled on full node runs.
	AnalyticsSidecar *bool `json:"analytics_sidecar,omitempty"`
	// IBDOptimize when true (default) prioritizes headers/block download during IBD: more assist peers,
	// deferred analytics sidecar until sync catches up, and UI “IBD focus” mode (Core-style).
	IBDOptimize *bool `json:"ibd_optimize,omitempty"`
	// DBCacheMB is Core -dbcache style UTXO working-set budget in megabytes (0 = auto from free RAM).
	DBCacheMB int `json:"dbcache,omitempty"`
	// AlertNotify runs a shell command when chain warnings change (Core -alertnotify); %s is the warning text.
	AlertNotify string `json:"alertnotify,omitempty"`
	// AssumeValid is Core -assumevalid block hash (empty = network default; "0" = verify all scripts).
	AssumeValid string `json:"assumevalid,omitempty"`
	// Checkpoints enables Core mapCheckpoints hash checks during header sync (default true when omitted).
	Checkpoints *bool `json:"checkpoints,omitempty"`
	// MaxTipAge is Core -maxtipage in seconds (0 = default 86400).
	MaxTipAge int `json:"maxtipage,omitempty"`
	// RpcTLSCert / RpcTLSKey enable TLS on the JSON-RPC listener (both required when either is set).
	RpcTLSCert string `json:"rpc_tls_cert,omitempty"`
	RpcTLSKey  string `json:"rpc_tls_key,omitempty"`
	// WebUITLSCert / WebUITLSKey enable TLS on the web dashboard listener.
	WebUITLSCert string `json:"webui_tls_cert,omitempty"`
	WebUITLSKey  string `json:"webui_tls_key,omitempty"`
	// WebUITLSLocal auto-generates HTTPS certs under datadir/tls/ for the web UI (no PEM paths required).
	WebUITLSLocal bool `json:"webui_tls_local,omitempty"`
	// RpcTLSLocal auto-generates HTTPS certs under datadir/tls/ for JSON-RPC.
	RpcTLSLocal bool `json:"rpc_tls_local,omitempty"`
	// LocalTLSTrustCA attempts to install the generated local CA into the OS user trust store on startup.
	LocalTLSTrustCA bool `json:"local_tls_trust_ca,omitempty"`
	// WebUIPINEnabled when true prompts for a 6-digit dashboard PIN (stored hashed in web_security.json).
	WebUIPINEnabled bool `json:"web_ui_pin_enabled,omitempty"`
	// WebUIRemoteAuth when true requires a valid dashboard PIN session for non-loopback /api reads
	// when webui binds beyond localhost/127.0.0.1 (LAN monitoring). PIN must be configured separately.
	WebUIRemoteAuth bool `json:"webui_remote_auth,omitempty"`
	// WebUIWebAuthn when true allows optional device biometric unlock after PIN is set (WebAuthn platform).
	WebUIWebAuthn bool `json:"web_ui_webauthn,omitempty"`
	// DogeGoRelayCGNAT configures integrated QUIC reachability relay (NODE_DOGEGO_RELAY_CGNAT).
	DogeGoRelayCGNAT DogeGoRelayCGNAT `json:"dogego_relay_cgnat,omitempty"`
	// CoreRPCAddr is Dogecoin Core JSON-RPC host:port for live parity probes (Features / Mempool tabs).
	CoreRPCAddr string `json:"core_rpc_addr,omitempty"`
	// CoreRPCUser / CoreRPCPassword authenticate Core JSON-RPC for parity probes (empty = DogeGo rpc_user/password).
	CoreRPCUser     string `json:"core_rpc_user,omitempty"`
	CoreRPCPassword string `json:"core_rpc_password,omitempty"`
	// SignerCmd runs an HWI-compatible external signer (stdin/stdout JSON), e.g. "python hwi.py --chain dogecoin --stdin".
	SignerCmd string `json:"signer_cmd,omitempty"`
	// Autostart is login to register OS sign-in autostart (Windows Task Scheduler, Linux systemd/XDG, macOS LaunchAgent).
	Autostart string `json:"autostart,omitempty"`
}

// AutostartOnLogin reports whether the node should start automatically at OS user login.
func (f File) AutostartOnLogin() bool {
	return strings.ToLower(strings.TrimSpace(f.Autostart)) == "login"
}

// WebUIHTTPS reports whether the dashboard listener uses TLS (explicit PEM or auto local HTTPS).
func (f File) WebUIHTTPS() bool {
	return f.WebUITLSLocal || strings.TrimSpace(f.WebUITLSCert) != ""
}

// DNSSeedLookupEnabled reports whether to query chain DNS seeds (Core -dnsseed default on).
func (f File) DNSSeedLookupEnabled() bool {
	if f.DNSSeedLookup != nil && !*f.DNSSeedLookup {
		return false
	}
	return true
}

// EmbeddedAnalyticsEnabled reports whether node/spvnode should run the background analytics sidecar.
func (f File) EmbeddedAnalyticsEnabled() bool {
	if f.AnalyticsSidecar != nil && !*f.AnalyticsSidecar {
		return false
	}
	return true
}

// IBDOptimizeEnabled reports whether dynamic IBD prioritization is on (default true).
func (f File) IBDOptimizeEnabled() bool {
	if f.IBDOptimize != nil && !*f.IBDOptimize {
		return false
	}
	return true
}

// EffectiveDBCacheMB resolves dbcache for this file (0 = auto; pass freeMB from the host or -1).
func (f File) EffectiveDBCacheMB(freeMB int64) int {
	return EffectiveDBCacheMB(f.DBCacheMB, freeMB)
}

// SearchPaths returns candidate paths to try when loading (first existing file wins).
// User config dir is preferred over ./dogecoinconf.json so a stray empty file in cwd does not override AppData settings.
func SearchPaths() []string {
	var out []string
	if e := os.Getenv("DOGECOINCONF"); e != "" {
		out = append(out, e)
	}
	for _, dir := range userConfigSearchDirs() {
		out = append(out, filepath.Join(dir, FileName))
	}
	if exe, err := os.Executable(); err == nil {
		out = append(out, filepath.Join(filepath.Dir(exe), FileName))
	}
	out = append(out, filepath.Join(".", FileName))
	return out
}

// LoadFirst reads the first existing config file from SearchPaths.
func LoadFirst() (f File, path string) {
	for _, p := range SearchPaths() {
		c, err := LoadFile(p)
		if err != nil {
			continue
		}
		return c, p
	}
	return File{}, ""
}

// LoadFile reads one dogecoinconf.json path.
func LoadFile(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var c File
	if err := json.Unmarshal(b, &c); err != nil {
		return File{}, err
	}
	normalizeLegacyUserAgent(&c, b)
	return c, nil
}

// normalizeLegacyUserAgent maps deprecated "useragent" JSON into uacomment when present.
func normalizeLegacyUserAgent(f *File, raw []byte) {
	var aux struct {
		Legacy string `json:"useragent"`
	}
	if err := json.Unmarshal(raw, &aux); err != nil || aux.Legacy == "" {
		return
	}
	if f.UAComment != "" {
		return
	}
	f.UAComment = aux.Legacy
}

// userConfigSearchDirs returns candidate config directories (preferred first).
func userConfigSearchDirs() []string {
	cd, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(cd, AppConfigDirName),
		filepath.Join(cd, legacyAppConfigDirName),
	}
}

// PreferredSaveDir is the directory used for a new config when none was loaded.
func PreferredSaveDir() (string, error) {
	dirs := userConfigSearchDirs()
	if len(dirs) == 0 {
		return ".", nil
	}
	d := dirs[0]
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	return d, nil
}

// ResolveSavePath picks where to write: same file if we loaded one, else user config dir.
func ResolveSavePath(loadedFrom string) (string, error) {
	if loadedFrom != "" {
		return loadedFrom, nil
	}
	dir, err := PreferredSaveDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// Save writes f to path (creates parent directories).
func Save(path string, f File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}
