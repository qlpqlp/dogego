// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"encoding/json"
	"math/big"
	"strings"

	"dogego/applog"
	"dogego/chain"
	"dogego/consensus"
	"dogego/extensions"
	"dogego/mempool"
	"dogego/rpc"
	"dogego/store"
	"dogego/wallet"
)

type earlyChainRPCEnv struct {
	Cfg              Config
	RuntimeSvc       *RuntimeServices
	EarlyRPC         *rpc.EarlyServer
	ChainRPCPaths    **rpc.DataPaths
	UIRPCInvoke      *func(method string, params []json.RawMessage) map[string]interface{}
	ExtMgr           *extensions.Manager
	ChainName        string
	J                *store.HeaderJournal
	AuxJ             *store.HeaderAuxJournal
	Pool             *mempool.Pool
	RbStore          *store.RawBlockStore
	TxIx             *store.TxIndex
	FilterIx         *store.BlockFilterIndex
	UtxoCache        *store.UtxoCache
	BlockStore       *BlockStoreCtx
	TipWait          *rpc.TipWaiter
	RawFill          *progressiveRawState
	FeeHistory       *consensus.FeeHistory
	ChainWorkCache   *ChainWorkCache
	ChainRoot        string
	BaseDataAbs      string
	ChainDataAbs     string
	AnalyticsOn      bool
	BanMgr           *rpc.FileBanManager
	Orphans          *mempool.OrphanPool
	PeerFeeFilters   *FeeFilterSet
	ContiguousForUI  func() int64
	HeaderCatchUpPending func() bool
	SaveUtxoShutdown     func()
	Disk                 *wallet.Disk
	WIFVer               byte
	PKHVer               byte
	SHVer                byte
}

// activateEarlyChainRPC wires getblockchaininfo/stop/saveutxosnapshot before blocking P2P setup.
func activateEarlyChainRPC(env earlyChainRPCEnv) {
	if env.ChainRPCPaths == nil || env.RuntimeSvc == nil {
		return
	}
	if *env.ChainRPCPaths != nil && env.RuntimeSvc.RPCDispatchReady() {
		return
	}
	if !chainRPCPathsNeeded(env.Cfg) {
		return
	}
	paths := &rpc.DataPaths{
		BaseDataDir:              env.BaseDataAbs,
		ChainDataDir:             env.ChainDataAbs,
		Extensions:               env.ExtMgr,
		MaxTipAgeSec:             chain.EffectiveMaxTipAge(env.Cfg.MaxTipAge),
		HeaderAux:                env.AuxJ,
		Utxo:                     env.UtxoCache,
		TipWaiter:                env.TipWait,
		EmbeddedAnalyticsSidecar: env.AnalyticsOn,
		FeeFilter:                env.PeerFeeFilters.Load,
		OrphanCount: func() int {
			if env.Orphans == nil {
				return 0
			}
			return env.Orphans.Count()
		},
		MaxMempoolEntries:  env.Pool.MaxMempoolLimitBytes(),
		MempoolExpiryHours: func() int { return env.Pool.ExpiryHours() },
		FullRBF:            func() bool { return env.Cfg.FullRBF },
		Standard: func() consensus.StandardPolicy {
			return env.Cfg.Standard
		},
		MempoolLimits: func() consensus.MempoolRelayLimits {
			return env.Cfg.MempoolLimits
		},
		MempoolMinRelayFee: func() uint64 {
			return env.Pool.MinRelayFeePerKB()
		},
		FeeHistory:          env.FeeHistory,
		ContiguousRawHeight: env.ContiguousForUI,
		CumulativeChainWork: func(through int64) (*big.Int, bool) {
			return env.ChainWorkCache.LookupThrough(env.J, through)
		},
		ChainWorkCacheReady: env.ChainWorkCache.Ready,
		BlockFilterIndex:    env.FilterIx,
		StorageSummary: nativeStorageSummary(env.ChainRoot, env.RbStore, env.TxIx, env.ContiguousForUI),
		SyncUtxo: func() error {
			if env.BlockStore != nil {
				return env.BlockStore.SyncUtxoCacheBounded(rpcSyncUtxoMaxBlocks)
			}
			return nil
		},
		SyncUtxoBounded: func(maxBlocks int) error {
			if env.BlockStore != nil {
				return env.BlockStore.SyncUtxoCacheBounded(maxBlocks)
			}
			return nil
		},
		UtxoConnectInFlight: UtxoConnectInFlight,
		FilterIndexThrough:  FilterIndexThrough,
		BanManager: env.BanMgr,
	}
	if env.HeaderCatchUpPending != nil {
		paths.HeaderCatchUpPending = env.HeaderCatchUpPending
	}
	if env.Disk != nil {
		wireWalletHD(paths, env.Disk, env.WIFVer, env.PKHVer, env.SHVer)
		wireWalletEncryption(paths, env.Disk)
	}
	if env.Cfg.FullNode && env.RbStore != nil && env.BlockStore != nil && env.RawFill != nil {
		paths.RawSyncProgress = func() map[string]interface{} {
			snap := env.RawFill.snapshot()
			enrichIBDProgressSnapshot(snap, env.J, env.BlockStore)
			enrichAssistDiagnosticsAuto(snap)
			if tip, err := env.J.TipHeight(); err == nil && tip >= 0 {
				snap["headers_tip"] = tip
			}
			return snap
		}
	}
	if env.Cfg.Stop != nil {
		stopFn := env.Cfg.Stop
		paths.Shutdown = func() {
			stopFn()
		}
	}
	*env.ChainRPCPaths = paths
	if env.Cfg.RPCAddr == "" && strings.TrimSpace(env.Cfg.WebUIAddr) == "" {
		return
	}
	relay := func([]byte) error { return nil }
	if env.Cfg.RPCAddr != "" && env.EarlyRPC != nil {
		env.EarlyRPC.Activate(rpc.HandlerCore(env.ChainName, env.J, env.Pool, paths, env.RbStore, env.TxIx, relay, env.Cfg.AllowUnverifiedMempool))
		env.RuntimeSvc.SetRPCDispatchReady(true)
		applog.Line("rpc", "JSON-RPC dispatch ready on "+env.Cfg.RPCAddr+" (pre-P2P)")
	}
	if env.UIRPCInvoke != nil && strings.TrimSpace(env.Cfg.WebUIAddr) != "" {
		*env.UIRPCInvoke = func(method string, params []json.RawMessage) map[string]interface{} {
			return rpc.Dispatch(env.ChainName, env.J, env.Pool, paths, env.RbStore, env.TxIx, relay, env.Cfg.AllowUnverifiedMempool, method, params, json.RawMessage(`1`))
		}
	}
}
