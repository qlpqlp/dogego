// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"dogego/consensus"
	"dogego/httptls"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wallet"
	"dogego/wallet/corewallet"
)

// resolveGetBlockHeader parses getblockheader params (same rules as RPC): empty = chainActive tip, else height or hash string.
func resolveGetBlockHeader(j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage, paths ...*DataPaths) (h80 []byte, height int64, err error) {
	if len(params) == 0 {
		height, _, _ = activeChainFromJournal(j, raw, paths...)
		h80, err = j.ReadHeaderAt(height)
		return h80, height, err
	}
	var p0 interface{}
	if err := json.Unmarshal(params[0], &p0); err != nil {
		return nil, 0, fmt.Errorf("bad param")
	}
	switch v := p0.(type) {
	case float64:
		if v < 0 || v > float64(math.MaxInt64) || v != float64(int64(v)) {
			return nil, 0, fmt.Errorf("invalid height")
		}
		height = int64(v)
		h80, err = j.ReadHeaderAt(height)
		return h80, height, err
	case string:
		s := strings.TrimSpace(v)
		if len(s) == 64 || (len(s) == 66 && strings.HasPrefix(strings.ToLower(s), "0x")) {
			height, err = j.HeightByDisplayHash(s)
			if err != nil {
				return nil, 0, err
			}
			h80, err = j.ReadHeaderAt(height)
			return h80, height, err
		}
		if fv, err2 := strconv.ParseFloat(s, 64); err2 == nil && fv >= 0 && fv == float64(int64(fv)) {
			height = int64(fv)
			h80, err = j.ReadHeaderAt(height)
			return h80, height, err
		}
		return nil, 0, fmt.Errorf("want height or 64-char block hash hex")
	default:
		return nil, 0, fmt.Errorf("unsupported param type")
	}
}

// Journal is the minimal header store surface for chain info RPC.
type Journal interface {
	TipHeight() (int64, error)
	BestBlockHashHex() (string, error)
	GenesisHashHex() (string, error)
	Count() (int64, error)
}

// HeaderJournal adds raw header access for block-style RPC (implemented by *store.HeaderJournal).
type HeaderJournal interface {
	Journal
	ReadHeaderAt(height int64) ([]byte, error)
	HeightByDisplayHash(displayHex string) (int64, error)
}

func blockHeaderJSON(j HeaderJournal, h80 []byte, height int64, chainworkHex string, aux *store.HeaderAuxJournal, confirmTip int64) map[string]interface{} {
	t := int64(binary.LittleEndian.Uint32(h80[68:72]))
	mediantime := t
	if m, err := headerMedianTimePast(j, height); err == nil {
		mediantime = m
	}
	bitsU := binary.LittleEndian.Uint32(h80[72:76])
	diff := float64(0)
	if d, err := pow.DifficultyFromCompact(bitsU); err == nil {
		diff = d
	}
	conf := int64(1)
	if height >= 0 && confirmTip >= height {
		conf = confirmTip - height + 1
	}
	nextHash := interface{}(nil)
	if height >= 0 {
		if nh80, err := j.ReadHeaderAt(height + 1); err == nil {
			nextHash = pow.BlockHashHex(nh80)
		}
	}
	out := map[string]interface{}{
		"hash":              pow.BlockHashHex(h80),
		"confirmations":     conf,
		"height":            height,
		"version":           int32(binary.LittleEndian.Uint32(h80[0:4])),
		"versionHex":        fmt.Sprintf("%08x", binary.LittleEndian.Uint32(h80[0:4])),
		"merkleroot":        pow.LEUint256DisplayHex(h80[36:68]),
		"time":              t,
		"mediantime":        mediantime,
		"nonce":             binary.LittleEndian.Uint32(h80[76:80]),
		"bits":              fmt.Sprintf("%08x", bitsU),
		"difficulty":        diff,
		"chainwork":         chainworkHex,
		"previousblockhash": pow.LEUint256DisplayHex(h80[4:36]),
		"nextblockhash":     nextHash,
	}
	attachAuxPowField(out, nil, aux, height)
	return out
}

