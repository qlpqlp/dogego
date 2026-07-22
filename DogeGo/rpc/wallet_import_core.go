// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dogego/chain"
	"dogego/store"
	"dogego/wallet/bdb"
	"dogego/wallet/corewallet"
)

func isBerkeleyWalletDat(path string) bool {
	return bdb.IsBDBFile(path)
}

func isTextWalletDump(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return false
	}
	s := string(buf[:n])
	if strings.HasPrefix(strings.TrimSpace(s), "#") {
		return true
	}
	comma := strings.IndexByte(s, ',')
	return comma > 0 && (strings.Contains(s, "label=") || strings.Count(s[:comma], " ") >= 1)
}

func writeNativeWalletDump(chainName, srcPath, chainDataDir, passphrase string) (dumpPath string, keyCount int, err error) {
	net, err := chain.ParseNetwork(chainName)
	if err != nil {
		return "", 0, err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", 0, err
	}
	ex, err := corewallet.ExtractDumpLinesWithPassphrase(srcPath, p.PrivKeyWIFVersion, passphrase)
	if err != nil {
		return "", 0, err
	}
	dumpPath = filepath.Join(chainDataDir, "dogego_native_import_"+time.Now().Format("20060102_150405")+".dump")
	if err := os.WriteFile(dumpPath, []byte(strings.Join(ex.Lines, "\n")+"\n"), 0o600); err != nil {
		return "", 0, err
	}
	return dumpPath, ex.KeyCount, nil
}

// nativeWalletDatPoolPrep probes Core pool metadata and replays matched HD receive
// pubkeys before dump import - ImportSpendPrivKey would clear HD if run first.
func nativeWalletDatPoolPrep(chainName string, paths *DataPaths, walletDatPath string) (*corewallet.ProbeResult, bool, int) {
	probe, err := probeWalletDat(chainName, walletDatPath)
	if err != nil || probe == nil {
		return nil, false, 0
	}
	replayed, stored := replayCorePoolEntries(paths, probe.PoolEntries)
	return probe, replayed, stored
}

func replayCorePoolEntries(paths *DataPaths, entries []corewallet.PoolEntry) (bool, int) {
	if paths == nil || paths.WalletReplayCorePool == nil || len(entries) == 0 {
		return false, 0
	}
	res, err := paths.WalletReplayCorePool(entries)
	if err != nil {
		return false, 0
	}
	return res.IndicesReplayed, res.CoreIndicesStored
}

