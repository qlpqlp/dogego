// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package dogeos

// NetworkProfile describes a DogeOS EVM network DogeGo can talk to.
type NetworkProfile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ChainID     int64  `json:"chain_id"`
	RPCURL      string `json:"rpc_url"`
	ExplorerURL string `json:"explorer_url"`
	Currency    string `json:"currency"`
	FaucetURL   string `json:"faucet_url,omitempty"`
	BridgeURL   string `json:"bridge_url,omitempty"`
	DocsURL     string `json:"docs_url,omitempty"`
	PortalURL   string `json:"portal_url,omitempty"`
	Available   bool   `json:"available"`
	Kind        string `json:"kind"` // testnet | mainnet
	Notes       string `json:"notes,omitempty"`
}

const (
	NetworkChikyuTestnet = "chikyu-testnet"
	NetworkMainnetSoon   = "mainnet"
)

// BuiltInNetworks are known DogeOS profiles. Only Chikyū testnet is live today.
func BuiltInNetworks() []NetworkProfile {
	return []NetworkProfile{
		{
			ID:          NetworkChikyuTestnet,
			Name:        "DogeOS Chikyū Testnet",
			ChainID:     6281971,
			RPCURL:      "https://rpc.testnet.dogeos.com/",
			ExplorerURL: "https://blockscout.testnet.dogeos.com",
			Currency:    "DOGE",
			FaucetURL:   "https://faucet.testnet.dogeos.com/",
			BridgeURL:   "https://docs.dogeos.com/en/getting-started/user-guide/bridge",
			DocsURL:     "https://docs.dogeos.com/en/developers",
			PortalURL:   "https://docs.dogeos.com/en/getting-started/user-guide/setup",
			Available:   true,
			Kind:        "testnet",
			Notes:       "Public EVM testnet. ~3s blocks. Use DOGE for gas.",
		},
		{
			ID:          NetworkMainnetSoon,
			Name:        "DogeOS Mainnet",
			ChainID:     0,
			RPCURL:      "",
			ExplorerURL: "",
			Currency:    "DOGE",
			DocsURL:     "https://docs.dogeos.com/en/developers",
			Available:   false,
			Kind:        "mainnet",
			Notes:       "Not live yet. Enable in Settings when DogeOS publishes mainnet RPC + chain id.",
		},
	}
}

func FindNetwork(id string) (NetworkProfile, bool) {
	for _, n := range BuiltInNetworks() {
		if n.ID == id {
			return n, true
		}
	}
	return NetworkProfile{}, false
}

func DefaultNetworkID() string { return NetworkChikyuTestnet }
