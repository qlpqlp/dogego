package ui

import "testing"

func TestBuildSetupFounderPreflightSkippedMainnet(t *testing.T) {
	r := buildSetupFounderPreflight(setupFounderPreflightRequest{Network: "mainnet"})
	if !r.OK || len(r.Checks) != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestBuildSetupFounderPreflightTestnetOK(t *testing.T) {
	r := buildSetupFounderPreflight(setupFounderPreflightRequest{
		Network: "testnet", NodeMode: "full", P2PConnectivity: "both",
		DataDir: t.TempDir(),
	})
	if !r.OK {
		t.Fatalf("%+v", r)
	}
}

func TestBuildSetupFounderPreflightSPVIssue(t *testing.T) {
	r := buildSetupFounderPreflight(setupFounderPreflightRequest{
		Network: "testnet", NodeMode: "spv", DataDir: t.TempDir(),
	})
	if r.OK {
		t.Fatal("spv should fail")
	}
}