// execImportWalletDat imports keys from a Core wallet via native BDB read, Core dumpwallet, or a text dump file.
// Params: filename string, optional options object { "via_core_rpc": true, "native_bdb": true, "passphrase": "…" }.
func execImportWalletDat(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var filename string
	if err := json.Unmarshal(params[0], &filename); err != nil {
		return nil, -8, "dogego_importwalletdat: filename must be a string"
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, -8, "dogego_importwalletdat: invalid filename"
	}
	validated, err := ValidateFilePath(dataPathRoots(paths), filename, true)
	if err != nil {
		return nil, -8, "dogego_importwalletdat: "+err.Error()
	}
	filename = validated
	viaCore := false
	nativeOnly := false
	passphrase := ""
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var opts struct {
			ViaCoreRPC bool   `json:"via_core_rpc"`
			NativeBDB  bool   `json:"native_bdb"`
			Passphrase string `json:"passphrase"`
		}
		if err := json.Unmarshal(params[1], &opts); err != nil {
			return nil, -8, "dogego_importwalletdat: options must be a JSON object"
		}
		viaCore = opts.ViaCoreRPC
		nativeOnly = opts.NativeBDB
		passphrase = opts.Passphrase
	}
	if paths == nil || paths.WalletImportSpendKey == nil {
		return nil, -1, "dogego_importwalletdat: wallet is not available"
	}

	importPath := filename
	usedCore := false
	usedNative := false
	var nativeKeys int
	var nativePoolProbe *corewallet.ProbeResult
	var poolIndicesReplayed bool
	var poolCoreIndicesStored int

	if isBerkeleyWalletDat(filename) {
		if paths.ChainDataDir == "" {
			return nil, -1, "dogego_importwalletdat: chain data directory not configured"
		}
		if viaCore {
			if _, _, _, ok := coreRPCFromPaths(paths); !ok {
				return nil, -1, "dogego_importwalletdat: core_rpc_addr not configured"
			}
			dumpPath := filepath.Join(paths.ChainDataDir, "dogego_core_import_"+time.Now().Format("20060102_150405")+".dump")
			if code, msg := coreDumpWallet(paths, chainName, dumpPath); code != 0 {
				return nil, code, "dogego_importwalletdat: Core dumpwallet failed: "+msg
			}
			importPath = dumpPath
			usedCore = true
		} else if nativeOnly {
			nativePoolProbe, poolIndicesReplayed, poolCoreIndicesStored = nativeWalletDatPoolPrep(chainName, paths, filename)
			dumpPath, nKeys, err := writeNativeWalletDump(chainName, filename, paths.ChainDataDir, passphrase)
			if err != nil {
				return nil, -8, "dogego_importwalletdat: "+err.Error()
			}
			importPath = dumpPath
			usedNative = true
			nativeKeys = nKeys
		} else if dumpPath, nKeys, err := writeNativeWalletDump(chainName, filename, paths.ChainDataDir, passphrase); err == nil {
			nativePoolProbe, poolIndicesReplayed, poolCoreIndicesStored = nativeWalletDatPoolPrep(chainName, paths, filename)
			importPath = dumpPath
			usedNative = true
			nativeKeys = nKeys
		} else if _, _, _, ok := coreRPCFromPaths(paths); ok {
			dumpPath := filepath.Join(paths.ChainDataDir, "dogego_core_import_"+time.Now().Format("20060102_150405")+".dump")
			if code, msg := coreDumpWallet(paths, chainName, dumpPath); code != 0 {
				return nil, code, "dogego_importwalletdat: Core dumpwallet failed: "+msg
			}
			importPath = dumpPath
			usedCore = true
		} else {
			return nil, -8, "dogego_importwalletdat: native BDB read failed ("+err.Error()+") - set core_rpc_addr and pass {\"via_core_rpc\":true}"
		}
	} else if !isTextWalletDump(filename) {
		return nil, -8, "dogego_importwalletdat: file is not a dumpwallet text dump - use importwallet for text dumps or pass a Core wallet.dat"
	}

	pathJ, err := json.Marshal(importPath)
	if err != nil {
		return nil, -1, err.Error()
	}
	res, code, msg := execImportWallet(chainName, paths, j, raw, []json.RawMessage{pathJ})
	if code != 0 {
		if !strings.HasPrefix(msg, "dogego_importwalletdat:") && !strings.HasPrefix(msg, "importwallet:") {
			msg = "dogego_importwalletdat: " + msg
		}
		return res, code, msg
	}
	out := map[string]interface{}{"imported_from": importPath}
	if usedCore {
		out["via_core_rpc"] = true
		if filename != importPath {
			out["source"] = filename
		}
	}
	if usedNative {
		out["via_native_bdb"] = true
		out["keys_imported"] = nativeKeys
		if filename != importPath {
			out["source"] = filename
		}
		probe := nativePoolProbe
		if probe == nil {
			probe, _ = probeWalletDat(chainName, filename)
		}
		if probe != nil {
			corewallet.ApplyPoolProbeFields(out, probe)
			if !poolIndicesReplayed {
				replayed, stored := replayCorePoolEntries(paths, probe.PoolEntries)
				if replayed {
					poolIndicesReplayed = true
				}
				if stored > poolCoreIndicesStored {
					poolCoreIndicesStored = stored
				}
			}
			if poolIndicesReplayed {
				out["pool_indices_replayed"] = true
			}
			if poolCoreIndicesStored > 0 {
				out["pool_core_indices_stored"] = poolCoreIndicesStored
			}
			if size := corewallet.SuggestedKeypoolRefillSize(probe.PoolKeysUnmatched); size > 0 {
				walletKeypoolRefillForPoolProbe(paths, probe)
				out["keypool_refill_size"] = size
			}
		}
	}
	return out, 0, ""
}

func probeWalletDat(chainName, path string) (*corewallet.ProbeResult, error) {
	net, err := chain.ParseNetwork(chainName)
	if err != nil {
		return nil, err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, err
	}
	return corewallet.ProbeWalletDat(path, p.PrivKeyWIFVersion)
}