// WalletPrunedImport is a receive credited via importprunedfunds.
type WalletPrunedImport struct {
	TxID        string
	BlockHeight int64
	BlockHash   string
	Vout        uint32
	AmountKoinu int64
	Script      []byte
}

// WalletAddressEntry is a tracked wallet address row for dogego_listwalletaddresses.
type WalletAddressEntry struct {
	Address            string `json:"address"`
	HDPath             string `json:"hdpath,omitempty"`
	IsChange           bool   `json:"ischange,omitempty"`
	Label              string `json:"label,omitempty"`
	WatchOnly          bool   `json:"watchonly,omitempty"`
	IsCosigner         bool   `json:"cosigner,omitempty"`
	IsKeypool          bool   `json:"iskeypool,omitempty"`
	IsNodeTip          bool   `json:"isnodetip,omitempty"`
	HDKeypoolCoreIndex *int64 `json:"hd_keypool_core_index,omitempty"`
}

// DataPaths optional absolute paths shown in getblockchaininfo (DogeGo per-network layout).
type DataPaths struct {
	BaseDataDir  string
	ChainDataDir string
	// FeeFilter, when set, returns the last inbound P2P feefilter rate (koinu per kB, BIP133-style wire integer).
	FeeFilter func() uint64
	// Uptime returns whole seconds since the node process started (for uptime RPC).
	Uptime func() int64
	// RPCTLSEnabled reports native TLS on the JSON-RPC listener (getrpcinfo dogego_rpc_tls).
	RPCTLSEnabled bool
	// NetRecv / NetSent return cumulative TCP bytes on the outbound P2P link (for getnettotals), if wired.
	NetRecv func() int64
	NetSent func() int64
	// Shutdown, when set, is invoked asynchronously after a successful JSON-RPC "stop" (Core-style).
	Shutdown func()
	// PeerInfo, when set, returns getpeerinfo-style maps for the active P2P session (typically one outbound).
	PeerInfo func() []map[string]interface{}
	// AddedNodes returns host:port strings for persistent addnode targets (excluding onetry); used by getaddednodeinfo.
	AddedNodes func() []string
	// IsPeerConnected reports whether a host:port has an active P2P session (getaddednodeinfo connected field).
	IsPeerConnected func(addr string) bool
	// NodeAddresses returns Core getnodeaddresses rows (count 0 = all known; optional ipv4/ipv6/onion filter).
	NodeAddresses func(count int, network string) []map[string]interface{}
	// AddrManInfo returns Core getaddrmaninfo-style addrbook summary when P2P is active.
	AddrManInfo func() map[string]interface{}
	// PeerAddresses returns Core-shaped address rows for a connected addnode session (may be nil).
	PeerAddresses func(addedNode string) []interface{}
	// AddNode handles addnode RPC (commands add, remove, onetry) when wired from the P2P layer.
	AddNode func(node string, command string) error
	// DisconnectNode disconnects the peer matching address (host:port) when wired.
	DisconnectNode func(address string) error
	// BanManager backs setban / listbanned / clearbanned (e.g. *MemoryBanManager).
	BanManager BanManager
	// BanDisconnect closes relay peers matching a new ban (optional; Core disconnects on setban add).
	BanDisconnect func()
	// PingPeers queues immediate outbound P2P pings on all connected peers (Core ping RPC).
	PingPeers func()
	// SetMaxConnections applies a new global max connection target when wired (Core setmaxconnections).
	SetMaxConnections func(max int) error
	// SetNetworkActive enables or disables P2P networking; returns the resulting networkactive flag (Core setnetworkactive).
	SetNetworkActive func(active bool) (networkActive bool, err error)
	// NetworkActive reports whether P2P is enabled for getnetworkinfo.networkactive when wired.
	NetworkActive func() bool
	// LocalP2P returns this process's protocol version, user-agent string, and NODE_* service bits for getnetworkinfo.
	LocalP2P func() (protocolVersion int32, userAgent string, services uint64)
	// P2PStats returns live connectivity fields (mode, health, connection counts) when wired.
	P2PStats func() map[string]interface{}
	// MedianPeerTimeOffset returns median peer version nTime offset (Core getnetworkinfo / getinfo timeoffset).
	MedianPeerTimeOffset func() int32
	// MaxTipAgeSec is Core -maxtipage for initialblockdownload (0 = use 86400).
	MaxTipAgeSec int64
	// ContiguousRawHeight returns cached highest contiguous stored block height (-1 if unknown).
	// Wired from BlockStoreCtx during sync so getblockchaininfo and the dashboard avoid O(tip) scans.
	ContiguousRawHeight func() int64
	// UtxoBodiesAligned reports whether stored raw bodies cover the in-memory UTXO tip (snapshot-safe).
	UtxoBodiesAligned func() bool
	// CumulativeChainWork returns cached total chain work through height when available (avoids reading all headers.bin).
	CumulativeChainWork func(through int64) (*big.Int, bool)
	// ChainWorkCacheReady false while the node is still warming the chain-work cache at startup.
	ChainWorkCacheReady func() bool
	// EmbeddedAnalyticsSidecar true when this process runs the Pebble analytics sidecar (see analytics.RunSidecar).
	EmbeddedAnalyticsSidecar bool
	// HeaderAux optional parallel auxpow store (headers_aux.bin); used by verifychain checklevel >=3.
	HeaderAux *store.HeaderAuxJournal
	// OrphanCount returns in-memory orphan transaction count when wired from the P2P layer.
	OrphanCount func() int
	// OrphanPool is the P2P orphan store; when set, sendrawtransaction may queue missing-parent txs (RPC returns Missing inputs).
	OrphanPool consensus.OrphanStore
	// MaxOrphanEntries is the configured orphan pool capacity (0 = use DogeGo default).
	MaxOrphanEntries int
	// MaxMempoolEntries is the configured mempool byte cap for getmempoolinfo (Core -maxmempool).
	MaxMempoolEntries int
	// MempoolExpiryHours returns configured mempool max age in hours (Core -mempoolexpiry).
	MempoolExpiryHours func() int
	// RPCWhitelist when non-nil restricts callable JSON-RPC methods (see rpcwhitelist in config).
	RPCWhitelist RPCWhitelist
	// FullRBF reports whether mempoolfullrbf policy is enabled (getmempoolinfo.fullrbf).
	FullRBF func() bool
	// Standard returns mempool relay standardness policy (dust, OP_RETURN, bare multisig).
	Standard func() consensus.StandardPolicy
	// MempoolLimits returns maxtxfee and package limits for admission.
	MempoolLimits func() consensus.MempoolRelayLimits
	// MempoolAdmissionView overrides prevout resolution (differential integration tests only).
	MempoolAdmissionView consensus.PrevOutView
	// MempoolAdmissionIndex overrides tx index for maturity checks (differential integration tests only).
	MempoolAdmissionIndex consensus.TxIndexer
	// MempoolAdmissionJournal overrides header chain for maturity checks (differential integration tests only).
	MempoolAdmissionJournal consensus.HeaderChain
	// MempoolFeeEstimate returns a mempool-based feerate hint for estimatesmartfee (koinu/kB; nblocks target).
	MempoolFeeEstimate func(nblocks int) uint64
	// MempoolFeeEstimateConservative returns the max mempool feerate when prevouts resolve.
	MempoolFeeEstimateConservative func(nblocks int) uint64
	// MempoolFeeEstimateEconomical returns a low mempool feerate percentile when prevouts resolve.
	MempoolFeeEstimateEconomical func(nblocks int) uint64
	// ConfirmedFeeEstimate returns feerates from recently connected blocks (koinu/kB).
	ConfirmedFeeEstimate func(nblocks int) uint64
	// ConfirmedFeeEstimateConservative returns a high confirmed feerate hint (Core conservative mode).
	ConfirmedFeeEstimateConservative func(nblocks int) uint64
	// FeeBucketEstimates returns DOGE/kB estimates keyed by confirmation target blocks (from fee history).
	FeeBucketEstimates func() map[string]float64
	// FeeBucketMarketStats returns per-target samples/medians (Core fee market subset).
	FeeBucketMarketStats func() map[string]map[string]interface{}
	// MempoolConfirmBucketStats returns feerates bucketed by blocks-to-confirm from our mempool.
	MempoolConfirmBucketStats func() map[string]map[string]interface{}
	// MempoolLeftBucketStats returns feerates bucketed by blocks-in-mempool when txs left unconfirmed.
	MempoolLeftBucketStats func() map[string]map[string]interface{}
	// FeeConfirmStatsBucketMarket returns exponential feerate bucket estimates (Core TxConfirmStats subset).
	FeeConfirmStatsBucketMarket func() map[string]map[string]interface{}
	// FeeHistory is the in-process fee estimator state (optional).
	FeeHistory *consensus.FeeHistory
	// HeaderTipHeight returns the current header chain tip height (-1 if unknown).
	HeaderTipHeight func() int64
	// HeaderSyncRecoveryHint returns operator recovery text when header sync is recovering (bad nBits, background catch-up).
	HeaderSyncRecoveryHint func() string
	// HeaderCatchUpPending true while headers retry in background (block-assist may still run).
	HeaderCatchUpPending func() bool
	// BlockAssistWorkersActive reports parallel block-download workers during IBD.
	BlockAssistWorkersActive func() bool
	// MempoolMinRelayFee returns the rolling minimum relay feerate (koinu/kB) after evictions.
	MempoolMinRelayFee func() uint64
	// BlockMaxWeight is the mining template weight limit (0 = consensus.MaxBlockWeight).
	BlockMaxWeight int
	// RelayBlock sends a P2P block message after submitblock when wired from the node.
	RelayBlock func([]byte) error
	// ConnectSubmittedBlock runs ConnectBlock + UTXO apply for submitblock (requires tx index).
	ConnectSubmittedBlock func(payload []byte, height int64) error
	// RawSyncProgress returns progressive raw-block download state (DogeGo full node).
	RawSyncProgress func() map[string]interface{}
	// Utxo is the in-memory UTXO set at the connected chain tip (full node with tx index).
	Utxo *store.UtxoCache
	// SyncUtxo refreshes the UTXO cache through the contiguous chain tip when wired (gettxoutsetinfo).
	SyncUtxo func() error
	// SyncUtxoBounded advances chainActive by at most N blocks (RPC syncutxo; avoids blocking during IBD).
	SyncUtxoBounded func(maxBlocks int) error
	// UtxoConnectInFlight true while ConnectBlock replay holds the connect mutex.
	UtxoConnectInFlight func() bool
	// FilterIndexThrough returns highest BIP158 basic filter height indexed during catch-up (-1 if unknown).
	FilterIndexThrough func() int64
	// RecoverHeaderJournal rewinds stale header journal data (dogego_recoverheaders / web UI).
	RecoverHeaderJournal func() (tipBefore, tipAfter int64, rewound bool, err error)
	// RestartHeaderSyncIfStuck retries background header sync when the journal did not change (operator nudge).
	RestartHeaderSyncIfStuck func() bool
	// TruncateToHeight removes chain data above height (truncatetoheight operator RPC).
	TruncateToHeight func(height int64) error
	// InvalidateBlock disconnects before hash and marks block(s) invalid (Core invalidateblock).
	InvalidateBlock func(displayHash string) error
	// ReconsiderBlock clears invalid status (Core reconsiderblock).
	ReconsiderBlock func(displayHash string) error
	// MarkPreciousBlock sets equal-work reorg preference (Core preciousblock).
	MarkPreciousBlock func(displayHash string) error
	// TipWaiter receives best-chain tip updates for waitfor* RPCs.
	TipWaiter *TipWaiter
	// MiningAddress is the P2PKH payout for generate / optional solo mining (wallet or config).
	MiningAddress string
	// WalletAddress returns the built-in testnet wallet P2PKH address when enabled (empty otherwise).
	WalletAddress func() string
	// WalletDefaultAddress returns the primary receive address (BIP44 index 0 when HD).
	WalletDefaultAddress func() string
	// WalletPeekReceiveAddress returns the next keypool receive address without issuing it (getaccountaddress).
	WalletPeekReceiveAddress func() string
	// WalletPeekChangeAddress returns the next keypool change address without issuing it (fundrawtransaction).
	WalletPeekChangeAddress func() string
	// WalletCommitChangeAddress consumes the peeked change slot after a funded change output is added.
	WalletCommitChangeAddress func(addr string) error
	// WalletNewAddress returns a new receive address (getnewaddress / BIP44 external).
	WalletNewAddress func() (string, error)
	// WalletNewChangeAddress returns a new change address (getrawchangeaddress).
	WalletNewChangeAddress func() (string, error)
	// WalletSpendScripts returns all spendable P2PKH scripts (HD receive + change + legacy).
	WalletSpendScripts func() [][]byte
	// WalletContainsAddress reports whether addr belongs to this wallet (HD or default).
	WalletContainsAddress func(addr string) bool
	// WalletWIFForAddress returns the WIF for a known address (dumpprivkey).
	WalletWIFForAddress func(addr string) (string, error)
	// WalletKeypoolRefill extends the HD receive keypool (keypoolrefill).
	WalletKeypoolRefill func(newSize int) error
	// WalletReplayCorePool reserves matched Core pool pubkeys into the HD receive keypool.
	WalletReplayCorePool func(entries []corewallet.PoolEntry) (wallet.PoolReplayResult, error)
	// WalletHDFormat reports wallet.json format string ("hd" or "").
	WalletHDFormat func() string
	// WalletKeypoolSize returns unused HD receive indices in the keypool.
	WalletKeypoolSize func() int
	// WalletChangeKeypoolSize returns unused HD change indices in the internal keypool.
	WalletChangeKeypoolSize func() int
	// WalletHDKeypoolCoreIndex returns receive→Core pool index rows persisted after import.
	WalletHDKeypoolCoreIndex func() []wallet.HDKeypoolCoreIndexEntry
	// WalletAddressInReceiveKeypool reports unused HD receive keypool membership.
	WalletAddressInReceiveKeypool func(addr string) bool
	// WalletAddressInChangeKeypool reports unused HD change keypool membership.
	WalletAddressInChangeKeypool func(addr string) bool
	// WalletAddressIsNodeTip reports the dedicated node-tip HD key at m/44'/3'/0'/2/0.
	WalletAddressIsNodeTip func(addr string) bool
	// WalletAddressCorePoolIndex returns Core BDB pool index for a receive address.
	WalletAddressCorePoolIndex func(addr string) (int64, bool)
	// WalletListDescriptors returns descriptor rows for listdescriptors.
	WalletListDescriptors func(chainName string) []WalletDescriptorRow
	// WalletAvoidReuse reports the avoid_reuse wallet flag.
	WalletAvoidReuse func() bool
	// WalletSetAvoidReuse sets avoid_reuse (setwalletflag).
	WalletSetAvoidReuse func(bool) error
	// WalletPqCommitmentsEnabled reports pq_commitments_enabled (OP_RETURN PQ outputs on sends).
	WalletPqCommitmentsEnabled func() bool
	// WalletSetPqCommitmentsEnabled sets pq_commitments_enabled (setwalletflag).
	WalletSetPqCommitmentsEnabled func(bool) error
	// WalletPqCarrierEnabled reports pq_carrier_enabled (TX_C/TX_R carrier sends).
	WalletPqCarrierEnabled func() bool
	// WalletSetPqCarrierEnabled sets pq_carrier_enabled (setwalletflag).
	WalletSetPqCarrierEnabled func(bool) error
	// WalletPQCarrierKeyMaterial returns PQ pk/sk for carrier signing.
	WalletPQCarrierKeyMaterial func(tag string) (opTag string, pk, sk []byte, err error)
	// WalletNextPQCommit returns the next auto OP_RETURN PQ commitment for a wallet send.
	WalletNextPQCommit func() (tag string, commitmentHex string, err error)
	// WalletIsScriptReused reports whether a scriptPubKey received funds (avoid_reuse index).
	WalletIsScriptReused func(pkScript []byte) bool
	// WalletRebuildUsedRecvScripts rebuilds the avoid_reuse receive-script index from scan history.
	WalletRebuildUsedRecvScripts func()
	// WalletAddImportedDescriptor persists importdescriptors metadata (timestamp, internal).
	WalletAddImportedDescriptor func(desc string, timestamp int64, internal, spendable bool) error
	// BlockFilterIndex holds persisted BIP158 basic filters (filters/basic/).
	BlockFilterIndex *store.BlockFilterIndex
	// StorageSummary returns native layout fields for getblockchaininfo / dashboard.
	StorageSummary func() map[string]interface{}
	// WalletKnownAddresses returns all tracked P2PKH/P2SH addresses (HD issued, watch, labels).
	WalletKnownAddresses func() []string
	// WalletAddressHDPath returns BIP44 hdkeypath and ischange for addr when HD is enabled.
	WalletAddressHDPath func(addr string) (hdpath string, ischange bool, ok bool)
	// WalletMasterKeyFingerprint returns BIP32 master key fingerprint when HD is enabled.
	WalletMasterKeyFingerprint func() (uint32, bool)
	// WalletCompressedPubKeyForAddress returns compressed pubkey for a known wallet address.
	WalletCompressedPubKeyForAddress func(addr string) ([]byte, bool)
	// CoreRPCAddr is Dogecoin Core JSON-RPC host:port for migration/parity (core_rpc_addr in config).
	CoreRPCAddr string
	CoreRPCUser     string
	CoreRPCPassword string
	// SignerCommand is argv for an HWI-compatible external signer (signer_cmd in config).
	SignerCommand []string
	// WalletHDSeedID returns Core-shaped hdseedid (SHA256 of HD seed) when unlocked.
	WalletHDSeedID func() string
	// WalletIsEncrypted reports wallet.json encrypted at rest.
	WalletIsEncrypted func() bool
	// WalletIsUnlocked reports spend keys are loaded.
	WalletIsUnlocked func() bool
	// WalletUnlockUntil is Unix seconds when auto-lock fires (0 if none).
	WalletUnlockUntil func() int64
	// WalletEncrypt encrypts keys at rest (encryptwallet).
	WalletEncrypt func(passphrase string) (string, error)
	// WalletUnlock decrypts keys for timeoutSec (walletpassphrase).
	WalletUnlock func(passphrase string, timeoutSec int64) error
	// WalletLock clears keys from memory (walletlock).
	WalletLock func() error
	// WalletChangePassphrase re-encrypts with a new passphrase.
	WalletChangePassphrase func(oldPass, newPass string) error
	// WalletRescanBlocks scans raw blocks from startHeight for wallet scripts (rescan).
	WalletRescanBlocks func(startHeight int64) error
	// WalletIsScanning reports an in-flight wallet block rescan (getwalletinfo scanning).
	WalletIsScanning func() bool
	// WalletMaxScannedBlockHeight returns the highest block height in wallet scan history (-1 if none).
	WalletMaxScannedBlockHeight func() int64
	// WalletListScannedTx returns persisted block-scan rows from the last rescan.
	WalletListScannedTx func() []wallet.ScannedTx
	// WalletSendFeeLookup returns persisted send fee from wallet block scan (compact tx index fast path).
	WalletSendFeeLookup func(txid string) (int64, bool)
	// WalletRememberTxHex persists signed tx hex for gettransaction without loading blocks.
	WalletRememberTxHex func(txid, hexStr string) error
	// WalletTxHexLookup returns persisted wallet tx hex (compact tx index fast path).
	WalletTxHexLookup func(txid string) (string, bool)
	// WalletWIF returns the wallet private key in WIF when enabled.
	WalletWIF func() string
	// WalletWIFs returns spend + cosigner WIFs when enabled (multisig signing).
	WalletWIFs func() []string
	// WalletP2PKHScript returns the wallet's scriptPubKey bytes when enabled.
	WalletP2PKHScript func() []byte
	// WalletImportPrivKey replaces the built-in wallet key when enabled.
	WalletImportPrivKey func(wif string) error
	// WalletImportSpendKey replaces the spend key from dumpwallet lines (importwallet).
	WalletImportSpendKey func(wif string) error
	// WalletImportMnemonic restores HD wallet from BIP39 mnemonic (dogego_importmnemonic).
	WalletImportMnemonic func(mnemonic, passphrase string) error
	// WalletImportBIP38 decrypts BIP38 and sweeps to spend key; returns new address.
	WalletImportBIP38 func(encrypted, passphrase string) (address string, err error)
	// WalletListAddresses returns tracked addresses for the web UI.
	WalletListAddresses func() []WalletAddressEntry
	// WalletImportWatch adds a watch-only scriptPubKey (importaddress).
	WalletImportWatch func(script []byte) error
	// WalletSetWatchRedeem stores redeemScript for a watch P2SH scriptPubKey (multisig imports).
	WalletSetWatchRedeem func(scriptPubKey, redeem []byte) error
	// WalletWatchRedeemScript returns redeemScript hex bytes for a watch P2SH scriptPubKey, or nil.
	WalletWatchRedeemScript func(scriptPubKey []byte) []byte
	// WalletWatchScripts returns imported watch-only scripts when enabled.
	WalletWatchScripts func() [][]byte
	// WalletIsWatchAddress reports whether addr is a watch-only import.
	WalletIsWatchAddress func(addr string) bool
	// WalletOwnsScript reports whether scriptPubKey belongs to the built-in wallet.
	WalletOwnsScript func(script []byte) bool
	// WalletImportPrunedReceive credits a proven receive (importprunedfunds).
	WalletImportPrunedReceive func(txid string, height int64, blockHash string, vout uint32, amountKoinu int64, script []byte) error
	// WalletListPrunedImports returns importprunedfunds credits for wallet transaction lists.
	WalletListPrunedImports func() []WalletPrunedImport
	// WalletRemovePrunedImport removes importprunedfunds records for txid (removeprunedfunds).
	WalletRemovePrunedImport func(txid string) bool
	// WalletPath returns the on-disk wallet.json path when the built-in wallet is enabled.
	WalletPath func() string
	// WalletPayTxFee returns the wallet fee rate in DOGE per kB (settxfee).
	WalletPayTxFee func() float64
	// WalletSetPayTxFee persists the wallet fee rate in DOGE per kB.
	WalletSetPayTxFee func(feeDOGEPerKB float64) error
	// WalletListLocked returns locked outpoints (lockunspent).
	WalletListLocked func() []wallet.LockedOutpoint
	// WalletSetLocked locks or unlocks outpoints (lockunspent).
	WalletSetLocked func(unlock bool, outs []wallet.LockedOutpoint) error
	// WalletIsLockedOutpoint reports whether an outpoint is locked.
	WalletIsLockedOutpoint func(txid string, vout uint32) bool
	// WalletGetLabel returns the setlabel name for an address (empty if unset).
	WalletGetLabel func(addr string) string
	// WalletSetLabel persists a label for a tracked address (empty removes).
	WalletSetLabel func(addr, label string) error
	// WalletListLabels returns sorted unique non-empty labels.
	WalletListLabels func() []string
	// WalletRecordTxReplacement persists bumpfee old->new txid mapping in wallet.json.
	WalletRecordTxReplacement func(oldTxid, newTxid string) error
	// WalletConflictsForTx returns persisted replacement conflicts for a display txid.
	WalletConflictsForTx func(txid string) []string
	// WalletAbandonTx persists an abandoned mempool transaction (abandontransaction).
	WalletAbandonTx func(txid, category, addr string, amountKoinu int64) error
	// WalletListAbandoned returns persisted abandoned transactions.
	WalletListAbandoned func() []wallet.AbandonedTx
	// WalletIsAbandoned reports whether txid was abandoned.
	WalletIsAbandoned func(txid string) bool
	// WalletRemoveAbandoned removes txid from the abandoned list (removeprunedfunds).
	WalletRemoveAbandoned func(txid string) bool
	// WalletRemoveReplacementsForTx clears bumpfee replacement map entries for txid.
	WalletRemoveReplacementsForTx func(txid string) error
	// ConnectionCount returns active P2P TCP sessions when wired (else RPC assumes 0/1).
	ConnectionCount func() int
	// ZmqNotifications returns Core getzmqnotifications rows when ZMQ PUB is active (nil = empty array).
	ZmqNotifications func() []map[string]interface{}
	// Extensions is the optional DogeGo extension host (list/install/enable; no wallet access).
	Extensions ExtensionsManager
}

