// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dogeos

import (
	"fmt"
	"strconv"
	"strings"
)

// Helpers returns copy-paste snippets for wallets and tooling.
func Helpers(n NetworkProfile, rpcURL string) map[string]interface{} {
	rpc := strings.TrimRight(rpcURL, "/")
	if rpc == "" {
		rpc = strings.TrimRight(n.RPCURL, "/")
	}
	chainHex := fmt.Sprintf("0x%x", n.ChainID)
	metamask := map[string]interface{}{
		"chainId":           chainHex,
		"chainName":         n.Name,
		"nativeCurrency":    map[string]interface{}{"name": "Dogecoin", "symbol": n.Currency, "decimals": 18},
		"rpcUrls":           []string{rpc + "/"},
		"blockExplorerUrls": []string{strings.TrimRight(n.ExplorerURL, "/")},
	}
	hardhat := fmt.Sprintf(`networks: {
  dogeos: {
    url: "%s/",
    chainId: %d,
  },
}`, rpc, n.ChainID)
	foundry := fmt.Sprintf(`# foundry.toml
[rpc_endpoints]
dogeos = "%s/"`, rpc)
	ethers := fmt.Sprintf(`import { ethers } from "ethers";
const provider = new ethers.JsonRpcProvider("%s/", %d);
const block = await provider.getBlockNumber();
console.log("DogeOS tip", block);`, rpc, n.ChainID)
	viem := fmt.Sprintf(`import { createPublicClient, http } from "viem";
const client = createPublicClient({
  chain: {
    id: %d,
    name: "%s",
    nativeCurrency: { name: "Dogecoin", symbol: "DOGE", decimals: 18 },
    rpcUrls: { default: { http: ["%s/"] } },
  },
  transport: http(),
});
const block = await client.getBlockNumber();`, n.ChainID, n.Name, rpc)
	curl := fmt.Sprintf(`curl -s -X POST %s/ -H "content-type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}'`, rpc)
	cast := fmt.Sprintf(`cast block-number --rpc-url %s/`, rpc)
	return map[string]interface{}{
		"network":          n,
		"rpc_url":          rpc + "/",
		"chain_id":         n.ChainID,
		"chain_id_hex":     chainHex,
		"metamask_add":     metamask,
		"hardhat_network":  hardhat,
		"foundry_toml":     foundry,
		"ethers_v6":        ethers,
		"viem":             viem,
		"curl_block":       curl,
		"cast_block":       cast,
		"faucet_url":       n.FaucetURL,
		"bridge_docs":      n.BridgeURL,
		"explorer_url":     n.ExplorerURL,
		"docs_url":         n.DocsURL,
		"wallet_setup":     n.PortalURL,
	}
}

// ExplorerTxURL builds a Blockscout tx link when possible.
func ExplorerTxURL(n NetworkProfile, txHash string) string {
	base := strings.TrimRight(n.ExplorerURL, "/")
	h := strings.TrimSpace(txHash)
	if base == "" || h == "" {
		return ""
	}
	if !strings.HasPrefix(h, "0x") {
		h = "0x" + h
	}
	return base + "/tx/" + h
}

// ExplorerAddressURL builds a Blockscout address link.
func ExplorerAddressURL(n NetworkProfile, address string) string {
	base := strings.TrimRight(n.ExplorerURL, "/")
	a := normalizeAddress(address)
	if base == "" || a == "" {
		return ""
	}
	return base + "/address/" + a
}

// FormatChainIDHex returns 0x-prefixed chain id.
func FormatChainIDHex(id int64) string {
	return "0x" + strconv.FormatInt(id, 16)
}
