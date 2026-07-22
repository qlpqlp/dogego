// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"strconv"
	"strings"

	"dogego/chain"
	"dogego/wire"
)

func parseBIP32PathString(path string) ([]uint32, error) {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "m/")
	path = strings.TrimPrefix(path, "M/")
	if path == "" {
		return nil, nil
	}
	parts := strings.Split(path, "/")
	out := make([]uint32, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		hardened := strings.HasSuffix(part, "'") || strings.HasSuffix(part, "h") || strings.HasSuffix(part, "H")
		part = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(part, "'"), "h"), "H")
		n, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, err
		}
		idx := uint32(n)
		if hardened {
			idx |= 0x80000000
		}
		out = append(out, idx)
	}
	return out, nil
}

func rpcWalletMasterFingerprint(paths *DataPaths) (uint32, bool) {
	if paths != nil && paths.WalletMasterKeyFingerprint != nil {
		return paths.WalletMasterKeyFingerprint()
	}
	return 0, false
}

func attachWalletPSBTDerivations(chainName string, paths *DataPaths, p *wire.Psbt) {
	if paths == nil || p == nil || p.UnsignedTx == nil {
		return
	}
	fp, ok := rpcWalletMasterFingerprint(paths)
	if !ok {
		return
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return
	}
	cp, err := chain.ParamsFor(net)
	if err != nil {
		return
	}
	for i, out := range p.UnsignedTx.Vout {
		attachPSBTDerivForScript(paths, p, fp, cp.PubkeyHashAddrID, cp.ScriptHashAddrID, true, i, out.PkScript)
	}
	for i, in := range p.UnsignedTx.Vin {
		if isCoinbaseWireIn(&in) {
			continue
		}
		spk := psbtInputScriptPubKey(p, i, paths)
		if len(spk) == 0 {
			continue
		}
		attachPSBTDerivForScript(paths, p, fp, cp.PubkeyHashAddrID, cp.ScriptHashAddrID, false, i, spk)
	}
}

func psbtInputScriptPubKey(p *wire.Psbt, i int, paths *DataPaths) []byte {
	if p == nil || p.UnsignedTx == nil || i < 0 || i >= len(p.UnsignedTx.Vin) {
		return nil
	}
	in := p.UnsignedTx.Vin[i]
	for _, kv := range p.Inputs[i] {
		if kv.Type == wire.PsbtInNonWitnessUtxo {
			parent, err := wire.DeserializeTx(kv.Value)
			if err == nil && int(in.PrevIdx) < len(parent.Vout) {
				return append([]byte(nil), parent.Vout[in.PrevIdx].PkScript...)
			}
		}
	}
	if paths != nil && paths.Utxo != nil {
		if e, ok := paths.Utxo.Lookup(txidToRPC(in.PrevHash), in.PrevIdx); ok {
			return append([]byte(nil), e.PkScript...)
		}
	}
	return nil
}

func attachPSBTDerivForScript(paths *DataPaths, p *wire.Psbt, fp uint32, pkhVer, shVer byte, isOutput bool, idx int, spk []byte) {
	if paths.WalletOwnsScript == nil || !paths.WalletOwnsScript(spk) {
		return
	}
	addr := chain.ScriptPubKeyAddress(spk, pkhVer, shVer)
	if addr == "" {
		return
	}
	if paths.WalletAddressHDPath == nil {
		return
	}
	hdpath, _, ok := paths.WalletAddressHDPath(addr)
	if !ok || hdpath == "" {
		return
	}
	path, err := parseBIP32PathString(hdpath)
	if err != nil || len(path) == 0 {
		return
	}
	var pub []byte
	if paths.WalletCompressedPubKeyForAddress != nil {
		pub, ok = paths.WalletCompressedPubKeyForAddress(addr)
	}
	if !ok || len(pub) == 0 {
		return
	}
	val := wire.EncodeBIP32DerivationValue(fp, path)
	if isOutput {
		p.SetOutputBIP32Derivation(idx, pub, val)
	} else {
		p.SetInputBIP32Derivation(idx, pub, val)
	}
}