// Handler returns the JSON-RPC HTTP handler (POST body: single object or batch array).
// pool may be nil (mempool_txs omitted from getblockchaininfo). paths, blocks, and txIndex may be nil.
// relayTx, when non-nil, is invoked with raw tx bytes after a successful sendrawtransaction mempool add
// (embedded node uses this to forward a P2P `tx` to the outbound peer). Errors from relayTx are ignored.
// allowUnverifiedMempool skips consensus.AcceptMempoolTx (coinbase + script checks); default false for full-node behavior.
// When auth is non-nil and auth.User is non-empty, HTTP Basic authentication is required on every request.
func Handler(chainName string, j HeaderJournal, pool *mempool.Pool, paths *DataPaths, blocks *store.RawBlockStore, txIndex *store.TxIndex, relayTx func([]byte) error, allowUnverifiedMempool bool, auth *RPCAuth) http.Handler {
	return wrapIfAuth(auth, HandlerCore(chainName, j, pool, paths, blocks, txIndex, relayTx, allowUnverifiedMempool))
}

// HandlerCore is the JSON-RPC handler without HTTP Basic auth (used when auth wraps an outer listener).
func HandlerCore(chainName string, j HeaderJournal, pool *mempool.Pool, paths *DataPaths, blocks *store.RawBlockStore, txIndex *store.TxIndex, relayTx func([]byte) error, allowUnverifiedMempool bool) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		trim := bytes.TrimSpace(body)
		if len(trim) > 0 && trim[0] == '[' {
			var batch []json.RawMessage
			if err := json.Unmarshal(body, &batch); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			out := make([]map[string]interface{}, len(batch))
			for i, rawMsg := range batch {
				var sub struct {
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
					ID     json.RawMessage   `json:"id"`
				}
				if err := json.Unmarshal(rawMsg, &sub); err != nil {
					out[i] = map[string]interface{}{
						"jsonrpc": "1.0",
						"id":      nil,
						"error":   map[string]interface{}{"code": -32700, "message": "parse error: " + err.Error()},
					}
					continue
				}
				out[i] = dispatchRequest(chainName, j, pool, paths, blocks, txIndex, relayTx, allowUnverifiedMempool, sub.Method, sub.Params, sub.ID)
			}
			_ = json.NewEncoder(w).Encode(out)
			return
		}

		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
			ID     json.RawMessage   `json:"id"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(dispatchRequest(chainName, j, pool, paths, blocks, txIndex, relayTx, allowUnverifiedMempool, req.Method, req.Params, req.ID))
	})
	return inner
}

// Serve starts a blocking JSON-RPC HTTP listener. When auth is enabled (see RPCAuth), clients must send Basic credentials.
// tls is optional PEM cert/key (Core rpcssl* analogue); empty serves plain HTTP.
func Serve(addr string, tls httptls.Pair, chainName string, j HeaderJournal, pool *mempool.Pool, paths *DataPaths, blocks *store.RawBlockStore, txIndex *store.TxIndex, relayTx func([]byte) error, allowUnverifiedMempool bool, auth *RPCAuth) error {
	ln, _, err := httptls.Listen(addr, tls)
	if err != nil {
		return err
	}
	return http.Serve(ln, Handler(chainName, j, pool, paths, blocks, txIndex, relayTx, allowUnverifiedMempool, auth))
}
