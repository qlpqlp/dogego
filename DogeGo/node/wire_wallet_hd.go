// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"sync"
	"time"

	"dogego/chain"
	"dogego/rpc"
	"dogego/store"
	"dogego/wallet"
	"dogego/wallet/corewallet"
)

func wireWalletHD(paths *rpc.DataPaths, disk *wallet.Disk, wifVer, pkhVer, shVer byte) {
	if paths == nil || disk == nil {
		return
	}
	paths.WalletDefaultAddress = func() string { return disk.DefaultAddress() }
	paths.WalletPeekReceiveAddress = func() string { return disk.PeekReceiveAddress() }
	paths.WalletPeekChangeAddress = func() string { return disk.PeekChangeAddress() }
	paths.WalletCommitChangeAddress = func(addr string) error { return disk.CommitChangeAddress(addr) }
	paths.WalletListDescriptors = func(chainName string) []rpc.WalletDescriptorRow {
		net := chain.RebootTestnet
		if chainName == "main" {
			net = chain.MainnetDogecoin
		}
		p, err := chain.ParamsFor(net)
		if err != nil {
			return nil
		}
		rows := disk.ListDescriptors(p.PubkeyHashAddrID, p.ScriptHashAddrID)
		out := make([]rpc.WalletDescriptorRow, len(rows))
		for i, r := range rows {
			out[i] = rpc.WalletDescriptorRow{Desc: r.Desc, Timestamp: r.Timestamp, Active: r.Active, Internal: r.Internal}
		}
		return out
	}
	paths.WalletAvoidReuse = func() bool { return disk.AvoidReuse() }
	paths.WalletSetAvoidReuse = func(v bool) error { return disk.SetAvoidReuse(v) }
	paths.WalletPqCommitmentsEnabled = func() bool { return disk.PqCommitmentsEnabled() }
	paths.WalletSetPqCommitmentsEnabled = func(v bool) error { return disk.SetPqCommitmentsEnabled(v) }
	paths.WalletPqCarrierEnabled = func() bool { return disk.PqCarrierEnabled() }
	paths.WalletSetPqCarrierEnabled = func(v bool) error { return disk.SetPqCarrierEnabled(v) }
	paths.WalletPQCarrierKeyMaterial = func(tag string) (string, []byte, []byte, error) {
		return disk.PQCarrierKeyMaterial(tag)
	}
	paths.WalletNextPQCommit = func() (string, string, error) { return disk.NextPQCommitment() }
	_ = disk.EnsurePQReady()
	paths.WalletIsScriptReused = func(pk []byte) bool { return disk.IsRecvScriptReused(pk) }
	disk.SetNetAddrVersions(pkhVer, shVer)
	disk.RebuildUsedRecvScripts(pkhVer, shVer)
	paths.WalletRebuildUsedRecvScripts = func() { disk.RebuildUsedRecvScripts(pkhVer, shVer) }
	paths.WalletAddImportedDescriptor = func(desc string, ts int64, internal, spendable bool) error {
		return disk.AddImportedDescriptor(desc, ts, internal, spendable)
	}
	paths.WalletAddress = paths.WalletDefaultAddress
	paths.WalletSpendScripts = func() [][]byte { return disk.SpendScripts() }
	paths.WalletContainsAddress = func(addr string) bool { return disk.ContainsAddress(addr) }
	paths.WalletNewAddress = func() (string, error) { return disk.NewReceiveAddress() }
	paths.WalletNewChangeAddress = func() (string, error) { return disk.NewChangeAddress() }
	paths.WalletKeypoolRefill = func(n int) error { return disk.KeypoolRefill(n) }
	paths.WalletReplayCorePool = func(entries []corewallet.PoolEntry) (wallet.PoolReplayResult, error) {
		return disk.ReplayCorePoolIntoHDKeypool(entries)
	}
	paths.WalletWIFForAddress = func(addr string) (string, error) {
		priv, err := disk.PrivKeyForAddress(addr)
		if err != nil {
			return "", err
		}
		return chain.EncodeWIF(priv.Serialize(), wifVer, true)
	}
	paths.WalletHDFormat = func() string {
		if disk.HDEnabled() {
			return "hd"
		}
		return ""
	}
	paths.WalletKeypoolSize = func() int { return disk.KeypoolSize() }
	paths.WalletChangeKeypoolSize = func() int { return disk.ChangeKeypoolSize() }
	paths.WalletHDKeypoolCoreIndex = func() []wallet.HDKeypoolCoreIndexEntry { return disk.HDKeypoolCoreIndexEntries() }
	paths.WalletAddressInReceiveKeypool = func(addr string) bool { return disk.IsReceiveInKeypool(addr) }
	paths.WalletAddressInChangeKeypool = func(addr string) bool { return disk.IsChangeInKeypool(addr) }
	paths.WalletAddressIsNodeTip = func(addr string) bool { return disk.IsNodeTipAddress(addr) }
	paths.WalletAddressCorePoolIndex = func(addr string) (int64, bool) { return disk.CorePoolIndexForAddress(addr) }
	paths.WalletKnownAddresses = func() []string {
		return disk.KnownAddresses(pkhVer, shVer)
	}
	paths.WalletAddressHDPath = func(addr string) (hdpath string, ischange bool, ok bool) {
		return disk.AddressHDPath(addr)
	}
	paths.WalletMasterKeyFingerprint = func() (uint32, bool) { return disk.MasterKeyFingerprint() }
	paths.WalletCompressedPubKeyForAddress = func(addr string) ([]byte, bool) {
		return disk.CompressedPubKeyForAddress(addr)
	}
	paths.WalletHDSeedID = func() string { return disk.HDSeedIDHex() }
	paths.WalletImportMnemonic = func(mnemonic, passphrase string) error {
		return disk.RestoreFromMnemonic(mnemonic, passphrase)
	}
	paths.WalletImportBIP38 = func(encrypted, passphrase string) (string, error) {
		if err := disk.ImportBIP38(encrypted, passphrase, wifVer, pkhVer); err != nil {
			return "", err
		}
		return disk.DefaultAddress(), nil
	}
	paths.WalletListAddresses = func() []rpc.WalletAddressEntry {
		rows := disk.ListAddressEntries(pkhVer, shVer)
		out := make([]rpc.WalletAddressEntry, len(rows))
		for i, r := range rows {
			out[i] = rpc.WalletAddressEntry{
				Address: r.Address, HDPath: r.HDPath, IsChange: r.IsChange,
				Label: r.Label, WatchOnly: r.WatchOnly, IsCosigner: r.IsCosigner,
				IsKeypool: r.IsKeypool, IsNodeTip: r.IsNodeTip, HDKeypoolCoreIndex: r.HDKeypoolCoreIndex,
			}
		}
		return out
	}
}

