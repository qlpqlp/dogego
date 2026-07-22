// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

// HeaderCheckpoint is a Core chainparams mapCheckpoints entry (height → block hash display hex).
type HeaderCheckpoint struct {
	Height  int64
	HashHex string
}

// MainnetHeaderCheckpoints mirrors Dogecoin Core CMainParams checkpointData (src/chainparams.cpp).
var MainnetHeaderCheckpoints = []HeaderCheckpoint{
	{0, "1a91e3dace36e2be3bf030a65679fe821aa1d6ef92e7c9902eb318182c355691"},
	{104679, "35eb87ae90d44b98898fec8c39577b76cb1eb08e1261cfc10706c8ce9a1d01cf"},
	{145000, "cc47cae70d7c5c92828d3214a266331dde59087d4a39071fa76ddfff9b7bde72"},
	{371337, "60323982f9c5ff1b5a954eac9dc1269352835f47c2c5222691d80f0d50dcf053"},
	{450000, "d279277f8f846a224d776450aa04da3cf978991a182c6f3075db4c48b173bbd7"},
	{771275, "1b7d789ed82cbdc640952e7e7a54966c6488a32eaad54fc39dff83f310dbaaed"},
	{1000000, "6aae55bea74235f0c80bd066349d4440c31f2d0f27d54265ecd484d8c1d11b47"},
	{1250000, "00c7a442055c1a990e11eea5371ca5c1c02a0677b33cc88ec728c45edc4ec060"},
	{1500000, "f1d32d6920de7b617d51e74bdf4e58adccaa582ffdc8657464454f16a952fca6"},
	{1750000, "5c8e7327984f0d6f59447d89d143e5f6eafc524c82ad95d176c5cec082ae2001"},
	{2000000, "9914f0e82e39bbf21950792e8816620d71b9965bdbbc14e72a95e3ab9618fea8"},
	{2031142, "893297d89afb7599a3c571ca31a3b80e8353f4cf39872400ad0f57d26c4c5d42"},
	{2250000, "0a87a8d4e40dca52763f93812a288741806380cd569537039ee927045c6bc338"},
	{2510150, "77e3f4a4bcb4a2c15e8015525e3d15b466f6c022f6ca82698f329edef7d9777e"},
	{2750000, "d4f8abb835930d3c4f92ca718aaa09bef545076bd872354e0b2b85deefacf2e3"},
	{3000000, "195a83b091fb3ee7ecb56f2e63d01709293f57f971ccf373d93890c8dc1033db"},
	{3250000, "7f3e28bf9e309c4b57a4b70aa64d3b2ea5250ae797af84976ddc420d49684034"},
	{3500000, "eaa303b93c1c64d2b3a2cdcf6ccf21b10cc36626965cc2619661e8e1879abdfb"},
	{3606083, "954c7c66dee51f0a3fb1edb26200b735f5275fe54d9505c76ebd2bcabac36f1e"},
	{3854173, "e4b4ecda4c022406c502a247c0525480268ce7abbbef632796e8ca1646425e75"},
	{3963597, "2b6927cfaa5e82353d45f02be8aadd3bfd165ece5ce24b9bfa4db20432befb5d"},
	{4303965, "ed7d266dcbd8bb8af80f9ccb8deb3e18f9cc3f6972912680feeb37b090f8cee0"},
	{5050000, "e7d4577405223918491477db725a393bcfc349d8ee63b0a4fde23cbfbfd81dea"},
}

// RebootTestnetHeaderCheckpoints mirrors Core CTestNetParams checkpointData (reboot testnet genesis).
var RebootTestnetHeaderCheckpoints = []HeaderCheckpoint{
	{0, "d5d619f8be025d4700940883c86f271d08cffa8dd1d3d4afa474c9ed9e8b68a0"},
}

// HeaderCheckpointsFor returns Core checkpoint heights for a network (nil if none).
func HeaderCheckpointsFor(net Network) []HeaderCheckpoint {
	switch net {
	case MainnetDogecoin:
		return MainnetHeaderCheckpoints
	case RebootTestnet:
		return RebootTestnetHeaderCheckpoints
	default:
		return nil
	}
}

// CheckpointHashAt returns the expected block hash at height when defined in Core checkpoints.
func CheckpointHashAt(net Network, height int64) (string, bool) {
	for _, cp := range HeaderCheckpointsFor(net) {
		if cp.Height == height {
			return cp.HashHex, true
		}
	}
	return "", false
}
