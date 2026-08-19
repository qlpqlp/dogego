// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "fmt"

// Network identifies which chain params set to use.
type Network int

const (
	// RebootTestnet matches CTestNetParams in parent src/chainparams.cpp (rebooted testnet).
	RebootTestnet Network = iota
	// MainnetDogecoin matches CMainParams (magic 0xc0c0c0c0, port 22556).
	MainnetDogecoin
)

// Params holds P2P and genesis fields for one network.
type Params struct {
	Net Network

	Magic [4]byte
	Port  int

	// DNSSeeds are hostnames for peer discovery (A/AAAA lookup, default Port).
	DNSSeeds []string
	// FixedPeers are host:port entries from chainparamsseeds.h (Core pnSeed6_*), appended after DNS.
	FixedPeers []string

	GenesisVer           int32
	GenesisTime          uint32
	GenesisNonce         uint32
	GenesisBits          uint32
	GenesisMerkleRootHex string
	GenesisBlockHashHex  string

	ProtocolVersion int32
	NodeNetwork     uint64

	// PubkeyHashAddrID is the base58 version byte for P2PKH addresses (see CChainParams::base58Prefixes).
	PubkeyHashAddrID byte
	// ScriptHashAddrID is the base58 version byte for P2SH addresses (PUBKEY_ADDRESS / SCRIPT_ADDRESS in chainparams.cpp).
	ScriptHashAddrID byte
	// PrivKeyWIFVersion is the Base58Check version byte for WIF-encoded private keys (SECRET_KEY in chainparams.cpp).
	PrivKeyWIFVersion byte

	// RelaxedPoW skips CheckScryptPoW when linking headers (legacy test helper; reboot testnet uses real scrypt PoW).
	RelaxedPoW bool
}

// IsRebootTestnet reports reboot testnet chain params (CTestNetParams in Core).
func (p Params) IsRebootTestnet() bool {
	return p.Net == RebootTestnet
}

// ParamsFor returns bundled chain parameters for reboot testnet and mainnet.
func ParamsFor(net Network) (Params, error) {
	switch net {
	case RebootTestnet:
		fp := make([]string, len(TestnetFixedSeedAddrs))
		copy(fp, TestnetFixedSeedAddrs)
		return Params{
			Net:                  RebootTestnet,
			Magic:                Magic,
			Port:                 Port,
			DNSSeeds:             []string{"seed.dogego.org"},
			FixedPeers:           fp,
			GenesisVer:           GenesisVer,
			GenesisTime:          GenesisTime,
			GenesisNonce:         GenesisNonce,
			GenesisBits:          GenesisBits,
			GenesisMerkleRootHex: GenesisMerkleRootHex,
			GenesisBlockHashHex:  GenesisBlockHashHex,
			ProtocolVersion:      ProtocolVersion,
			NodeNetwork:          NodeNetwork,
			PubkeyHashAddrID:     0x41,
			ScriptHashAddrID:     0x42,
			PrivKeyWIFVersion:    193,
			RelaxedPoW:           false,
		}, nil
	case MainnetDogecoin:
		mfp := make([]string, len(MainnetFixedSeedAddrs))
		copy(mfp, MainnetFixedSeedAddrs)
		// Dogecoin mainnet (CMainParams): pchMessageStart c0 c0 c0 c0, port 22556.
		return Params{
			Net:                  MainnetDogecoin,
			Magic:                [4]byte{0xc0, 0xc0, 0xc0, 0xc0},
			Port:                 22556,
			DNSSeeds:             []string{"seed.multidoge.org", "seed2.multidoge.org"},
			FixedPeers:           mfp,
			ProtocolVersion:      ProtocolVersion,
			NodeNetwork:          NodeNetwork,
			PubkeyHashAddrID:     30,
			ScriptHashAddrID:     22,
			PrivKeyWIFVersion:    158,
			GenesisMerkleRootHex: "5b2a3f53f605d62c53e62932dac6925e3d74afa5a4b459745c36d42d0ed26a69",
			GenesisBlockHashHex:  "1a91e3dace36e2be3bf030a65679fe821aa1d6ef92e7c9902eb318182c355691",
			GenesisVer:           1,
			GenesisTime:          1386325540,
			GenesisNonce:         99943,
			GenesisBits:          0x1e0ffff0,
			RelaxedPoW:           false,
		}, nil
	default:
		return Params{}, fmt.Errorf("unknown network %d", net)
	}
}