func wireWalletRescan(paths *rpc.DataPaths, disk *wallet.Disk, j *store.HeaderJournal, raw *store.RawBlockStore, pkhVer, shVer byte) {
	if paths == nil || disk == nil {
		return
	}
	paths.WalletIsScanning = WalletIsScanning
	paths.WalletMaxScannedBlockHeight = func() int64 { return disk.MaxScannedBlockHeight() }
	paths.WalletListScannedTx = func() []wallet.ScannedTx { return disk.ListScannedTx() }
	paths.WalletSendFeeLookup = func(txid string) (int64, bool) { return disk.LookupSendFee(txid) }
	paths.WalletRememberTxHex = func(txid, hexStr string) error { return disk.RememberTxHex(txid, hexStr) }
	paths.WalletTxHexLookup = func(txid string) (string, bool) { return disk.LookupTxHex(txid) }
	paths.WalletRescanBlocks = func(start int64) error {
		if j == nil || raw == nil {
			return wallet.ErrNoRawBlocks
		}
		setWalletScanning(true)
		defer setWalletScanning(false)
		err := disk.RescanBlocks(j, raw, pkhVer, shVer, start)
		if err == nil {
			rpc.RefreshWalletUtxoCache(paths, disk.TrackedScripts())
		}
		return err
	}
}

// wireWalletUtxoCacheOnAdvance persists wallet_utxo_scan.cache.json after connect advances (debounced).
func wireWalletUtxoCacheOnAdvance(bs *BlockStoreCtx, paths *rpc.DataPaths, disk *wallet.Disk) {
	if bs == nil || paths == nil || disk == nil {
		return
	}
	var st struct {
		mu         sync.Mutex
		lastHeight int64
		lastAt     time.Time
	}
	bs.AppendOnChainActiveAdvance(func(h int64) {
		st.mu.Lock()
		defer st.mu.Unlock()
		if h == st.lastHeight {
			return
		}
		if st.lastHeight >= 0 && h-st.lastHeight < 4 && time.Since(st.lastAt) < 15*time.Second {
			return
		}
		st.lastHeight = h
		st.lastAt = time.Now()
		rpc.RefreshWalletUtxoCache(paths, disk.TrackedScripts())
	})
}

// wireWalletUtxoCacheWarm rebuilds wallet_utxo_scan.cache.json once at startup when UTXO snapshot is loaded.
func wireWalletUtxoCacheWarm(paths *rpc.DataPaths, disk *wallet.Disk) {
	if paths == nil || disk == nil || paths.Utxo == nil {
		return
	}
	if paths.Utxo.TipHeight() < 0 {
		return
	}
	rpc.RefreshWalletUtxoCache(paths, disk.TrackedScripts())
}
