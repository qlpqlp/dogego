package ui

import (
	"testing"

	"dogego/config"
)

func TestProbeFounderSkippedMainnet(t *testing.T) {
	r := ProbeFounder("mainnet", config.File{Network: "mainnet"})
	if !r.OK || !r.Skipped {
		t.Fatalf("%+v", r)
	}
}

func TestProbeFounderTestnet(t *testing.T) {
	r := ProbeFounder("testnet", config.File{Network: "testnet", NodeMode: "full", P2PConnectivity: "both"})
	if !r.OK || r.Skipped {
		t.Fatalf("%+v", r)
	}
}

func TestProbeFounderTestnetSPV(t *testing.T) {
	r := ProbeFounder("testnet", config.File{Network: "testnet", NodeMode: "spv"})
	if r.OK || r.Skipped {
		t.Fatal("spv founder should fail")
	}
}
