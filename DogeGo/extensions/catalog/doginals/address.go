// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"encoding/hex"
	"strings"

	"dogego/chain"
	"dogego/extensions"
	"dogego/wire"
)

func paramsForHost(host extensions.Host) (pubVer, shVer byte) {
	net := ""
	if host != nil {
		net = strings.ToLower(strings.TrimSpace(host.Network()))
	}
	switch net {
	case "testnet", "reboottestnet":
		if p, err := chain.ParamsFor(chain.RebootTestnet); err == nil {
			return p.PubkeyHashAddrID, p.ScriptHashAddrID
		}
		return 0x71, 0xc4
	default:
		if p, err := chain.ParamsFor(chain.MainnetDogecoin); err == nil {
			return p.PubkeyHashAddrID, p.ScriptHashAddrID
		}
		return 0x1e, 0x16
	}
}

func addressFromPkScript(host extensions.Host, pk []byte) string {
	pub, sh := paramsForHost(host)
	return chain.ScriptPubKeyAddress(pk, pub, sh)
}

// resolveInputAddress returns the address that funded vin[0] (sender), via txindex LookupTxHex.
func resolveInputAddress(host extensions.Host, tx *wire.Tx) string {
	if host == nil || tx == nil || len(tx.Vin) == 0 {
		return ""
	}
	in := tx.Vin[0]
	if in.PrevIdx == 0xffffffff {
		return "" // coinbase
	}
	prevTxid := powLEDisplayHex(in.PrevHash)
	hx, _, ok := host.LookupTxHex(prevTxid)
	if !ok || hx == "" {
		return ""
	}
	raw, err := hex.DecodeString(strings.TrimSpace(hx))
	if err != nil {
		return ""
	}
	prev, err := wire.DeserializeTx(raw)
	if err != nil || prev == nil {
		return ""
	}
	if int(in.PrevIdx) >= len(prev.Vout) {
		return ""
	}
	return addressFromPkScript(host, prev.Vout[in.PrevIdx].PkScript)
}

// firstSpendableOutputAddress returns the first non-OP_RETURN output address (typical recipient).
func firstSpendableOutputAddress(host extensions.Host, tx *wire.Tx) string {
	if tx == nil {
		return ""
	}
	for _, o := range tx.Vout {
		if len(o.PkScript) > 0 && o.PkScript[0] == 0x6a {
			continue
		}
		if a := addressFromPkScript(host, o.PkScript); a != "" {
			return a
		}
	}
	return ""
}

func powLEDisplayHex(h [32]byte) string {
	return TxDisplayHex(h)
}

func outpointKey(txid string, vout uint32) string {
	return strings.ToLower(strings.TrimSpace(txid)) + ":" + itoa(int(vout))
}
